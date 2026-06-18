package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalizeAccountInspectionSettingsUsesSmallServerSafeBounds(t *testing.T) {
	settings := NormalizeAccountInspectionSettings(AccountInspectionSettings{
		BatchLimit:        10000,
		Concurrency:       99,
		BatchSleepSeconds: 99999,
		RecheckAfterHours: 24 * 365,
	})

	if settings.BatchLimit != maxAccountInspectionBatchLimit {
		t.Fatalf("batch limit = %d, want %d", settings.BatchLimit, maxAccountInspectionBatchLimit)
	}
	if settings.Concurrency != maxAccountInspectionConcurrency {
		t.Fatalf("concurrency = %d, want %d", settings.Concurrency, maxAccountInspectionConcurrency)
	}
	if settings.BatchSleepSeconds != maxAccountInspectionBatchSleepSeconds {
		t.Fatalf("batch sleep = %d, want %d", settings.BatchSleepSeconds, maxAccountInspectionBatchSleepSeconds)
	}
	if settings.RecheckAfterHours != maxAccountInspectionRecheckAfterHours {
		t.Fatalf("recheck hours = %d, want %d", settings.RecheckAfterHours, maxAccountInspectionRecheckAfterHours)
	}

	defaults := DefaultAccountInspectionSettings()
	if defaults.Enabled {
		t.Fatal("default account inspection should not start automatically")
	}
	if defaults.BatchLimit != defaultAccountInspectionBatchLimit {
		t.Fatalf("default batch limit = %d, want %d", defaults.BatchLimit, defaultAccountInspectionBatchLimit)
	}
	if defaults.Concurrency != 1 {
		t.Fatalf("default concurrency = %d, want 1", defaults.Concurrency)
	}
	if !defaults.LowResourceMode {
		t.Fatal("default account inspection should enable low resource protection")
	}
	if defaults.MaxAccountsPerRun != defaultAccountInspectionMaxAccountsPerRun {
		t.Fatalf("default max accounts per run = %d, want %d", defaults.MaxAccountsPerRun, defaultAccountInspectionMaxAccountsPerRun)
	}
}

func TestClassifyAccountHealthCheckErrorKeepsQuotaAsRateLimited(t *testing.T) {
	status, category, httpStatus, errorCode := ClassifyAccountHealthCheckError(
		"failed",
		`Responses API returned 429: {"error":{"code":"usage_limit_reached","message":"usage limit reached"}}`,
	)

	if status != AccountHealthStatusRateLimited {
		t.Fatalf("status = %q, want %q", status, AccountHealthStatusRateLimited)
	}
	if category != AccountHealthCategoryQuotaExhausted {
		t.Fatalf("category = %q, want %q", category, AccountHealthCategoryQuotaExhausted)
	}
	if httpStatus != 429 {
		t.Fatalf("http status = %d, want 429", httpStatus)
	}
	if errorCode != "usage_limit_reached" {
		t.Fatalf("error code = %q, want usage_limit_reached", errorCode)
	}
}

func TestClassifyAccountHealthCheckErrorSeparates402PaymentRequired(t *testing.T) {
	status, category, httpStatus, _ := ClassifyAccountHealthCheckError(
		"failed",
		"OpenAI returned 402 payment_required",
	)

	if status != AccountHealthStatusUnavailable {
		t.Fatalf("status = %q, want %q", status, AccountHealthStatusUnavailable)
	}
	if category != AccountHealthCategoryPaymentRequired {
		t.Fatalf("category = %q, want %q", category, AccountHealthCategoryPaymentRequired)
	}
	if httpStatus != 402 {
		t.Fatalf("http status = %d, want 402", httpStatus)
	}
}

