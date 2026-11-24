//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Claims commit on separate connections. Give each test its own inspection tables
// while retaining the migrated column types, constraints and indexes.
func inspectionTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	schema := fmt.Sprintf("inspection_%d", time.Now().UnixNano())
	_, err := integrationDB.Exec(`CREATE SCHEMA ` + schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.Exec(`DROP SCHEMA ` + schema + ` CASCADE`)
		require.NoError(t, err)
	})
	for _, table := range []string{"settings", "account_inspection_runs", "account_inspection_results", "account_inspection_states"} {
		_, err := integrationDB.Exec(`CREATE TABLE ` + schema + `.` + table + ` (LIKE public.` + table + ` INCLUDING ALL)`)
		require.NoError(t, err)
	}
	u, err := url.Parse(integrationDSN)
	require.NoError(t, err)
	q := u.Query()
	q.Set("search_path", schema+",public")
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db, u.String()
}

func TestAccountInspectionMissingSettingsIsReadOnly(t *testing.T) {
	_, dsn := inspectionTestDB(t)
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	q := u.Query()
	q.Set("default_transaction_read_only", "on")
	u.RawQuery = q.Encode()
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	repo := NewAccountInspectionRepository(db, nil)
	settings, err := repo.GetSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, service.DefaultAccountInspectionSettings(), settings)
}

func TestAccountInspectionLeaseTakeoverFencesPreviousOwner(t *testing.T) {
	db, _ := inspectionTestDB(t)
	repo := NewAccountInspectionRepository(db, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	start := make(chan struct{})
	type claim struct {
		run *service.AccountInspectionRun
		err error
	}
	claims := make(chan claim, 2)
	for range 2 {
		go func() {
			<-start
			run, err := repo.ClaimRun(ctx, false, false)
			claims <- claim{run, err}
		}()
	}
	close(start)
	a, b := <-claims, <-claims
	if errors.Is(a.err, service.ErrAccountInspectionRunning) {
		a, b = b, a
	}
	require.NoError(t, a.err)
	require.ErrorIs(t, b.err, service.ErrAccountInspectionRunning)
	old := a.run
	old.Status = service.AccountInspectionRunRunning
	cursor := int64(42)
	old.NextCursor = cursor
	require.NoError(t, repo.SaveRun(ctx, old, &cursor))
	require.NoError(t, repo.RenewRun(ctx, old.ID))
	result := service.AccountInspectionResult{RunID: old.ID, AccountID: 1, CheckedAt: time.Now()}
	require.NoError(t, repo.SaveResult(ctx, result))
	require.NoError(t, repo.UpdateState(ctx, result, service.AccountInspectionStatePatch{}))
	_, err := db.ExecContext(ctx, `UPDATE account_inspection_runs SET lease_expires_at = clock_timestamp() WHERE id=$1`, old.ID)
	require.NoError(t, err)
	require.ErrorIs(t, repo.RenewRun(ctx, old.ID), service.ErrAccountInspectionLeaseLost)
	require.ErrorIs(t, repo.SaveResult(ctx, result), service.ErrAccountInspectionLeaseLost)
	newRun, err := repo.ClaimRun(ctx, true, true)
	require.NoError(t, err)
	require.NotNil(t, newRun)
	require.NotEqual(t, old.ID, newRun.ID)
	require.Equal(t, cursor, newRun.Cursor)
	require.Equal(t, cursor, newRun.SettingsSnapshot.Cursor)
	old.Status = service.AccountInspectionRunCompleted
	zero := int64(0)
	require.ErrorIs(t, repo.SaveRun(ctx, old, &zero), service.ErrAccountInspectionLeaseLost)
	disabled := true
	require.ErrorIs(t, repo.UpdateState(ctx, result, service.AccountInspectionStatePatch{AutoDisabled: &disabled}), service.ErrAccountInspectionLeaseLost)
	actual, err := repo.GetAccountAutoDisabled(ctx, result.AccountID)
	require.NoError(t, err)
	require.False(t, actual)
	results, err := repo.ListRecentResults(ctx, old.ID, 10)
	require.NoError(t, err)
	require.Len(t, results, 1)
	settings, err := repo.GetSettings(ctx)
	require.NoError(t, err)
	require.True(t, settings.Enabled)
	require.Equal(t, cursor, settings.Cursor)

	stopping, err := repo.RequestStop(ctx)
	require.NoError(t, err)
	require.Equal(t, service.AccountInspectionRunStopping, stopping.Status)
	require.ErrorIs(t, repo.RenewRun(ctx, newRun.ID), service.ErrAccountInspectionStopping)
	_, err = repo.ClaimRun(ctx, false, false)
	require.ErrorIs(t, err, service.ErrAccountInspectionRunning)
	newRun.Status = service.AccountInspectionRunRunning
	require.ErrorIs(t, repo.SaveRun(ctx, newRun, nil), service.ErrAccountInspectionStopping)
	result.RunID = newRun.ID
	require.NoError(t, repo.SaveResult(ctx, result))
	newRun.Status = service.AccountInspectionRunStopped
	require.NoError(t, repo.SaveRun(ctx, newRun, nil))
	require.ErrorIs(t, repo.SaveResult(ctx, result), service.ErrAccountInspectionLeaseLost)
	resumed, err := repo.ClaimRun(ctx, false, true)
	require.NoError(t, err)
	require.Nil(t, resumed)
}

func TestAccountInspectionCheckpointPreservesEditedScope(t *testing.T) {
	db, _ := inspectionTestDB(t)
	repo := NewAccountInspectionRepository(db, nil)
	ctx := context.Background()
	run, err := repo.ClaimRun(ctx, false, false)
	require.NoError(t, err)
	_, err = repo.EditSettings(ctx, func(settings *service.AccountInspectionSettings) {
		settings.Filters.Search = "another scope"
		settings.Cursor = 7
	})
	require.NoError(t, err)
	cursor := int64(81)
	run.Status = service.AccountInspectionRunPaused
	run.NextCursor = cursor
	run.HasMore = true
	require.NoError(t, repo.SaveRun(ctx, run, &cursor))
	latest, err := repo.GetLatestRun(ctx)
	require.NoError(t, err)
	require.Equal(t, run.SettingsSnapshot, latest.SettingsSnapshot)
	require.Equal(t, cursor, latest.NextCursor)
	settings, err := repo.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "another scope", settings.Filters.Search)
	require.Equal(t, int64(7), settings.Cursor)
	require.False(t, settings.Enabled)
}
