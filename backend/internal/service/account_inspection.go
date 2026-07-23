package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	AccountInspectionRunQueued    = "queued"
	AccountInspectionRunRunning   = "running"
	AccountInspectionRunStopping  = "stopping"
	AccountInspectionRunStopped   = "stopped"
	AccountInspectionRunPaused    = "paused"
	AccountInspectionRunCompleted = "completed"
	AccountInspectionRunFailed    = "failed"

	AccountInspectionActionNone          = "none"
	AccountInspectionActionDelete        = "delete"
	AccountInspectionActionRetain        = "retain"
	AccountInspectionActionPendingDelete = "delete_pending"
	AccountInspectionActionDisable       = "disable"
	AccountInspectionActionRestore       = "restore"

	AccountInspectionLogInfo  = "info"
	AccountInspectionLogWarn  = "warn"
	AccountInspectionLogError = "error"
)

const (
	defaultAccountInspectionBatchLimit             = 50
	defaultAccountInspectionConcurrency            = 1
	defaultAccountInspectionBatchSleepSeconds      = 30
	defaultAccountInspectionRecheckAfterHours      = 168
	defaultAccountInspectionMaxAccountsPerRun      = 1000
	defaultAccountInspectionAutoPauseErrors        = 3
	defaultAccountInspectionDeleteMinConsecutive   = 1
	defaultAccountInspectionDeleteOtherConsecutive = 3
	defaultAccountInspectionDeleteQuotaHours       = 168
	defaultAccountInspectionDeletePaymentHours     = 168
	defaultAccountInspectionDeleteOtherHours       = 168
	defaultAccountInspectionLogRetention           = 1000
	defaultAccountInspectionResultRetention        = 5000
	maxAccountInspectionBatchLimit                 = 500
	maxAccountInspectionConcurrency                = 5
	maxAccountInspectionBatchSleepSeconds          = 3600
	maxAccountInspectionRecheckAfterHours          = 24 * 90
	maxAccountInspectionMaxAccountsPerRun          = 100000
	maxAccountInspectionAutoPauseErrors            = 100
)

type AccountInspectionTester interface {
	RunTestBackground(ctx context.Context, accountID int64, modelID string) (*ScheduledTestResult, error)
}

type AccountInspectionFilters struct {
	Platform    string `json:"platform"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Group       string `json:"group"`
	Search      string `json:"search"`
	PrivacyMode string `json:"privacy_mode"`
}

type AccountInspectionSettings struct {
	Enabled                             bool                     `json:"enabled"`
	Filters                             AccountInspectionFilters `json:"filters"`
	ModelID                             string                   `json:"model_id"`
	BatchLimit                          int                      `json:"batch_limit"`
	Concurrency                         int                      `json:"concurrency"`
	BatchSleepSeconds                   int                      `json:"batch_sleep_seconds"`
	RecheckAfterHours                   int                      `json:"recheck_after_hours"`
	LowResourceMode                     bool                     `json:"low_resource_mode"`
	MaxAccountsPerRun                   int                      `json:"max_accounts_per_run"`
	AutoPauseConsecutiveErrors          int                      `json:"auto_pause_consecutive_errors"`
	IncludeUnschedulable                bool                     `json:"include_unschedulable"`
	DeleteAuthInvalid                   bool                     `json:"delete_auth_invalid"`
	DeleteAuthInvalidMinConsecutive     int                      `json:"delete_auth_invalid_min_consecutive"`
	DeleteAuthInvalidAfterHours         int                      `json:"delete_auth_invalid_after_hours"`
	DeleteQuotaExhausted                bool                     `json:"delete_quota_exhausted"`
	DeleteQuotaExhaustedMinConsecutive  int                      `json:"delete_quota_exhausted_min_consecutive"`
	DeleteQuotaExhaustedAfterHours      int                      `json:"delete_quota_exhausted_after_hours"`
	DeletePaymentRequired               bool                     `json:"delete_payment_required"`
	DeletePaymentRequiredMinConsecutive int                      `json:"delete_payment_required_min_consecutive"`
	DeletePaymentRequiredAfterHours     int                      `json:"delete_payment_required_after_hours"`
	DeleteOtherFailure                  bool                     `json:"delete_other_failure"`
	DeleteOtherFailureMinConsecutive    int                      `json:"delete_other_failure_min_consecutive"`
	DeleteOtherFailureAfterHours        int                      `json:"delete_other_failure_after_hours"`
	DisableQuotaExhausted               bool                     `json:"disable_quota_exhausted"`
	RestoreAutoDisabled                 bool                     `json:"restore_auto_disabled"`
	Cursor                              int64                    `json:"cursor"`
	UpdatedAt                           time.Time                `json:"updated_at"`
}

type AccountInspectionCandidate struct {
	AccountID                  int64
	Name                       string
	Platform                   string
	Type                       string
	Schedulable                bool
	AutoDisabled               bool
	DeleteCandidateCategory    string
	DeleteCandidateFirstSeenAt *time.Time
	DeleteCandidateCount       int
}

type AccountInspectionCandidateBatch struct {
	Items      []AccountInspectionCandidate
	Cursor     int64
	NextCursor int64
	HasMore    bool
	Total      int64
}

type AccountInspectionRun struct {
	ID               int64                     `json:"id"`
	Status           string                    `json:"status"`
	SettingsSnapshot AccountInspectionSettings `json:"settings_snapshot"`
	TotalAccounts    int64                     `json:"total_accounts"`
	Cursor           int64                     `json:"cursor"`
	NextCursor       int64                     `json:"next_cursor"`
	HasMore          bool                      `json:"has_more"`
	Error            string                    `json:"error,omitempty"`
	StartedAt        *time.Time                `json:"started_at,omitempty"`
	FinishedAt       *time.Time                `json:"finished_at,omitempty"`
	CreatedAt        time.Time                 `json:"created_at"`
	UpdatedAt        time.Time                 `json:"updated_at"`
}

type AccountInspectionResult struct {
	ID          int64      `json:"id"`
	RunID       int64      `json:"run_id"`
	AccountID   int64      `json:"account_id"`
	Name        string     `json:"name"`
	Platform    string     `json:"platform"`
	Type        string     `json:"type"`
	Status      string     `json:"status"`
	Category    string     `json:"category"`
	HTTPStatus  int        `json:"http_status,omitempty"`
	ErrorCode   string     `json:"error_code,omitempty"`
	Message     string     `json:"message,omitempty"`
	LatencyMs   int64      `json:"latency_ms,omitempty"`
	Action      string     `json:"action"`
	ActionError string     `json:"action_error,omitempty"`
	CheckedAt   time.Time  `json:"checked_at"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
}

