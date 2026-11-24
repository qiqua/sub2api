package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// The worker, its caller and a second service each see independent repository rows.
type scriptedAccountInspectionRepository struct {
	noopAccountInspectionRepository
	mu        sync.Mutex
	settings  AccountInspectionSettings
	latestRun *AccountInspectionRun
	expired   bool
	batches   []AccountInspectionCandidateBatch

	beforeClaim func()
	beforeSave  func(*AccountInspectionRun)
	afterSave   func(*AccountInspectionRun, error)
}

func inspectionRunCopy(run *AccountInspectionRun) *AccountInspectionRun {
	if run == nil {
		return nil
	}
	cp := *run
	if run.StartedAt != nil {
		started := *run.StartedAt
		cp.StartedAt = &started
	}
	if run.FinishedAt != nil {
		finished := *run.FinishedAt
		cp.FinishedAt = &finished
	}
	return &cp
}

func (r *scriptedAccountInspectionRepository) GetSettings(context.Context) (AccountInspectionSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.settings, nil
}

func (r *scriptedAccountInspectionRepository) EditSettings(_ context.Context, edit func(*AccountInspectionSettings)) (AccountInspectionSettings, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	edit(&r.settings)
	return r.settings, nil
}

func (r *scriptedAccountInspectionRepository) GetLatestRun(context.Context) (*AccountInspectionRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return inspectionRunCopy(r.latestRun), nil
}

func (r *scriptedAccountInspectionRepository) ClaimRun(_ context.Context, resetCursor, resumeOnly bool) (*AccountInspectionRun, error) {
	if r.beforeClaim != nil {
		r.beforeClaim()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	id := int64(1)
	continueScan := false
	if last := r.latestRun; last != nil {
		id = last.ID + 1
		if isAccountInspectionActiveStatus(last.Status) {
			if !r.expired {
				return nil, ErrAccountInspectionRunning
			}
			last.Status = AccountInspectionRunStopped
			continueScan = r.settings.Enabled
		} else {
			continueScan = last.Status == AccountInspectionRunPaused && last.HasMore
		}
	}
	if resumeOnly && !r.settings.Enabled {
		return nil, nil
	}
	if resetCursor && !continueScan {
		r.settings.Cursor = 0
	}
	r.settings.Enabled = true
	r.expired = false
	r.latestRun = &AccountInspectionRun{
		ID: id, Status: AccountInspectionRunQueued, SettingsSnapshot: r.settings,
		Cursor: r.settings.Cursor, NextCursor: r.settings.Cursor,
	}
	if len(r.batches) > 0 {
		r.latestRun.TotalAccounts = r.batches[0].Total
	}
	return inspectionRunCopy(r.latestRun), nil
}

func (r *scriptedAccountInspectionRepository) RequestStop(context.Context) (*AccountInspectionRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.settings.Enabled = false
	if r.latestRun != nil && isAccountInspectionActiveStatus(r.latestRun.Status) {
		r.latestRun.Status = AccountInspectionRunStopping
	}
	return inspectionRunCopy(r.latestRun), nil
}

func (r *scriptedAccountInspectionRepository) RenewRun(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ownerError(id)
}

// Called with mu held.
func (r *scriptedAccountInspectionRepository) ownerError(id int64) error {
	if r.latestRun == nil || r.latestRun.ID != id || r.expired || !isAccountInspectionActiveStatus(r.latestRun.Status) {
		return ErrAccountInspectionLeaseLost
	}
	if r.latestRun.Status == AccountInspectionRunStopping {
		return ErrAccountInspectionStopping
	}
	return nil
}

func (r *scriptedAccountInspectionRepository) SaveRun(_ context.Context, run *AccountInspectionRun, cursor *int64) (err error) {
	if r.beforeSave != nil {
		r.beforeSave(run)
	}
	if r.afterSave != nil {
		defer func() { r.afterSave(run, err) }()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	err = r.ownerError(run.ID)
	if err != nil && !(err == ErrAccountInspectionStopping && run.Status == AccountInspectionRunStopped) {
		return err
	}
	r.latestRun = inspectionRunCopy(run)
	if cursor != nil && SameAccountInspectionScanScope(r.settings, run.SettingsSnapshot) {
		r.settings.Cursor = *cursor
	}
	switch run.Status {
	case AccountInspectionRunCompleted, AccountInspectionRunPaused, AccountInspectionRunFailed:
		r.settings.Enabled = false
	}
	return nil
}

func (r *scriptedAccountInspectionRepository) ListCandidates(context.Context, AccountInspectionSettings) (AccountInspectionCandidateBatch, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.batches) == 0 {
		return AccountInspectionCandidateBatch{}, nil
	}
	batch := r.batches[0]
	r.batches = r.batches[1:]
	return batch, nil
}

func awaitInspection[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for inspection")
		var zero T
		return zero
	}
}

