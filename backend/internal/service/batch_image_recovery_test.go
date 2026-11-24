//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBatchRecoveryLooksUpOriginalOwnersSubmission(t *testing.T) {
	ctx := context.Background()
	svc, repo, queue, provider, _ := newTestBatchImagePublicService(true)
	owner := testBatchImageOwner()
	created, err := svc.Submit(ctx, owner, validBatchImageSubmitRequest(), "lost-response")
	require.NoError(t, err)
	recovered, err := svc.GetByIdempotencyKey(ctx, owner, "lost-response")
	require.NoError(t, err)
	require.Equal(t, created.ID, recovered.ID)
	require.Len(t, provider.submits, 1)
	require.Equal(t, []string{created.ID}, queue.enqueued)

	for _, other := range []BatchImageOwner{{UserID: owner.UserID, APIKeyID: owner.APIKeyID + 1}, {UserID: owner.UserID + 1, APIKeyID: owner.APIKeyID}} {
		_, err = svc.GetByIdempotencyKey(ctx, other, "lost-response")
		require.ErrorIs(t, err, ErrBatchImageJobNotFound)
	}
	_, err = svc.GetByIdempotencyKey(ctx, owner, "unknown-request")
	require.ErrorIs(t, err, ErrBatchImageJobNotFound)
	_, err = svc.GetByIdempotencyKey(ctx, owner, "")
	require.ErrorIs(t, err, ErrBatchImageInvalidIdempotencyKey)
	now := time.Now()
	repo.jobs[created.ID].UserDeletedAt = &now
	_, err = svc.GetByIdempotencyKey(ctx, owner, "lost-response")
	require.ErrorIs(t, err, ErrBatchImageSubmissionRemoved)
}

// Another request claimed this user's key between the first lookup and insert.
// Store the competing row and let the service observe the real conflict outcome.
type competingBatchInsert struct {
	*fakeBatchImageRepository
	changeHash bool
}

func (r *competingBatchInsert) CreateBatchImageJob(ctx context.Context, p CreateBatchImageJobParams) (*BatchImageJob, error) {
	if r.changeHash {
		p.RequestHash = batchImageStringPtr("different-request-hash")
	}
	if _, err := r.fakeBatchImageRepository.CreateBatchImageJob(ctx, p); err != nil {
		return nil, err
	}
	return nil, ErrBatchImageJobExists
}

func TestBatchRecoveryConcurrentSubmitDoesNotRepeatHoldOrProviderCall(t *testing.T) {
	for _, changed := range []bool{false, true} {
		svc, repo, _, provider, _ := newTestBatchImagePublicService(true)
		svc.Repo = &competingBatchInsert{repo, changed}
		result, err := svc.Submit(context.Background(), testBatchImageOwner(), validBatchImageSubmitRequest(), "concurrent-submit")
		if changed {
			require.ErrorIs(t, err, ErrBatchImageIdempotencyConflict)
		} else {
			require.NoError(t, err)
			require.Contains(t, repo.jobs, result.ID)
		}
		require.Empty(t, provider.submits)
		require.Empty(t, svc.BillingRepo.(*fakeBatchImageBillingRepo).reserves)
	}
}