type AccountInspectionLog struct {
	ID        int64     `json:"id"`
	RunID     *int64    `json:"run_id,omitempty"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type AccountInspectionSummary struct {
	Total          int64          `json:"total"`
	Checked        int64          `json:"checked"`
	Available      int64          `json:"available"`
	RateLimited    int64          `json:"rate_limited"`
	Unavailable    int64          `json:"unavailable"`
	QuotaExhausted int64          `json:"quota_exhausted"`
	Auth401        int64          `json:"auth_401"`
	Payment402     int64          `json:"payment_402"`
	OtherFailure   int64          `json:"other_failure"`
	Unknown        int64          `json:"unknown"`
	Deleted        int64          `json:"deleted"`
	Retained       int64          `json:"retained"`
	PendingDelete  int64          `json:"pending_delete"`
	Disabled       int64          `json:"disabled"`
	Restored       int64          `json:"restored"`
	ActionFailed   int64          `json:"action_failed"`
	ByCategory     map[string]int `json:"by_category"`
}

type AccountInspectionStatus struct {
	Settings       AccountInspectionSettings `json:"settings"`
	Run            *AccountInspectionRun     `json:"run,omitempty"`
	Summary        AccountInspectionSummary  `json:"summary"`
	CandidateTotal int64                     `json:"candidate_total"`
	Running        bool                      `json:"running"`
	RecentResults  []AccountInspectionResult `json:"recent_results"`
	Logs           []AccountInspectionLog    `json:"logs"`
}

type AccountInspectionStatePatch struct {
	AutoDisabled               *bool
	Reason                     string
	At                         time.Time
	UpdateDeleteCandidate      bool
	DeleteCandidateCategory    string
	DeleteCandidateFirstSeenAt *time.Time
	DeleteCandidateCount       int
}

type AccountInspectionBatchStats struct {
	Processed   int
	InfraErrors int
}

type AccountInspectionRepository interface {
	GetSettings(ctx context.Context) (AccountInspectionSettings, error)
	SaveSettings(ctx context.Context, settings AccountInspectionSettings) (AccountInspectionSettings, error)
	CreateRun(ctx context.Context, settings AccountInspectionSettings, total int64) (*AccountInspectionRun, error)
	GetLatestRun(ctx context.Context) (*AccountInspectionRun, error)
	GetRun(ctx context.Context, id int64) (*AccountInspectionRun, error)
	UpdateRun(ctx context.Context, run *AccountInspectionRun) error
	ListCandidates(ctx context.Context, settings AccountInspectionSettings) (AccountInspectionCandidateBatch, error)
	CountCandidates(ctx context.Context, settings AccountInspectionSettings) (int64, error)
	SaveResult(ctx context.Context, result AccountInspectionResult) error
	UpdateState(ctx context.Context, result AccountInspectionResult, patch AccountInspectionStatePatch) error
	GetAccountAutoDisabled(ctx context.Context, accountID int64) (bool, error)
	ListRecentResults(ctx context.Context, runID int64, limit int) ([]AccountInspectionResult, error)
	ListLogs(ctx context.Context, limit int) ([]AccountInspectionLog, error)
	GetSummary(ctx context.Context, runID int64, total int64) (AccountInspectionSummary, error)
	RecordLog(ctx context.Context, runID *int64, level, message string) error
	Prune(ctx context.Context, keepLogs, keepResults int) error
}

type AccountInspectionService struct {
	repo         AccountInspectionRepository
	adminSvc     AdminService
	tester       AccountInspectionTester
	rateLimitSvc *RateLimitService

	mu        sync.Mutex
	cancel    context.CancelFunc
	runningID int64
	startOnce sync.Once
}

func NewAccountInspectionService(
	repo AccountInspectionRepository,
	adminSvc AdminService,
	tester AccountInspectionTester,
	rateLimitSvc *RateLimitService,
) *AccountInspectionService {
	return &AccountInspectionService{
		repo:         repo,
		adminSvc:     adminSvc,
		tester:       tester,
		rateLimitSvc: rateLimitSvc,
	}
}

func ProvideAccountInspectionService(
	repo AccountInspectionRepository,
	adminSvc AdminService,
	tester *AccountTestService,
	rateLimitSvc *RateLimitService,
) *AccountInspectionService {
	svc := NewAccountInspectionService(repo, adminSvc, tester, rateLimitSvc)
	svc.Start()
	return svc
}

func (s *AccountInspectionService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		go func() {
			time.Sleep(3 * time.Second)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			settings, err := s.repo.GetSettings(ctx)
			if err != nil {
				slog.Warn("account_inspection.load_settings_failed", "error", err)
				return
			}
			if settings.Enabled {
				if _, err := s.StartRun(context.Background()); err != nil {
					slog.Warn("account_inspection.resume_failed", "error", err)
				}
			}
		}()
	})
}

func (s *AccountInspectionService) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	cancel := s.cancel
	s.cancel = nil
	s.runningID = 0
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (s *AccountInspectionService) GetStatus(ctx context.Context) (AccountInspectionStatus, error) {
	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return AccountInspectionStatus{}, err
	}
	settings = NormalizeAccountInspectionSettings(settings)

	run, err := s.repo.GetLatestRun(ctx)
	if err != nil {
		return AccountInspectionStatus{}, err
	}
	run = s.reconcileOrphanedActiveRun(ctx, run)

	runID := int64(0)
	total := int64(0)
	if run != nil {
		runID = run.ID
		total = run.TotalAccounts
	}
	summary, err := s.repo.GetSummary(ctx, runID, total)
	if err != nil {
		return AccountInspectionStatus{}, err
	}

	candidateTotal, err := s.repo.CountCandidates(ctx, settings)
	if err != nil {
		candidateTotal = 0
	}
	results, _ := s.repo.ListRecentResults(ctx, runID, 30)
	logs, _ := s.repo.ListLogs(ctx, 120)

	return AccountInspectionStatus{
		Settings:       settings,
		Run:            run,
		Summary:        summary,
		CandidateTotal: candidateTotal,
		Running:        run != nil && isAccountInspectionActiveStatus(run.Status),
		RecentResults:  results,
		Logs:           logs,
	}, nil
}

func (s *AccountInspectionService) UpdateSettings(ctx context.Context, settings AccountInspectionSettings) (AccountInspectionSettings, error) {
	settings = NormalizeAccountInspectionSettings(settings)
	current, err := s.repo.GetSettings(ctx)
	if err == nil {
		settings.Enabled = current.Enabled
		if sameAccountInspectionScanScope(settings, current) && settings.Cursor == 0 {
			settings.Cursor = current.Cursor
		}
		if !sameAccountInspectionScanScope(settings, current) {
			settings.Cursor = 0
		}
	}
	return s.repo.SaveSettings(ctx, settings)
}

func (s *AccountInspectionService) StartRun(ctx context.Context, resetCursor ...bool) (*AccountInspectionRun, error) {
	if s == nil || s.repo == nil || s.adminSvc == nil || s.tester == nil {
		return nil, errors.New("account inspection service is not configured")
	}

	s.mu.Lock()
	if s.cancel != nil {
		s.mu.Unlock()
		return nil, errors.New("account inspection is already running")
	}
	s.mu.Unlock()

	settings, err := s.repo.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	settings = NormalizeAccountInspectionSettings(settings)
	settings.Enabled = true
	if len(resetCursor) > 0 && resetCursor[0] {
		latestRun, err := s.repo.GetLatestRun(ctx)
		if err != nil {
			return nil, err
		}
		if latestRun == nil || latestRun.Status != AccountInspectionRunPaused || !latestRun.HasMore {
			settings.Cursor = 0
		}
	}
	settings, err = s.repo.SaveSettings(ctx, settings)
	if err != nil {
		return nil, err
	}

	total, err := s.repo.CountCandidates(ctx, settings)
	if err != nil {
		return nil, err
	}
	run, err := s.repo.CreateRun(ctx, settings, total)
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.cancel = cancel
	s.runningID = run.ID
	s.mu.Unlock()

	go s.run(runCtx, run)
	return run, nil
}

func (s *AccountInspectionService) StopRun(ctx context.Context) (*AccountInspectionRun, error) {
	settings, err := s.repo.GetSettings(ctx)
	if err == nil {
		settings.Enabled = false
		_, _ = s.repo.SaveSettings(ctx, settings)
	}

	s.mu.Lock()
	cancel := s.cancel
	runID := s.runningID
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if runID > 0 {
		if run, err := s.repo.GetRun(ctx, runID); err == nil && run != nil && run.Status == AccountInspectionRunRunning {
			run.Status = AccountInspectionRunStopping
			run.UpdatedAt = time.Now()
			_ = s.repo.UpdateRun(ctx, run)
		}
	}
	run, err := s.repo.GetLatestRun(ctx)
	if err != nil {
		return nil, err
	}
	return s.reconcileOrphanedActiveRun(ctx, run), nil
}

func (s *AccountInspectionService) run(ctx context.Context, run *AccountInspectionRun) {
	defer func() {
		s.mu.Lock()
		if s.runningID == run.ID {
			s.cancel = nil
			s.runningID = 0
		}
		s.mu.Unlock()
	}()

	run.Status = AccountInspectionRunRunning
	now := time.Now()
	run.StartedAt = &now
	run.UpdatedAt = now
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		slog.Warn("account_inspection.mark_run_running_failed", "run_id", run.ID, "error", err)
	}
	s.log(ctx, run.ID, AccountInspectionLogInfo, fmt.Sprintf("inspection started: batch=%d concurrency=%d sleep=%ds cursor=%d max=%d", run.SettingsSnapshot.BatchLimit, run.SettingsSnapshot.Concurrency, run.SettingsSnapshot.BatchSleepSeconds, run.SettingsSnapshot.Cursor, run.SettingsSnapshot.MaxAccountsPerRun))

	settings := run.SettingsSnapshot
	processedThisRun := 0
	consecutiveInfraErrors := 0
	for {
		if ctx.Err() != nil {
			s.finishRun(context.Background(), run, AccountInspectionRunStopped, "")
			return
		}

		if settings.MaxAccountsPerRun > 0 && processedThisRun >= settings.MaxAccountsPerRun {
			settings.Enabled = false
			_, _ = s.repo.SaveSettings(context.Background(), settings)
			s.log(context.Background(), run.ID, AccountInspectionLogWarn, fmt.Sprintf("inspection paused: reached per-run limit %d; click start again to continue from cursor=%d", settings.MaxAccountsPerRun, settings.Cursor))
			s.finishRun(context.Background(), run, AccountInspectionRunPaused, "per-run account limit reached")
			return
		}

		batchSettings := settings
		if settings.MaxAccountsPerRun > 0 {
			remaining := settings.MaxAccountsPerRun - processedThisRun
			if remaining > 0 && remaining < batchSettings.BatchLimit {
				batchSettings.BatchLimit = remaining
			}
		}
		batch, err := s.repo.ListCandidates(ctx, batchSettings)
		if err != nil {
			s.log(context.Background(), run.ID, AccountInspectionLogError, "load accounts failed: "+err.Error())
			if isAccountInspectionInfraError(err) {
				settings.Enabled = false
				_, _ = s.repo.SaveSettings(context.Background(), settings)
				s.finishRun(context.Background(), run, AccountInspectionRunPaused, err.Error())
				return
			}
			s.finishRun(context.Background(), run, AccountInspectionRunFailed, err.Error())
			return
		}

		if run.TotalAccounts <= 0 {
			run.TotalAccounts = batch.Total
		}
		run.Cursor = batch.Cursor
		run.NextCursor = batch.NextCursor
		run.HasMore = batch.HasMore
		_ = s.repo.UpdateRun(ctx, run)

		if len(batch.Items) == 0 {
			settings.Enabled = false
			settings.Cursor = 0
			_, _ = s.repo.SaveSettings(context.Background(), settings)
			s.log(context.Background(), run.ID, AccountInspectionLogInfo, "no more accounts need inspection; run completed")
			s.finishRun(context.Background(), run, AccountInspectionRunCompleted, "")
			return
		}

		s.log(ctx, run.ID, AccountInspectionLogInfo, fmt.Sprintf("loaded batch: accounts=%d cursor=%d", len(batch.Items), settings.Cursor))
		stats := s.processBatch(ctx, run.ID, settings, batch.Items)
		processedThisRun += stats.Processed
		if ctx.Err() != nil {
			s.finishRun(context.Background(), run, AccountInspectionRunStopped, "")
			return
		}
		if stats.InfraErrors > 0 {
			consecutiveInfraErrors++
			s.log(context.Background(), run.ID, AccountInspectionLogWarn, fmt.Sprintf("batch completed with infrastructure errors: %d consecutive_error_batches=%d", stats.InfraErrors, consecutiveInfraErrors))
		} else {
			consecutiveInfraErrors = 0
		}

		settings.Cursor = batch.NextCursor
		settings.Enabled = true
		if _, err := s.repo.SaveSettings(context.Background(), settings); err != nil {
			slog.Warn("account_inspection.save_cursor_failed", "run_id", run.ID, "cursor", settings.Cursor, "error", err)
		}
		run.NextCursor = batch.NextCursor
		run.HasMore = batch.HasMore
		_ = s.repo.UpdateRun(context.Background(), run)
		_ = s.repo.Prune(context.Background(), defaultAccountInspectionLogRetention, defaultAccountInspectionResultRetention)

		if settings.AutoPauseConsecutiveErrors > 0 && consecutiveInfraErrors >= settings.AutoPauseConsecutiveErrors {
			settings.Enabled = false
			_, _ = s.repo.SaveSettings(context.Background(), settings)
			s.log(context.Background(), run.ID, AccountInspectionLogError, fmt.Sprintf("inspection paused: %d consecutive batches had database/redis style errors; cursor=%d", consecutiveInfraErrors, settings.Cursor))
			s.finishRun(context.Background(), run, AccountInspectionRunPaused, "paused by infrastructure error protection")
			return
		}

		if !batch.HasMore {
			settings.Enabled = false
			settings.Cursor = 0
			_, _ = s.repo.SaveSettings(context.Background(), settings)
			s.log(context.Background(), run.ID, AccountInspectionLogInfo, "current filter scope has been fully inspected")
			s.finishRun(context.Background(), run, AccountInspectionRunCompleted, "")
			return
		}

		if !sleepAccountInspectionWithContext(ctx, time.Duration(settings.BatchSleepSeconds)*time.Second) {
			s.finishRun(context.Background(), run, AccountInspectionRunStopped, "")
			return
		}
	}
}

func (s *AccountInspectionService) processBatch(ctx context.Context, runID int64, settings AccountInspectionSettings, candidates []AccountInspectionCandidate) AccountInspectionBatchStats {
	sem := make(chan struct{}, settings.Concurrency)
	var wg sync.WaitGroup
	infraErrors := make(chan bool, len(candidates))
accountLoop:
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			break
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break accountLoop
		}
		c := candidate
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			infraErrors <- s.processCandidate(ctx, runID, settings, c)
		}()
	}
	wg.Wait()
	close(infraErrors)

	stats := AccountInspectionBatchStats{}
	for infraError := range infraErrors {
		stats.Processed++
		if infraError {
			stats.InfraErrors++
		}
	}
	return stats
}

func (s *AccountInspectionService) processCandidate(ctx context.Context, runID int64, settings AccountInspectionSettings, candidate AccountInspectionCandidate) bool {
	s.log(ctx, runID, AccountInspectionLogInfo, fmt.Sprintf("testing account: %s (%d)", candidate.Name, candidate.AccountID))
	started := time.Now()

	testCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	testResult, err := s.tester.RunTestBackground(testCtx, candidate.AccountID, settings.ModelID)
	finished := time.Now()
	if ctx.Err() != nil {
		return false
	}

	message := ""
	latencyMs := finished.Sub(started).Milliseconds()
	testStatus := "failed"
	if testResult != nil {
		message = testResult.ErrorMessage
		latencyMs = testResult.LatencyMs
		testStatus = testResult.Status
	}
	if err != nil && message == "" {
		message = err.Error()
	}
	status, category, httpStatus, errorCode := ClassifyAccountHealthCheckError(testStatus, message)

	result := AccountInspectionResult{
		RunID:      runID,
		AccountID:  candidate.AccountID,
		Name:       candidate.Name,
		Platform:   candidate.Platform,
		Type:       candidate.Type,
		Status:     status,
		Category:   category,
		HTTPStatus: httpStatus,
		ErrorCode:  errorCode,
		Message:    message,
		LatencyMs:  latencyMs,
		Action:     AccountInspectionActionNone,
		CheckedAt:  finished,
	}

	action, statePatch := DecideAccountInspectionAction(settings, result, candidate, finished)
	result.Action = action
	skipStateUpdate := false
	infraError := false
	if actionRequiresAccountInspectionApply(action) {
		if actionErr := s.applyAction(ctx, action, candidate, result); actionErr != nil {
			result.ActionError = actionErr.Error()
			if isAccountInspectionInfraError(actionErr) {
				infraError = true
			}
			s.log(ctx, runID, AccountInspectionLogWarn, fmt.Sprintf("action failed: %s (%d) action=%s err=%v", candidate.Name, candidate.AccountID, action, actionErr))
		} else {
			s.log(ctx, runID, AccountInspectionLogInfo, fmt.Sprintf("action applied: %s (%d) action=%s", candidate.Name, candidate.AccountID, action))
			switch action {
			case AccountInspectionActionDelete:
				skipStateUpdate = true
			case AccountInspectionActionDisable:
				if candidate.Schedulable || candidate.AutoDisabled {
					v := true
					statePatch.AutoDisabled = &v
					statePatch.Reason = category
				}
			case AccountInspectionActionRestore:
				v := false
				statePatch.AutoDisabled = &v
				statePatch.Reason = ""
			}
		}
	} else if action != AccountInspectionActionNone {
		s.log(ctx, runID, AccountInspectionLogInfo, fmt.Sprintf("action queued: %s (%d) action=%s", candidate.Name, candidate.AccountID, action))
	}

	if shouldDisableQuotaAccountDuringInspection(settings, result, candidate, action) {
		if actionErr := s.applyAction(ctx, AccountInspectionActionDisable, candidate, result); actionErr != nil {
			result.ActionError = actionErr.Error()
			if isAccountInspectionInfraError(actionErr) {
				infraError = true
			}
			s.log(ctx, runID, AccountInspectionLogWarn, fmt.Sprintf("quota protection failed: %s (%d) err=%v", candidate.Name, candidate.AccountID, actionErr))
		} else {
			v := true
			statePatch.AutoDisabled = &v
			statePatch.Reason = category
			s.log(ctx, runID, AccountInspectionLogInfo, fmt.Sprintf("quota protection applied: %s (%d) action=%s", candidate.Name, candidate.AccountID, AccountInspectionActionDisable))
		}
	}

	if status == AccountHealthStatusAvailable && s.rateLimitSvc != nil {
		_, _ = s.rateLimitSvc.RecoverAccountAfterSuccessfulTest(ctx, candidate.AccountID)
	}

	if err := s.repo.SaveResult(context.Background(), result); err != nil {
		infraError = true
		slog.Warn("account_inspection.save_result_failed", "run_id", runID, "account_id", candidate.AccountID, "error", err)
	}
	if !skipStateUpdate {
		if err := s.repo.UpdateState(context.Background(), result, statePatch); err != nil {
			infraError = true
			slog.Warn("account_inspection.update_state_failed", "run_id", runID, "account_id", candidate.AccountID, "error", err)
		}
	}
	return infraError
}

func (s *AccountInspectionService) applyAction(ctx context.Context, action string, candidate AccountInspectionCandidate, result AccountInspectionResult) error {
	switch action {
	case AccountInspectionActionDelete:
		return s.adminSvc.DeleteAccount(ctx, candidate.AccountID)
	case AccountInspectionActionDisable:
		_, err := s.adminSvc.SetAccountSchedulable(ctx, candidate.AccountID, false)
		return err
	case AccountInspectionActionRestore:
		_, err := s.adminSvc.SetAccountSchedulable(ctx, candidate.AccountID, true)
		return err
	default:
		return nil
	}
}

func (s *AccountInspectionService) finishRun(ctx context.Context, run *AccountInspectionRun, status, errMsg string) {
	now := time.Now()
	run.Status = status
	run.Error = errMsg
	run.FinishedAt = &now
	run.UpdatedAt = now
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		slog.Warn("account_inspection.finish_run_failed", "run_id", run.ID, "error", err)
	}
}

func (s *AccountInspectionService) reconcileOrphanedActiveRun(ctx context.Context, run *AccountInspectionRun) *AccountInspectionRun {
	if s == nil || s.repo == nil || run == nil || !isAccountInspectionActiveStatus(run.Status) {
		return run
	}

	s.mu.Lock()
	isCurrentRun := s.runningID == run.ID && s.cancel != nil
	s.mu.Unlock()
	if isCurrentRun {
		return run
	}

	now := time.Now()
	run.Status = AccountInspectionRunStopped
	run.Error = "stopped because no active inspection worker was found"
	run.FinishedAt = &now
	run.UpdatedAt = now
	if err := s.repo.UpdateRun(ctx, run); err != nil {
		slog.Warn("account_inspection.reconcile_orphaned_run_failed", "run_id", run.ID, "error", err)
	}
	return run
}

func (s *AccountInspectionService) log(ctx context.Context, runID int64, level, message string) {
	if s == nil || s.repo == nil {
		return
	}
	if strings.TrimSpace(message) == "" {
		return
	}
	_ = s.repo.RecordLog(ctx, &runID, level, message)
}

func NormalizeAccountInspectionSettings(settings AccountInspectionSettings) AccountInspectionSettings {
	settings.Filters.Platform = strings.TrimSpace(settings.Filters.Platform)
	settings.Filters.Type = strings.TrimSpace(settings.Filters.Type)
	settings.Filters.Status = strings.TrimSpace(settings.Filters.Status)
	settings.Filters.Group = strings.TrimSpace(settings.Filters.Group)
	settings.Filters.Search = strings.TrimSpace(settings.Filters.Search)
	settings.Filters.PrivacyMode = strings.TrimSpace(settings.Filters.PrivacyMode)
	settings.ModelID = strings.TrimSpace(settings.ModelID)

	if settings.BatchLimit <= 0 {
		settings.BatchLimit = defaultAccountInspectionBatchLimit
	}
	if settings.BatchLimit > maxAccountInspectionBatchLimit {
		settings.BatchLimit = maxAccountInspectionBatchLimit
	}
	if settings.Concurrency <= 0 {
		settings.Concurrency = defaultAccountInspectionConcurrency
	}
	if settings.Concurrency > maxAccountInspectionConcurrency {
		settings.Concurrency = maxAccountInspectionConcurrency
	}
	if settings.BatchSleepSeconds <= 0 {
		settings.BatchSleepSeconds = defaultAccountInspectionBatchSleepSeconds
	}
	if settings.BatchSleepSeconds > maxAccountInspectionBatchSleepSeconds {
		settings.BatchSleepSeconds = maxAccountInspectionBatchSleepSeconds
	}
	if settings.RecheckAfterHours < 0 {
		settings.RecheckAfterHours = defaultAccountInspectionRecheckAfterHours
	}
	if settings.RecheckAfterHours > maxAccountInspectionRecheckAfterHours {
		settings.RecheckAfterHours = maxAccountInspectionRecheckAfterHours
	}
	if settings.MaxAccountsPerRun < 0 {
		settings.MaxAccountsPerRun = 0
	}
	if settings.MaxAccountsPerRun > maxAccountInspectionMaxAccountsPerRun {
		settings.MaxAccountsPerRun = maxAccountInspectionMaxAccountsPerRun
	}
	if settings.AutoPauseConsecutiveErrors < 0 {
		settings.AutoPauseConsecutiveErrors = 0
	}
	if settings.AutoPauseConsecutiveErrors > maxAccountInspectionAutoPauseErrors {
		settings.AutoPauseConsecutiveErrors = maxAccountInspectionAutoPauseErrors
	}
	if settings.DeleteAuthInvalidMinConsecutive <= 0 {
		settings.DeleteAuthInvalidMinConsecutive = defaultAccountInspectionDeleteMinConsecutive
	}
	if settings.DeleteQuotaExhaustedMinConsecutive <= 0 {
		settings.DeleteQuotaExhaustedMinConsecutive = defaultAccountInspectionDeleteMinConsecutive
	}
	if settings.DeletePaymentRequiredMinConsecutive <= 0 {
		settings.DeletePaymentRequiredMinConsecutive = defaultAccountInspectionDeleteMinConsecutive
	}
	if settings.DeleteOtherFailureMinConsecutive <= 0 {
		settings.DeleteOtherFailureMinConsecutive = defaultAccountInspectionDeleteOtherConsecutive
	}
	if settings.DeleteAuthInvalidAfterHours < 0 {
		settings.DeleteAuthInvalidAfterHours = 0
	}
	if settings.DeleteQuotaExhaustedAfterHours < 0 {
		settings.DeleteQuotaExhaustedAfterHours = defaultAccountInspectionDeleteQuotaHours
	}
	if settings.DeletePaymentRequiredAfterHours < 0 {
		settings.DeletePaymentRequiredAfterHours = defaultAccountInspectionDeletePaymentHours
	}
	if settings.DeleteOtherFailureAfterHours < 0 {
		settings.DeleteOtherFailureAfterHours = defaultAccountInspectionDeleteOtherHours
	}
	if settings.LowResourceMode {
		if settings.BatchLimit > 50 {
			settings.BatchLimit = 50
		}
		if settings.Concurrency > 1 {
			settings.Concurrency = 1
		}
		if settings.BatchSleepSeconds < 30 {
			settings.BatchSleepSeconds = 30
		}
		if settings.MaxAccountsPerRun <= 0 || settings.MaxAccountsPerRun > defaultAccountInspectionMaxAccountsPerRun {
			settings.MaxAccountsPerRun = defaultAccountInspectionMaxAccountsPerRun
		}
		if settings.AutoPauseConsecutiveErrors <= 0 || settings.AutoPauseConsecutiveErrors > defaultAccountInspectionAutoPauseErrors {
			settings.AutoPauseConsecutiveErrors = defaultAccountInspectionAutoPauseErrors
		}
	}
	return settings
}

func sameAccountInspectionScanScope(a, b AccountInspectionSettings) bool {
	a = NormalizeAccountInspectionSettings(a)
	b = NormalizeAccountInspectionSettings(b)
	return a.Filters == b.Filters &&
		a.ModelID == b.ModelID &&
		a.IncludeUnschedulable == b.IncludeUnschedulable &&
		a.RecheckAfterHours == b.RecheckAfterHours
}

func DefaultAccountInspectionSettings() AccountInspectionSettings {
	return NormalizeAccountInspectionSettings(AccountInspectionSettings{
		Enabled:                             false,
		BatchLimit:                          defaultAccountInspectionBatchLimit,
		Concurrency:                         defaultAccountInspectionConcurrency,
		BatchSleepSeconds:                   defaultAccountInspectionBatchSleepSeconds,
		RecheckAfterHours:                   defaultAccountInspectionRecheckAfterHours,
		LowResourceMode:                     true,
		MaxAccountsPerRun:                   defaultAccountInspectionMaxAccountsPerRun,
		AutoPauseConsecutiveErrors:          defaultAccountInspectionAutoPauseErrors,
		IncludeUnschedulable:                true,
		DeleteAuthInvalid:                   true,
		DeleteAuthInvalidMinConsecutive:     defaultAccountInspectionDeleteMinConsecutive,
		DeleteAuthInvalidAfterHours:         0,
		DeleteQuotaExhausted:                false,
		DeleteQuotaExhaustedMinConsecutive:  defaultAccountInspectionDeleteMinConsecutive,
		DeleteQuotaExhaustedAfterHours:      defaultAccountInspectionDeleteQuotaHours,
		DeletePaymentRequired:               false,
		DeletePaymentRequiredMinConsecutive: defaultAccountInspectionDeleteMinConsecutive,
		DeletePaymentRequiredAfterHours:     defaultAccountInspectionDeletePaymentHours,
		DeleteOtherFailure:                  false,
		DeleteOtherFailureMinConsecutive:    defaultAccountInspectionDeleteOtherConsecutive,
		DeleteOtherFailureAfterHours:        defaultAccountInspectionDeleteOtherHours,
		DisableQuotaExhausted:               true,
		RestoreAutoDisabled:                 true,
	})
}

func DecideAccountInspectionAction(settings AccountInspectionSettings, result AccountInspectionResult, candidate AccountInspectionCandidate, now time.Time) (string, AccountInspectionStatePatch) {
	settings = NormalizeAccountInspectionSettings(settings)
	patch := AccountInspectionStatePatch{At: now}
	if result.Status == AccountHealthStatusAvailable {
		patch.UpdateDeleteCandidate = true
		if settings.RestoreAutoDisabled && candidate.AutoDisabled {
			return AccountInspectionActionRestore, patch
		}
		return AccountInspectionActionNone, patch
	}

	if rule, ok := accountInspectionDeleteRule(settings, result); ok {
		count := 1
		firstSeen := now
		if candidate.DeleteCandidateCategory == rule.category {
			if candidate.DeleteCandidateFirstSeenAt != nil {
				firstSeen = *candidate.DeleteCandidateFirstSeenAt
			}
			count = candidate.DeleteCandidateCount + 1
		}
		patch.UpdateDeleteCandidate = true
		patch.DeleteCandidateCategory = rule.category
		patch.DeleteCandidateFirstSeenAt = &firstSeen
		patch.DeleteCandidateCount = count

		if count < rule.minConsecutive {
			return AccountInspectionActionRetain, patch
		}
		if rule.afterHours > 0 && now.Sub(firstSeen) < time.Duration(rule.afterHours)*time.Hour {
			return AccountInspectionActionPendingDelete, patch
		}
		return AccountInspectionActionDelete, patch
	}

	patch.UpdateDeleteCandidate = true
	if settings.DisableQuotaExhausted && result.Category == AccountHealthCategoryQuotaExhausted {
		if candidate.Schedulable && !candidate.AutoDisabled {
			return AccountInspectionActionDisable, patch
		}
		return AccountInspectionActionNone, patch
	}
	return AccountInspectionActionNone, patch
}

type accountInspectionDeletionRule struct {
	category       string
	minConsecutive int
	afterHours     int
}

func accountInspectionDeleteRule(settings AccountInspectionSettings, result AccountInspectionResult) (accountInspectionDeletionRule, bool) {
	if settings.DeleteAuthInvalid && isDeletableAuthInvalid(result) {
		return accountInspectionDeletionRule{
			category:       AccountHealthCategoryAuthInvalid,
			minConsecutive: settings.DeleteAuthInvalidMinConsecutive,
			afterHours:     settings.DeleteAuthInvalidAfterHours,
		}, true
	}
	if settings.DeleteQuotaExhausted && result.Category == AccountHealthCategoryQuotaExhausted {
		return accountInspectionDeletionRule{
			category:       AccountHealthCategoryQuotaExhausted,
			minConsecutive: settings.DeleteQuotaExhaustedMinConsecutive,
			afterHours:     settings.DeleteQuotaExhaustedAfterHours,
		}, true
	}
	if settings.DeletePaymentRequired && result.Category == AccountHealthCategoryPaymentRequired {
		return accountInspectionDeletionRule{
			category:       AccountHealthCategoryPaymentRequired,
			minConsecutive: settings.DeletePaymentRequiredMinConsecutive,
			afterHours:     settings.DeletePaymentRequiredAfterHours,
		}, true
	}
	if settings.DeleteOtherFailure && isDeletableOtherFailure(result) {
		return accountInspectionDeletionRule{
			category:       result.Category,
			minConsecutive: settings.DeleteOtherFailureMinConsecutive,
			afterHours:     settings.DeleteOtherFailureAfterHours,
		}, true
	}
	return accountInspectionDeletionRule{}, false
}

func shouldDisableQuotaAccountDuringInspection(settings AccountInspectionSettings, result AccountInspectionResult, candidate AccountInspectionCandidate, action string) bool {
	return settings.DisableQuotaExhausted &&
		result.Category == AccountHealthCategoryQuotaExhausted &&
		action != AccountInspectionActionDisable &&
		action != AccountInspectionActionDelete &&
		candidate.Schedulable &&
		!candidate.AutoDisabled
}

func isDeletableAuthInvalid(result AccountInspectionResult) bool {
	if result.Category != AccountHealthCategoryAuthInvalid {
		return false
	}
	if result.HTTPStatus == 401 {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(result.ErrorCode)) {
	case "invalid_refresh_token", "refresh_token_invalid", "refresh_token_expired", "refresh_token_reused", "invalid_grant", "invalid_client", "unauthorized_client", "invalid_api_key":
		return true
	}
	lower := strings.ToLower(result.Message)
	return strings.Contains(lower, "invalid refresh token") || strings.Contains(lower, "refresh token expired")
}

func isDeletableOtherFailure(result AccountInspectionResult) bool {
	if result.Status != AccountHealthStatusUnavailable {
		return false
	}
	switch result.Category {
	case AccountHealthCategoryAvailable,
		AccountHealthCategoryRateLimited,
		AccountHealthCategoryQuotaExhausted,
		AccountHealthCategoryPaymentRequired,
		AccountHealthCategoryUnknownError:
		return false
	case AccountHealthCategoryAuthInvalid:
		return !isDeletableAuthInvalid(result)
	default:
		return strings.TrimSpace(result.Category) != ""
	}
}

func isAccountInspectionActiveStatus(status string) bool {
	switch status {
	case AccountInspectionRunQueued, AccountInspectionRunRunning, AccountInspectionRunStopping:
		return true
	default:
		return false
	}
}

func actionRequiresAccountInspectionApply(action string) bool {
	switch action {
	case AccountInspectionActionDelete, AccountInspectionActionDisable, AccountInspectionActionRestore:
		return true
	default:
		return false
	}
}

func isAccountInspectionInfraError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return accountHealthContainsAny(lower,
		"context deadline exceeded",
		"connection refused",
		"server misbehaving",
		"lookup redis",
		"redis:",
		"sql:",
		"database",
		"connection reset",
		"broken pipe",
	)
}

func sleepAccountInspectionWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