func TestDecideAccountInspectionAction(t *testing.T) {
	settings := DefaultAccountInspectionSettings()

	tests := []struct {
		name         string
		result       AccountInspectionResult
		schedulable  bool
		autoDisabled bool
		want         string
	}{
		{
			name: "401 auth invalid is deleted",
			result: AccountInspectionResult{
				Status:     AccountHealthStatusUnavailable,
				Category:   AccountHealthCategoryAuthInvalid,
				HTTPStatus: 401,
			},
			want: AccountInspectionActionDelete,
		},
		{
			name: "quota exhausted is disabled",
			result: AccountInspectionResult{
				Status:   AccountHealthStatusRateLimited,
				Category: AccountHealthCategoryQuotaExhausted,
			},
			schedulable: true,
			want:        AccountInspectionActionDisable,
		},
		{
			name: "available auto-disabled account is restored",
			result: AccountInspectionResult{
				Status:   AccountHealthStatusAvailable,
				Category: AccountHealthCategoryAvailable,
			},
			autoDisabled: true,
			want:         AccountInspectionActionRestore,
		},
		{
			name: "available manually disabled account is left alone",
			result: AccountInspectionResult{
				Status:   AccountHealthStatusAvailable,
				Category: AccountHealthCategoryAvailable,
			},
			autoDisabled: false,
			want:         AccountInspectionActionNone,
		},
		{
			name: "402 payment required is not destructive by default",
			result: AccountInspectionResult{
				Status:     AccountHealthStatusUnavailable,
				Category:   AccountHealthCategoryPaymentRequired,
				HTTPStatus: 402,
			},
			want: AccountInspectionActionNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := DecideAccountInspectionAction(settings, tt.result, AccountInspectionCandidate{
				Schedulable:  tt.schedulable,
				AutoDisabled: tt.autoDisabled,
			}, time.Now())
			if got != tt.want {
				t.Fatalf("action = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecideAccountInspectionActionRetainsUntilDeleteRuleMatures(t *testing.T) {
	settings := DefaultAccountInspectionSettings()
	settings.DeleteQuotaExhausted = true
	settings.DeleteQuotaExhaustedAfterHours = 168
	settings.DeleteQuotaExhaustedMinConsecutive = 2
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	result := AccountInspectionResult{
		Status:   AccountHealthStatusRateLimited,
		Category: AccountHealthCategoryQuotaExhausted,
	}

	action, patch := DecideAccountInspectionAction(settings, result, AccountInspectionCandidate{}, now)
	if action != AccountInspectionActionRetain {
		t.Fatalf("first action = %q, want retain", action)
	}
	if patch.DeleteCandidateCount != 1 {
		t.Fatalf("first count = %d, want 1", patch.DeleteCandidateCount)
	}

	firstSeen := now.Add(-time.Hour)
	action, patch = DecideAccountInspectionAction(settings, result, AccountInspectionCandidate{
		DeleteCandidateCategory:    AccountHealthCategoryQuotaExhausted,
		DeleteCandidateFirstSeenAt: &firstSeen,
		DeleteCandidateCount:       1,
	}, now)
	if action != AccountInspectionActionPendingDelete {
		t.Fatalf("second action before hold = %q, want pending delete", action)
	}
	if patch.DeleteCandidateCount != 2 {
		t.Fatalf("second count = %d, want 2", patch.DeleteCandidateCount)
	}

	firstSeen = now.Add(-169 * time.Hour)
	action, _ = DecideAccountInspectionAction(settings, result, AccountInspectionCandidate{
		DeleteCandidateCategory:    AccountHealthCategoryQuotaExhausted,
		DeleteCandidateFirstSeenAt: &firstSeen,
		DeleteCandidateCount:       1,
	}, now)
	if action != AccountInspectionActionDelete {
		t.Fatalf("matured action = %q, want delete", action)
	}
}

func TestDecideAccountInspectionActionDeletesOtherFailureWhenRuleEnabled(t *testing.T) {
	settings := DefaultAccountInspectionSettings()
	result := AccountInspectionResult{
		Status:   AccountHealthStatusUnavailable,
		Category: AccountHealthCategoryProxyError,
	}

	action, _ := DecideAccountInspectionAction(settings, result, AccountInspectionCandidate{}, time.Now())
	if action != AccountInspectionActionNone {
		t.Fatalf("default action = %q, want none", action)
	}

	settings.DeleteOtherFailure = true
	settings.DeleteOtherFailureAfterHours = 0
	settings.DeleteOtherFailureMinConsecutive = 1

	action, patch := DecideAccountInspectionAction(settings, result, AccountInspectionCandidate{}, time.Now())
	if action != AccountInspectionActionDelete {
		t.Fatalf("enabled action = %q, want delete", action)
	}
	if patch.DeleteCandidateCategory != AccountHealthCategoryProxyError {
		t.Fatalf("delete candidate category = %q, want %q", patch.DeleteCandidateCategory, AccountHealthCategoryProxyError)
	}
}

func TestProcessCandidateDisablesQuotaAccountWhileWaitingForDeleteRule(t *testing.T) {
	settings := DefaultAccountInspectionSettings()
	settings.DeleteQuotaExhausted = true
	settings.DeleteQuotaExhaustedAfterHours = 168
	settings.DeleteQuotaExhaustedMinConsecutive = 2
	admin := &recordingInspectionAdminService{}
	repo := &recordingAccountInspectionRepository{}
	tester := &quotaExhaustedInspectionTester{}
	svc := NewAccountInspectionService(repo, admin, tester, nil)

	svc.processCandidate(context.Background(), 1, settings, AccountInspectionCandidate{
		AccountID:   42,
		Name:        "quota",
		Platform:    "openai",
		Type:        "auth",
		Schedulable: true,
	})

	if admin.disabledCount.Load() != 1 {
		t.Fatalf("disabled count = %d, want 1", admin.disabledCount.Load())
	}
	if repo.savedResult.Action != AccountInspectionActionRetain {
		t.Fatalf("saved action = %q, want retain", repo.savedResult.Action)
	}
	if repo.statePatch.AutoDisabled == nil || !*repo.statePatch.AutoDisabled {
		t.Fatal("state patch should mark the account as auto-disabled during quota observation")
	}
	if repo.statePatch.DeleteCandidateCategory != AccountHealthCategoryQuotaExhausted {
		t.Fatalf("delete candidate category = %q, want quota_exhausted", repo.statePatch.DeleteCandidateCategory)
	}
}

func TestSameAccountInspectionScanScope(t *testing.T) {
	current := DefaultAccountInspectionSettings()
	current.Cursor = 999
	current.Filters.Group = "13"
	next := current
	next.Cursor = 0

	if !sameAccountInspectionScanScope(next, current) {
		t.Fatal("same filters should keep the scan scope")
	}

	next.Filters.Group = "14"
	if sameAccountInspectionScanScope(next, current) {
		t.Fatal("changed group filter should reset the scan scope")
	}

	next = current
	next.RecheckAfterHours = current.RecheckAfterHours + 1
	if sameAccountInspectionScanScope(next, current) {
		t.Fatal("changed skip window should reset the scan scope")
	}

	next = current
	next.MaxAccountsPerRun = current.MaxAccountsPerRun / 2
	if !sameAccountInspectionScanScope(next, current) {
		t.Fatal("changed per-run throttle should keep the scan scope")
	}
}

func TestAccountInspectionRunKeepsInitialTotal(t *testing.T) {
	repo := &scriptedAccountInspectionRepository{
		settings: DefaultAccountInspectionSettings(),
		batches: []AccountInspectionCandidateBatch{
			{Items: []AccountInspectionCandidate{{AccountID: 1, Name: "one"}}, Cursor: 0, NextCursor: 1, HasMore: true, Total: 2},
			{Items: []AccountInspectionCandidate{{AccountID: 2, Name: "two"}}, Cursor: 1, NextCursor: 2, HasMore: false, Total: 1},
		},
	}
	tester := &successInspectionTester{}
	svc := NewAccountInspectionService(repo, noopAdminService{}, tester, nil)
	settings := repo.settings
	run := &AccountInspectionRun{
		ID:               1,
		Status:           AccountInspectionRunQueued,
		SettingsSnapshot: settings,
		TotalAccounts:    2,
	}

	svc.run(context.Background(), run)

	if run.TotalAccounts != 2 {
		t.Fatalf("run total = %d, want initial total 2", run.TotalAccounts)
	}
	if run.Status != AccountInspectionRunCompleted {
		t.Fatalf("run status = %q, want completed", run.Status)
	}
}

func TestStartRunResetsCursorForManualStart(t *testing.T) {
	settings := DefaultAccountInspectionSettings()
	settings.Cursor = 9876
	repo := &scriptedAccountInspectionRepository{
		settings:   settings,
		countTotal: 12,
		batches: []AccountInspectionCandidateBatch{
			{Items: nil, Cursor: 0, NextCursor: 0, HasMore: false, Total: 12},
		},
	}
	svc := NewAccountInspectionService(repo, noopAdminService{}, &successInspectionTester{}, nil)

	run, err := svc.StartRun(context.Background(), true)
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}

	if run.SettingsSnapshot.Cursor != 0 {
		t.Fatalf("run cursor = %d, want 0", run.SettingsSnapshot.Cursor)
	}
	if repo.createdSettings.Cursor != 0 {
		t.Fatalf("created settings cursor = %d, want 0", repo.createdSettings.Cursor)
	}
}

func TestStartRunKeepsCursorForAutomaticResume(t *testing.T) {
	settings := DefaultAccountInspectionSettings()
	settings.Cursor = 9876
	repo := &scriptedAccountInspectionRepository{
		settings:   settings,
		countTotal: 12,
		batches: []AccountInspectionCandidateBatch{
			{Items: nil, Cursor: 9876, NextCursor: 9876, HasMore: false, Total: 0},
		},
	}
	svc := NewAccountInspectionService(repo, noopAdminService{}, &successInspectionTester{}, nil)

	run, err := svc.StartRun(context.Background())
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}

	if run.SettingsSnapshot.Cursor != 9876 {
		t.Fatalf("run cursor = %d, want 9876", run.SettingsSnapshot.Cursor)
	}
	if repo.createdSettings.Cursor != 9876 {
		t.Fatalf("created settings cursor = %d, want 9876", repo.createdSettings.Cursor)
	}
}

func TestStartRunContinuesPausedRunWithMoreCandidates(t *testing.T) {
	settings := DefaultAccountInspectionSettings()
	settings.Cursor = 9876
	repo := &scriptedAccountInspectionRepository{
		settings:   settings,
		countTotal: 12,
		latestRun: &AccountInspectionRun{
			ID:         99,
			Status:     AccountInspectionRunPaused,
			NextCursor: 9876,
			HasMore:    true,
		},
		batches: []AccountInspectionCandidateBatch{
			{Items: nil, Cursor: 9876, NextCursor: 9876, HasMore: false, Total: 12},
		},
	}
	svc := NewAccountInspectionService(repo, noopAdminService{}, &successInspectionTester{}, nil)

	run, err := svc.StartRun(context.Background(), true)
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}

	if run.SettingsSnapshot.Cursor != 9876 {
		t.Fatalf("run cursor = %d, want 9876", run.SettingsSnapshot.Cursor)
	}
	if repo.createdSettings.Cursor != 9876 {
		t.Fatalf("created settings cursor = %d, want 9876", repo.createdSettings.Cursor)
	}
}

func TestAccountInspectionProcessBatchStopsDispatchingAfterCancel(t *testing.T) {
	tester := &blockingInspectionTester{
		started: make(chan struct{}),
	}
	svc := NewAccountInspectionService(&noopAccountInspectionRepository{}, nil, tester, nil)
	settings := DefaultAccountInspectionSettings()
	settings.Concurrency = 1
	settings.RestoreAutoDisabled = false

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var stats AccountInspectionBatchStats
	go func() {
		defer close(done)
		stats = svc.processBatch(ctx, 1, settings, []AccountInspectionCandidate{
			{AccountID: 1, Name: "one"},
			{AccountID: 2, Name: "two"},
		})
	}()

	select {
	case <-tester.started:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("first account test did not start")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("processBatch did not return after cancellation")
	}
	if got := tester.calls.Load(); got != 1 {
		t.Fatalf("tester calls = %d, want 1", got)
	}
	if stats.Processed != 1 {
		t.Fatalf("processed = %d, want 1", stats.Processed)
	}
}

func TestGetStatusStopsOrphanedActiveRun(t *testing.T) {
	startedAt := time.Now().Add(-5 * time.Minute)
	repo := &scriptedAccountInspectionRepository{
		settings: DefaultAccountInspectionSettings(),
		latestRun: &AccountInspectionRun{
			ID:            7,
			Status:        AccountInspectionRunStopping,
			TotalAccounts: 10,
			StartedAt:     &startedAt,
			CreatedAt:     startedAt,
			UpdatedAt:     startedAt,
		},
	}
	svc := NewAccountInspectionService(repo, noopAdminService{}, &successInspectionTester{}, nil)

	status, err := svc.GetStatus(context.Background())
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}

	if status.Running {
		t.Fatal("status should not report an orphaned stopping run as running")
	}
	if status.Run == nil {
		t.Fatal("status run is nil")
	}
	if status.Run.Status != AccountInspectionRunStopped {
		t.Fatalf("run status = %q, want %q", status.Run.Status, AccountInspectionRunStopped)
	}
	if status.Run.FinishedAt == nil {
		t.Fatal("finished_at should be set when orphaned run is stopped")
	}
	if len(repo.updates) != 1 {
		t.Fatalf("UpdateRun calls = %d, want 1", len(repo.updates))
	}
}

