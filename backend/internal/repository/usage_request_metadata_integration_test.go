//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmeta"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUsageRequestMetadataDatabaseRoundTrip(t *testing.T) {
	client := testEntClient(t)
	repo := newUsageLogRepositoryWithSQL(client, integrationDB)
	user := mustCreateUser(t, client, &service.User{Email: "request-meta-" + uuid.NewString() + "@example.com"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-meta-" + uuid.NewString(), Name: "metadata"})
	account := mustCreateAccount(t, client, &service.Account{Name: "request-meta-" + uuid.NewString()})
	for _, captured := range []bool{false, true} {
		ctx := context.Background()
		if captured {
			var capture *requestmeta.Capture
			ctx, capture = requestmeta.Begin(ctx, "POST")
			capture.RecordStatus(201)
			requestmeta.RecordAttempt(ctx)
			requestmeta.RecordAttempt(ctx)
			ctx = requestmeta.SnapshotContext(ctx, context.Background())
		}
		log := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: uuid.NewString(), Model: "gpt-5.1", RateMultiplier: 1, CreatedAt: time.Now().UTC()}
		inserted, err := repo.Create(ctx, log)
		require.NoError(t, err)
		require.True(t, inserted)
		stored, err := repo.GetByID(context.Background(), log.ID)
		require.NoError(t, err)
		if captured {
			require.Equal(t, "POST", *stored.HTTPMethod)
			require.Equal(t, 201, *stored.StatusCode)
			require.Equal(t, 2, *stored.AttemptCount)
		} else {
			require.Nil(t, stored.HTTPMethod)
			require.Nil(t, stored.StatusCode)
			require.Nil(t, stored.AttemptCount)
		}
	}
}
