package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIUsagePreservesObservedTurnAttempts(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	svc := newOpenAIRecordUsageServiceForTest(usageRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	attempts := 3
	require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		APIKey:  &APIKey{ID: 2, User: &User{ID: 1}},
		User:    &User{ID: 1},
		Account: &Account{ID: 3},
		Result: &OpenAIForwardResult{
			RequestID: "observed-ws-attempts", Model: "gpt-5.1", OpenAIWSMode: true,
			Usage: OpenAIUsage{InputTokens: 1, OutputTokens: 1}, AttemptCount: &attempts,
		},
	}))
	require.NotNil(t, usageRepo.lastLog)
	require.NotNil(t, usageRepo.lastLog.AttemptCount)
	require.Equal(t, 3, *usageRepo.lastLog.AttemptCount)
}
