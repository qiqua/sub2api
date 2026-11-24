package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const inspectionActive = `status IN ('queued', 'running', 'stopping')`
const inspectionLease = `interval '30 seconds'`

// Serialize settings and lifecycle changes even when neither a setting nor a run exists.
func (r *accountInspectionRepository) beginInspectionChange(ctx context.Context) (*sql.Tx, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(742319830126)`); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func writeInspectionSettings(ctx context.Context, tx *sql.Tx, settings *service.AccountInspectionSettings) ([]byte, error) {
	*settings = service.NormalizeAccountInspectionSettings(*settings)
	settings.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`,
		accountInspectionSettingsKey, string(data), settings.UpdatedAt)
	return data, err
}

func (r *accountInspectionRepository) EditSettings(ctx context.Context, edit func(*service.AccountInspectionSettings)) (service.AccountInspectionSettings, error) {
	tx, err := r.beginInspectionChange(ctx)
	if err != nil {
		return service.AccountInspectionSettings{}, err
	}
	defer func() { _ = tx.Rollback() }()
	settings, err := readInspectionSettings(ctx, tx)
	if err != nil {
		return settings, err
	}
	edit(&settings)
	if _, err := writeInspectionSettings(ctx, tx, &settings); err != nil {
		return settings, err
	}
	return settings, tx.Commit()
}