type gatedInspectionTester struct {
	entered chan context.Context
	release chan struct{}
}

func (g *gatedInspectionTester) RunTestBackground(ctx context.Context, _ int64, _ string) (*ScheduledTestResult, error) {
	g.entered <- ctx
	<-g.release
	return &ScheduledTestResult{Status: "success"}, ctx.Err()
}

func TestAccountInspectionSharedClaimAndRemoteStop(t *testing.T) {
	ctx := context.Background()
	arrived := make(chan struct{}, 2)
	claim := make(chan struct{})
	allowClaim := sync.OnceFunc(func() { close(claim) })
	defer allowClaim()
	stopped := make(chan struct{}, 1)
	repo := &scriptedAccountInspectionRepository{
		settings:    DefaultAccountInspectionSettings(),
		batches:     []AccountInspectionCandidateBatch{{Items: []AccountInspectionCandidate{{AccountID: 7}}, Total: 1}},
		beforeClaim: func() { arrived <- struct{}{}; <-claim },
		afterSave: func(run *AccountInspectionRun, err error) {
			if run.Status == AccountInspectionRunStopped && err == nil {
				stopped <- struct{}{}
			}
		},
	}
	tester := &gatedInspectionTester{make(chan context.Context, 2), make(chan struct{})}
	release := sync.OnceFunc(func() { close(tester.release) })
	defer release()
	type result struct {
		svc *AccountInspectionService
		run *AccountInspectionRun
		err error
	}
	results := make(chan result, 2)
	for range 2 {
		svc := NewAccountInspectionService(repo, noopAdminService{}, tester, nil)
		svc.heartbeatInterval = 5 * time.Millisecond
		t.Cleanup(svc.Stop)
		go func() {
			run, err := svc.StartRun(ctx, true)
			results <- result{svc, run, err}
		}()
	}
	awaitInspection(t, arrived)
	awaitInspection(t, arrived)
	allowClaim()
	owner, peer := awaitInspection(t, results), awaitInspection(t, results)
	if owner.err != nil {
		owner, peer = peer, owner
	}
	if owner.err != nil || owner.run == nil || !errors.Is(peer.err, ErrAccountInspectionRunning) || peer.run != nil {
		t.Fatalf("concurrent starts: owner=%+v peer=%+v", owner, peer)
	}
	probe := awaitInspection(t, tester.entered)
	status, err := peer.svc.GetStatus(ctx)
	if err != nil || !status.Running || status.Run == nil || status.Run.ID != owner.run.ID || status.Run.Status != AccountInspectionRunRunning {
		t.Fatalf("remote status = %+v, err = %v", status, err)
	}
	stopping, err := peer.svc.StopRun(ctx)
	if err != nil || stopping == nil || stopping.ID != owner.run.ID || stopping.Status != AccountInspectionRunStopping {
		t.Fatalf("remote stop = %+v, err = %v", stopping, err)
	}
	awaitInspection(t, probe.Done())
	release()
	awaitInspection(t, stopped)
	status, err = peer.svc.GetStatus(ctx)
	if err != nil || status.Running || status.Settings.Enabled || status.Run == nil || status.Run.Status != AccountInspectionRunStopped || status.Run.FinishedAt == nil {
		t.Fatalf("after owner cancellation: %+v, err = %v", status, err)
	}
	if owner.run.Status != AccountInspectionRunQueued || owner.run.StartedAt != nil || owner.run.FinishedAt != nil || stopping.Status != AccountInspectionRunStopping {
		t.Fatalf("returned snapshots changed: start=%+v stop=%+v", owner.run, stopping)
	}
}

