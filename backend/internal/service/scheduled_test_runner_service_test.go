//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type scheduledTestPlanRepoStub struct {
	listDueCalls int
}

func (s *scheduledTestPlanRepoStub) Create(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	panic("unexpected Create call")
}

func (s *scheduledTestPlanRepoStub) GetByID(ctx context.Context, id int64) (*ScheduledTestPlan, error) {
	panic("unexpected GetByID call")
}

func (s *scheduledTestPlanRepoStub) ListByAccountID(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error) {
	panic("unexpected ListByAccountID call")
}

func (s *scheduledTestPlanRepoStub) ListDue(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error) {
	s.listDueCalls++
	return nil, nil
}

func (s *scheduledTestPlanRepoStub) Update(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	panic("unexpected Update call")
}

func (s *scheduledTestPlanRepoStub) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (s *scheduledTestPlanRepoStub) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	panic("unexpected UpdateAfterRun call")
}

func TestScheduledTestRunner_DoesNotListDueWhenDisabled(t *testing.T) {
	resetScheduledAccountTestsCache(t)

	planRepo := &scheduledTestPlanRepoStub{}
	settingRepo := &scheduledAccountTestsRepoStub{
		values: map[string]string{SettingKeyScheduledAccountTestsEnabled: "false"},
	}
	settingSvc := NewSettingService(settingRepo, &config.Config{})
	runner := NewScheduledTestRunnerService(planRepo, nil, nil, nil, settingSvc, &config.Config{})

	runner.runScheduled()

	require.Equal(t, 0, planRepo.listDueCalls)
	require.Equal(t, 1, settingRepo.calls)
}
