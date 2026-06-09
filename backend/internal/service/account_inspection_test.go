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
			want: AccountInspectionActionDisable,
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
			if got := DecideAccountInspectionAction(settings, tt.result, tt.autoDisabled); got != tt.want {
				t.Fatalf("action = %q, want %q", got, tt.want)
			}
		})
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
	go func() {
		defer close(done)
		svc.processBatch(ctx, 1, settings, []AccountInspectionCandidate{
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
	settings AccountInspectionSettings
	batches  []AccountInspectionCandidateBatch
	updates  []*AccountInspectionRun
}

func (r *scriptedAccountInspectionRepository) GetSettings(context.Context) (AccountInspectionSettings, error) {
	return r.settings, nil
}

func (r *scriptedAccountInspectionRepository) SaveSettings(_ context.Context, settings AccountInspectionSettings) (AccountInspectionSettings, error) {
	r.settings = settings
	return settings, nil
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