func TestAccountInspectionGetStatusLeavesExpiredStoppingRunAlone(t *testing.T) {
	ctx := context.Background()
	before := AccountInspectionRun{ID: 19, Status: AccountInspectionRunStopping, NextCursor: 76}
	repo := &scriptedAccountInspectionRepository{
		latestRun: &before, expired: true, settings: DefaultAccountInspectionSettings(),
		beforeSave:  func(*AccountInspectionRun) { t.Fatal("GET attempted to write a run") },
		beforeClaim: func() { t.Fatal("GET attempted to claim a run") },
	}
	svc := NewAccountInspectionService(repo, noopAdminService{}, &successInspectionTester{}, nil)
	status, err := svc.GetStatus(ctx)
	if err != nil || !status.Running || status.Run == nil || status.Run.ID != 19 || status.Run.Status != AccountInspectionRunStopping || status.Run.FinishedAt != nil {
		t.Fatalf("GET expired stopping run = %+v, err = %v", status, err)
	}
}

func TestAccountInspectionRecoveryFencesLateCompletion(t *testing.T) {
	ctx := context.Background()
	finishing := make(chan struct{})
	resumeFinish := make(chan struct{})
	release := sync.OnceFunc(func() { close(resumeFinish) })
	defer release()
	late := make(chan error, 1)
	settings := DefaultAccountInspectionSettings()
	settings.Cursor = 73
	repo := &scriptedAccountInspectionRepository{
		settings: settings,
		batches: []AccountInspectionCandidateBatch{
			{}, {Items: []AccountInspectionCandidate{{AccountID: 91}}, Total: 1},
		},
		beforeSave: func(run *AccountInspectionRun) {
			if run.ID == 1 && run.Status == AccountInspectionRunCompleted {
				close(finishing)
				<-resumeFinish
			}
		},
		afterSave: func(run *AccountInspectionRun, err error) {
			if run.ID == 1 && run.Status == AccountInspectionRunCompleted {
				late <- err
			}
		},
	}
	old := NewAccountInspectionService(repo, noopAdminService{}, &successInspectionTester{}, nil)
	t.Cleanup(old.Stop)
	first, err := old.StartRun(ctx)
	if err != nil || first == nil {
		t.Fatalf("initial start = %+v, err = %v", first, err)
	}
	awaitInspection(t, finishing)
	repo.mu.Lock()
	repo.expired = true
	repo.mu.Unlock()
	tester := &gatedInspectionTester{make(chan context.Context, 2), make(chan struct{})}
	successor := NewAccountInspectionService(repo, noopAdminService{}, tester, nil)
	t.Cleanup(successor.Stop)
	defer close(tester.release)
	next, err := successor.startRun(ctx, false, true)
	if err != nil || next == nil || next.ID == first.ID || next.SettingsSnapshot.Cursor != 73 {
		t.Fatalf("recovery = %+v, err = %v", next, err)
	}
	awaitInspection(t, tester.entered)
	release()
	if err := awaitInspection(t, late); !errors.Is(err, ErrAccountInspectionLeaseLost) {
		t.Fatalf("old completion returned %v, want lease lost", err)
	}
	status, err := successor.GetStatus(ctx)
	if err != nil || status.Run == nil || status.Run.ID != next.ID || status.Run.Status != AccountInspectionRunRunning || !status.Settings.Enabled || status.Settings.Cursor != 73 {
		t.Fatalf("successor after late completion = %+v, err = %v", status, err)
	}
}