type blockingInspectionTester struct {
	calls   atomic.Int32
	started chan struct{}
}

func (t *blockingInspectionTester) RunTestBackground(ctx context.Context, accountID int64, modelID string) (*ScheduledTestResult, error) {
	if t.calls.Add(1) == 1 {
		close(t.started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return &ScheduledTestResult{Status: "success", LatencyMs: 1}, nil
}

type successInspectionTester struct{}

func (t *successInspectionTester) RunTestBackground(context.Context, int64, string) (*ScheduledTestResult, error) {
	return &ScheduledTestResult{Status: "success", LatencyMs: 1}, nil
}

type quotaExhaustedInspectionTester struct{}

func (t *quotaExhaustedInspectionTester) RunTestBackground(context.Context, int64, string) (*ScheduledTestResult, error) {
	return &ScheduledTestResult{
		Status:       "failed",
		ErrorMessage: `Responses API returned 429: {"error":{"code":"usage_limit_reached","message":"usage limit reached"}}`,
		LatencyMs:    1,
	}, nil
}

type noopAdminService struct {
	AdminService
}

func (noopAdminService) DeleteAccount(context.Context, int64) error {
	return nil
}

func (noopAdminService) SetAccountSchedulable(_ context.Context, id int64, schedulable bool) (*Account, error) {
	return &Account{ID: id, Status: StatusActive, Schedulable: schedulable}, nil
}

type noopAccountInspectionRepository struct{}

func (r *noopAccountInspectionRepository) GetSettings(context.Context) (AccountInspectionSettings, error) {
	return DefaultAccountInspectionSettings(), nil
}

func (r *noopAccountInspectionRepository) SaveSettings(_ context.Context, settings AccountInspectionSettings) (AccountInspectionSettings, error) {
	return settings, nil
}

func (r *noopAccountInspectionRepository) CreateRun(context.Context, AccountInspectionSettings, int64) (*AccountInspectionRun, error) {
	return nil, nil
}

func (r *noopAccountInspectionRepository) GetLatestRun(context.Context) (*AccountInspectionRun, error) {
	return nil, nil
}

func (r *noopAccountInspectionRepository) GetRun(context.Context, int64) (*AccountInspectionRun, error) {
	return nil, nil
}

func (r *noopAccountInspectionRepository) UpdateRun(context.Context, *AccountInspectionRun) error {
	return nil
}

func (r *noopAccountInspectionRepository) ListCandidates(context.Context, AccountInspectionSettings) (AccountInspectionCandidateBatch, error) {
	return AccountInspectionCandidateBatch{}, nil
}

func (r *noopAccountInspectionRepository) CountCandidates(context.Context, AccountInspectionSettings) (int64, error) {
	return 0, nil
}

func (r *noopAccountInspectionRepository) SaveResult(context.Context, AccountInspectionResult) error {
	return nil
}

func (r *noopAccountInspectionRepository) UpdateState(context.Context, AccountInspectionResult, AccountInspectionStatePatch) error {
	return nil
}

func (r *noopAccountInspectionRepository) GetAccountAutoDisabled(context.Context, int64) (bool, error) {
	return false, nil
}

func (r *noopAccountInspectionRepository) ListRecentResults(context.Context, int64, int) ([]AccountInspectionResult, error) {
	return nil, nil
}

func (r *noopAccountInspectionRepository) ListLogs(context.Context, int) ([]AccountInspectionLog, error) {
	return nil, nil
}

func (r *noopAccountInspectionRepository) GetSummary(context.Context, int64, int64) (AccountInspectionSummary, error) {
	return AccountInspectionSummary{}, nil
}

func (r *noopAccountInspectionRepository) RecordLog(context.Context, *int64, string, string) error {
	return nil
}

func (r *noopAccountInspectionRepository) Prune(context.Context, int, int) error {
	return nil
}

type scriptedAccountInspectionRepository struct {
	noopAccountInspectionRepository
	settings        AccountInspectionSettings
	batches         []AccountInspectionCandidateBatch
	updates         []*AccountInspectionRun
	countTotal      int64
	countSettings   AccountInspectionSettings
	createdSettings AccountInspectionSettings
	latestRun       *AccountInspectionRun
}

func (r *scriptedAccountInspectionRepository) GetSettings(context.Context) (AccountInspectionSettings, error) {
	return r.settings, nil
}

func (r *scriptedAccountInspectionRepository) SaveSettings(_ context.Context, settings AccountInspectionSettings) (AccountInspectionSettings, error) {
	r.settings = settings
	return settings, nil
}

func (r *scriptedAccountInspectionRepository) CreateRun(_ context.Context, settings AccountInspectionSettings, total int64) (*AccountInspectionRun, error) {
	r.createdSettings = settings
	return &AccountInspectionRun{
		ID:               1,
		Status:           AccountInspectionRunQueued,
		SettingsSnapshot: settings,
		TotalAccounts:    total,
		Cursor:           settings.Cursor,
		NextCursor:       settings.Cursor,
	}, nil
}

func (r *scriptedAccountInspectionRepository) GetLatestRun(context.Context) (*AccountInspectionRun, error) {
	return r.latestRun, nil
}

func (r *scriptedAccountInspectionRepository) CountCandidates(_ context.Context, settings AccountInspectionSettings) (int64, error) {
	r.countSettings = settings
	return r.countTotal, nil
}

func (r *scriptedAccountInspectionRepository) ListCandidates(context.Context, AccountInspectionSettings) (AccountInspectionCandidateBatch, error) {
	if len(r.batches) == 0 {
		return AccountInspectionCandidateBatch{}, nil
	}
	batch := r.batches[0]
	r.batches = r.batches[1:]
	return batch, nil
}

func (r *scriptedAccountInspectionRepository) UpdateRun(_ context.Context, run *AccountInspectionRun) error {
	cp := *run
	r.updates = append(r.updates, &cp)
	if r.latestRun != nil && r.latestRun.ID == run.ID {
		r.latestRun = &cp
	}
	return nil
}

func (r *scriptedAccountInspectionRepository) SaveResult(context.Context, AccountInspectionResult) error {
	return nil
}

func (r *scriptedAccountInspectionRepository) UpdateState(context.Context, AccountInspectionResult, AccountInspectionStatePatch) error {
	return nil
}

func (r *scriptedAccountInspectionRepository) RecordLog(context.Context, *int64, string, string) error {
	return nil
}

func (r *scriptedAccountInspectionRepository) Prune(context.Context, int, int) error {
	return nil
}

type recordingInspectionAdminService struct {
	noopAdminService
	disabledCount atomic.Int32
}

func (s *recordingInspectionAdminService) SetAccountSchedulable(_ context.Context, id int64, schedulable bool) (*Account, error) {
	if !schedulable {
		s.disabledCount.Add(1)
	}
	return &Account{ID: id, Status: StatusActive, Schedulable: schedulable}, nil
}

type recordingAccountInspectionRepository struct {
	noopAccountInspectionRepository
	savedResult AccountInspectionResult
	statePatch  AccountInspectionStatePatch
}

func (r *recordingAccountInspectionRepository) SaveResult(_ context.Context, result AccountInspectionResult) error {
	r.savedResult = result
	return nil
}

func (r *recordingAccountInspectionRepository) UpdateState(_ context.Context, _ AccountInspectionResult, patch AccountInspectionStatePatch) error {
	r.statePatch = patch
	return nil
}