func retireExpiredInspections(ctx context.Context, tx *sql.Tx) (bool, error) {
	res, err := tx.ExecContext(ctx, `UPDATE account_inspection_runs SET status = 'stopped',
		error = 'inspection lease expired', finished_at = clock_timestamp(), updated_at = clock_timestamp(), lease_expires_at = NULL
		WHERE `+inspectionActive+` AND (lease_expires_at IS NULL OR lease_expires_at <= clock_timestamp())`)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *accountInspectionRepository) ClaimRun(ctx context.Context, resetCursor, resumeOnly bool) (*service.AccountInspectionRun, error) {
	tx, err := r.beginInspectionChange(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	recovered, err := retireExpiredInspections(ctx, tx)
	if err != nil {
		return nil, err
	}
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT FROM account_inspection_runs WHERE `+inspectionActive+`)`).Scan(&active); err != nil {
		return nil, err
	}
	if active {
		return nil, service.ErrAccountInspectionRunning
	}
	settings, err := readInspectionSettings(ctx, tx)
	if err != nil {
		return nil, err
	}
	if resumeOnly && !settings.Enabled {
		return nil, tx.Commit()
	}
	if resetCursor && (!recovered || !settings.Enabled) {
		latest, err := latestInspectionRun(ctx, tx)
		if err != nil {
			return nil, err
		}
		if latest == nil || latest.Status != service.AccountInspectionRunPaused || !latest.HasMore {
			settings.Cursor = 0
		}
	}
	settings.Enabled = true
	where, args, err := accountInspectionCandidateWhere(settings)
	if err != nil {
		return nil, err
	}
	total, err := countInspectionCandidates(ctx, tx, where, args)
	if err != nil {
		return nil, err
	}
	data, err := writeInspectionSettings(ctx, tx, &settings)
	if err != nil {
		return nil, err
	}
	run, err := scanAccountInspectionRun(tx.QueryRowContext(ctx, `INSERT INTO account_inspection_runs
		(status, settings_snapshot, total_accounts, cursor, next_cursor, lease_expires_at)
		VALUES ('queued', $1::jsonb, $2, $3, $3, clock_timestamp() + `+inspectionLease+`)
		RETURNING `+inspectionRunColumns, string(data), total, settings.Cursor))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return run, nil
}

func (r *accountInspectionRepository) RequestStop(ctx context.Context) (*service.AccountInspectionRun, error) {
	tx, err := r.beginInspectionChange(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	settings, err := readInspectionSettings(ctx, tx)
	if err != nil {
		return nil, err
	}
	settings.Enabled = false
	if _, err := writeInspectionSettings(ctx, tx, &settings); err != nil {
		return nil, err
	}
	// Evaluate expiry after locking even if a result writer or heartbeat held the row.
	_, err = tx.ExecContext(ctx, `WITH locked AS MATERIALIZED (
		SELECT id, lease_expires_at FROM account_inspection_runs WHERE `+inspectionActive+` FOR UPDATE
	), decisions AS MATERIALIZED (SELECT id, lease_expires_at > clock_timestamp() AS live FROM locked)
	UPDATE account_inspection_runs r SET status = CASE WHEN d.live THEN 'stopping' ELSE 'stopped' END,
		finished_at = CASE WHEN d.live THEN NULL ELSE clock_timestamp() END,
		lease_expires_at = CASE WHEN d.live THEN r.lease_expires_at ELSE NULL END, updated_at = clock_timestamp()
	FROM decisions d WHERE r.id = d.id`)
	if err != nil {
		return nil, err
	}
	run, err := latestInspectionRun(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return run, nil
}

const inspectionLockedRun = `WITH locked AS MATERIALIZED (
	SELECT id, status, lease_expires_at FROM account_inspection_runs WHERE id = $1 FOR UPDATE
)`

func (r *accountInspectionRepository) RenewRun(ctx context.Context, id int64) error {
	var status string
	err := r.db.QueryRowContext(ctx, inspectionLockedRun+`, live AS MATERIALIZED (
		SELECT * FROM locked WHERE `+inspectionActive+` AND lease_expires_at > clock_timestamp()
	), renewed AS (
		UPDATE account_inspection_runs r SET lease_expires_at = clock_timestamp() + `+inspectionLease+`
		FROM live WHERE r.id = live.id AND live.status <> 'stopping' RETURNING r.status
	) SELECT status FROM renewed UNION ALL SELECT status FROM live WHERE status = 'stopping'`, id).Scan(&status)
	return inspectionOwnerError(status, err, false)
}

func inspectionOwnerError(status string, err error, allowStopping bool) error {
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrAccountInspectionLeaseLost
	}
	if err == nil && status == service.AccountInspectionRunStopping && !allowStopping {
		return service.ErrAccountInspectionStopping
	}
	return err
}

func (r *accountInspectionRepository) withWritableInspection(ctx context.Context, id int64, write func(*sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Concurrent candidate writes may share the lock. A takeover, stop or finish cannot.
	var status string
	err = tx.QueryRowContext(ctx, `WITH locked AS MATERIALIZED (
		SELECT status, lease_expires_at FROM account_inspection_runs WHERE id = $1 FOR SHARE
	) SELECT status FROM locked WHERE `+inspectionActive+` AND lease_expires_at > clock_timestamp()`, id).Scan(&status)
	// The stopping owner may drain in-flight results until it acknowledges the stop.
	if err := inspectionOwnerError(status, err, true); err != nil {
		return err
	}
	if err := write(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *accountInspectionRepository) SaveRun(ctx context.Context, run *service.AccountInspectionRun, cursor *int64) error {
	if run == nil {
		return service.ErrAccountInspectionLeaseLost
	}
	terminal := true
	disable := false
	switch run.Status {
	case service.AccountInspectionRunRunning:
		terminal = false
	case service.AccountInspectionRunCompleted, service.AccountInspectionRunPaused, service.AccountInspectionRunFailed:
		disable = true
	case service.AccountInspectionRunStopped:
	default:
		return fmt.Errorf("invalid inspection checkpoint status %q", run.Status)
	}
	tx, err := r.beginInspectionChange(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var status string
	err = tx.QueryRowContext(ctx, inspectionLockedRun+` SELECT status FROM locked
		WHERE `+inspectionActive+` AND lease_expires_at > clock_timestamp()`, run.ID).Scan(&status)
	if err := inspectionOwnerError(status, err, run.Status == service.AccountInspectionRunStopped); err != nil {
		return err
	}
	if cursor != nil || disable {
		settings, err := readInspectionSettings(ctx, tx)
		if err != nil {
			return err
		}
		if cursor != nil && service.SameAccountInspectionScanScope(run.SettingsSnapshot, settings) {
			settings.Cursor = *cursor
		}
		if disable {
			settings.Enabled = false
		}
		if _, err := writeInspectionSettings(ctx, tx, &settings); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE account_inspection_runs SET status = $2,
		total_accounts = $3, cursor = $4, next_cursor = $5, has_more = $6, error = $7,
		started_at = COALESCE(started_at, $8), finished_at = CASE WHEN $9 THEN COALESCE($10, clock_timestamp()) END,
		updated_at = clock_timestamp(), lease_expires_at = CASE WHEN NOT $9 THEN clock_timestamp() + `+inspectionLease+` END
		WHERE id = $1`, run.ID, run.Status, run.TotalAccounts, run.Cursor, run.NextCursor, run.HasMore,
		inspectionNullString(run.Error), run.StartedAt, terminal, run.FinishedAt)
	if err != nil {
		return err
	}
	return tx.Commit()
}