func TestAccountInspectionStartCursor(t *testing.T) {
	for _, tc := range []struct {
		name       string
		enabled    bool
		reset      bool
		resumeOnly bool
		previous   string
		hasMore    bool
		wantRun    bool
		wantCursor int64
	}{
		{name: "fresh manual start resets", reset: true, wantRun: true},
		{name: "automatic recovery resumes enabled scan", enabled: true, resumeOnly: true, wantRun: true, wantCursor: 9876},
		{name: "automatic recovery leaves manual pause alone", previous: AccountInspectionRunPaused, hasMore: true, resumeOnly: true},
		{name: "manual start continues paused scan", previous: AccountInspectionRunPaused, hasMore: true, reset: true, wantRun: true, wantCursor: 9876},
		{name: "manual start resets exhausted pause", previous: AccountInspectionRunPaused, reset: true, wantRun: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			repo := &scriptedAccountInspectionRepository{settings: DefaultAccountInspectionSettings()}
			repo.settings.Enabled = tc.enabled
			repo.settings.Cursor = 9876
			if tc.previous != "" {
				repo.latestRun = &AccountInspectionRun{ID: 12, Status: tc.previous, HasMore: tc.hasMore, NextCursor: 9876}
			}
			svc := NewAccountInspectionService(repo, noopAdminService{}, &successInspectionTester{}, nil)
			t.Cleanup(svc.Stop)
			var run *AccountInspectionRun
			var err error
			if tc.resumeOnly {
				run, err = svc.startRun(ctx, false, true)
			} else {
				run, err = svc.StartRun(ctx, tc.reset)
			}
			if err != nil || (run != nil) != tc.wantRun {
				t.Fatalf("start = %+v, err = %v", run, err)
			}
			if run != nil && run.SettingsSnapshot.Cursor != tc.wantCursor {
				t.Fatalf("start cursor = %d, want %d", run.SettingsSnapshot.Cursor, tc.wantCursor)
			}
			if run == nil {
				status, err := svc.GetStatus(ctx)
				if err != nil || status.Settings.Enabled || status.Settings.Cursor != 9876 || status.Run == nil || status.Run.ID != 12 || status.Run.Status != AccountInspectionRunPaused {
					t.Fatalf("manual pause after recovery = %+v, err = %v", status, err)
				}
			}
		})
	}
}

func TestAccountInspectionPausedLimitCheckpointsCursorUntilManualRestart(t *testing.T) {
	ctx := context.Background()
	settings := DefaultAccountInspectionSettings()
	settings.MaxAccountsPerRun = 1
	settings.LowResourceMode = false
	settings.BatchSleepSeconds = 1
	terminal := make(chan string, 2)
	repo := &scriptedAccountInspectionRepository{
		settings: settings,
		batches: []AccountInspectionCandidateBatch{
			{Items: []AccountInspectionCandidate{{AccountID: 41}}, NextCursor: 41, HasMore: true, Total: 2},
			{Items: []AccountInspectionCandidate{{AccountID: 42}}, Cursor: 41, NextCursor: 42, Total: 1},
		},
		afterSave: func(run *AccountInspectionRun, err error) {
			if err == nil && !isAccountInspectionActiveStatus(run.Status) {
				terminal <- run.Status
			}
		},
	}
	owner := NewAccountInspectionService(repo, noopAdminService{}, &successInspectionTester{}, nil)
	t.Cleanup(owner.Stop)
	if _, err := owner.StartRun(ctx, true); err != nil {
		t.Fatal(err)
	}
	if got := awaitInspection(t, terminal); got != AccountInspectionRunPaused {
		t.Fatalf("run at limit = %s, want paused", got)
	}
	peer := NewAccountInspectionService(repo, noopAdminService{}, &successInspectionTester{}, nil)
	t.Cleanup(peer.Stop)
	paused, err := peer.GetStatus(ctx)
	if err != nil || paused.Settings.Enabled || paused.Settings.Cursor != 41 || paused.Run == nil || !paused.Run.HasMore {
		t.Fatalf("checkpoint at pause = %+v, err = %v", paused, err)
	}
	if run, err := peer.startRun(ctx, false, true); run != nil || err != nil {
		t.Fatalf("automatic recovery at pause = %+v, err = %v", run, err)
	}
	run, err := peer.StartRun(ctx, true)
	if err != nil || run == nil || run.SettingsSnapshot.Cursor != 41 {
		t.Fatalf("manual restart = %+v, err = %v", run, err)
	}
	if got := awaitInspection(t, terminal); got != AccountInspectionRunCompleted {
		t.Fatalf("resumed run = %s, want completed", got)
	}
	finished, err := peer.GetStatus(ctx)
	if err != nil || finished.Settings.Enabled || finished.Settings.Cursor != 0 {
		t.Fatalf("checkpoint after completion = %+v, err = %v", finished, err)
	}
}

func TestAccountInspectionCompletionPreservesConcurrentSettingsEdit(t *testing.T) {
	ctx := context.Background()
	finished := make(chan struct{}, 1)
	settings := DefaultAccountInspectionSettings()
	settings.ModelID = "original-model"
	repo := &scriptedAccountInspectionRepository{
		settings: settings,
		batches:  []AccountInspectionCandidateBatch{{Items: []AccountInspectionCandidate{{AccountID: 5}}, NextCursor: 5, Total: 1}},
		afterSave: func(run *AccountInspectionRun, err error) {
			if err == nil && run.Status == AccountInspectionRunCompleted {
				finished <- struct{}{}
			}
		},
	}
	tester := &gatedInspectionTester{make(chan context.Context, 1), make(chan struct{})}
	release := sync.OnceFunc(func() { close(tester.release) })
	defer release()
	owner := NewAccountInspectionService(repo, noopAdminService{}, tester, nil)
	t.Cleanup(owner.Stop)
	peer := NewAccountInspectionService(repo, noopAdminService{}, &successInspectionTester{}, nil)
	t.Cleanup(peer.Stop)
	started, err := owner.StartRun(ctx, true)
	if err != nil || started == nil {
		t.Fatalf("start = %+v, err = %v", started, err)
	}
	awaitInspection(t, tester.entered)
	settings.ModelID = "next-model"
	settings.Filters.Group = "9"
	edited, err := peer.UpdateSettings(ctx, settings)
	if err != nil || !edited.Enabled || edited.ModelID != "next-model" {
		t.Fatalf("edit during run = %+v, err = %v", edited, err)
	}
	release()
	awaitInspection(t, finished)
	status, err := peer.GetStatus(ctx)
	if err != nil || status.Run == nil || status.Settings.ModelID != "next-model" || status.Settings.Filters.Group != "9" || status.Settings.Enabled {
		t.Fatalf("settings after completion = %+v, err = %v", status, err)
	}
	if started.SettingsSnapshot.ModelID != "original-model" || status.Run.SettingsSnapshot != started.SettingsSnapshot {
		t.Fatalf("run snapshot after settings edit = %+v, initial = %+v", status.Run.SettingsSnapshot, started.SettingsSnapshot)
	}
}

func TestAccountInspectionExpiredProbeCannotDisableAccount(t *testing.T) {
	ctx := context.Background()
	repo := &scriptedAccountInspectionRepository{settings: DefaultAccountInspectionSettings()}
	old, err := repo.ClaimRun(ctx, false, false)
	if err != nil {
		t.Fatal(err)
	}
	repo.expired = true
	if _, err := repo.ClaimRun(ctx, false, true); err != nil {
		t.Fatal(err)
	}
	// The old worker resumes before its heartbeat has observed the takeover.
	admin := &recordingInspectionAdminService{}
	svc := NewAccountInspectionService(repo, admin, &quotaExhaustedInspectionTester{}, nil)
	t.Cleanup(svc.Stop)
	svc.processCandidate(ctx, old.ID, old.SettingsSnapshot, AccountInspectionCandidate{AccountID: 42, Schedulable: true})
	if got := admin.disabledCount.Load(); got != 0 {
		t.Fatalf("expired worker disabled %d accounts", got)
	}
}
