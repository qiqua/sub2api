package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

const accountInspectionSettingsKey = "account_inspection_settings"

type accountInspectionRepository struct {
	db          *sql.DB
	settingRepo service.SettingRepository
}

func NewAccountInspectionRepository(db *sql.DB, settingRepo service.SettingRepository) service.AccountInspectionRepository {
	return &accountInspectionRepository{db: db, settingRepo: settingRepo}
}

func (r *accountInspectionRepository) GetSettings(ctx context.Context) (service.AccountInspectionSettings, error) {
	if r == nil || r.settingRepo == nil {
		return service.DefaultAccountInspectionSettings(), nil
	}
	raw, err := r.settingRepo.GetValue(ctx, accountInspectionSettingsKey)
	if err != nil {
		if errors.Is(err, service.ErrSettingNotFound) {
			defaults := service.DefaultAccountInspectionSettings()
			if _, saveErr := r.SaveSettings(ctx, defaults); saveErr != nil {
				return defaults, saveErr
			}
			return defaults, nil
		}
		return service.AccountInspectionSettings{}, err
	}
	var settings service.AccountInspectionSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		settings = service.DefaultAccountInspectionSettings()
	}
	return service.NormalizeAccountInspectionSettings(settings), nil
}

func (r *accountInspectionRepository) SaveSettings(ctx context.Context, settings service.AccountInspectionSettings) (service.AccountInspectionSettings, error) {
	settings = service.NormalizeAccountInspectionSettings(settings)
	settings.UpdatedAt = time.Now()
	data, err := json.Marshal(settings)
	if err != nil {
		return settings, err
	}
	if err := r.settingRepo.Set(ctx, accountInspectionSettingsKey, string(data)); err != nil {
		return settings, err
	}
	return settings, nil
}

func (r *accountInspectionRepository) CreateRun(ctx context.Context, settings service.AccountInspectionSettings, total int64) (*service.AccountInspectionRun, error) {
	settings = service.NormalizeAccountInspectionSettings(settings)
	data, err := json.Marshal(settings)
	if err != nil {
		return nil, err
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO account_inspection_runs (
			status, settings_snapshot, total_accounts, cursor, next_cursor, has_more, created_at, updated_at
		)
		VALUES ($1, $2::jsonb, $3, $4, $4, FALSE, NOW(), NOW())
		RETURNING id, status, settings_snapshot, total_accounts, cursor, next_cursor, has_more, error,
			started_at, finished_at, created_at, updated_at
	`, service.AccountInspectionRunQueued, string(data), total, settings.Cursor)
	return scanAccountInspectionRun(row)
}

func (r *accountInspectionRepository) GetLatestRun(ctx context.Context) (*service.AccountInspectionRun, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, status, settings_snapshot, total_accounts, cursor, next_cursor, has_more, error,
			started_at, finished_at, created_at, updated_at
		FROM account_inspection_runs
		ORDER BY created_at DESC
		LIMIT 1
	`)
	run, err := scanAccountInspectionRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

func (r *accountInspectionRepository) GetRun(ctx context.Context, id int64) (*service.AccountInspectionRun, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, status, settings_snapshot, total_accounts, cursor, next_cursor, has_more, error,
			started_at, finished_at, created_at, updated_at
		FROM account_inspection_runs
		WHERE id = $1
	`, id)
	run, err := scanAccountInspectionRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

func (r *accountInspectionRepository) UpdateRun(ctx context.Context, run *service.AccountInspectionRun) error {
	if run == nil {
		return nil
	}
	data, err := json.Marshal(run.SettingsSnapshot)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE account_inspection_runs
		SET status = $2,
			settings_snapshot = $3::jsonb,
			total_accounts = $4,
			cursor = $5,
			next_cursor = $6,
			has_more = $7,
			error = $8,
			started_at = $9,
			finished_at = $10,
			updated_at = NOW()
		WHERE id = $1
	`, run.ID, run.Status, string(data), run.TotalAccounts, run.Cursor, run.NextCursor, run.HasMore, inspectionNullString(run.Error), run.StartedAt, run.FinishedAt)
	return err
}

func (r *accountInspectionRepository) ListCandidates(ctx context.Context, settings service.AccountInspectionSettings) (service.AccountInspectionCandidateBatch, error) {
	settings = service.NormalizeAccountInspectionSettings(settings)
	where, args, err := accountInspectionCandidateWhere(settings)
	if err != nil {
		return service.AccountInspectionCandidateBatch{}, err
	}
	total, err := r.countCandidatesWithWhere(ctx, where, args)
	if err != nil {
		return service.AccountInspectionCandidateBatch{}, err
	}

	limit := settings.BatchLimit + 1
	args = append(args, limit)
	query := fmt.Sprintf(`
		SELECT a.id, a.name, a.platform, a.type, a.schedulable, COALESCE(s.auto_disabled, FALSE),
			COALESCE(s.delete_candidate_category, ''), s.delete_candidate_first_seen_at,
			COALESCE(s.delete_candidate_count, 0)
		FROM accounts a
		LEFT JOIN account_inspection_states s ON s.account_id = a.id
		%s
		ORDER BY a.id ASC
		LIMIT $%d
	`, where, len(args))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return service.AccountInspectionCandidateBatch{}, err
	}
	defer func() { _ = rows.Close() }()

	items := make([]service.AccountInspectionCandidate, 0, settings.BatchLimit)
	hasMore := false
	nextCursor := settings.Cursor
	for rows.Next() {
		var item service.AccountInspectionCandidate
		var firstSeen sql.NullTime
		if err := rows.Scan(
			&item.AccountID,
			&item.Name,
			&item.Platform,
			&item.Type,
			&item.Schedulable,
			&item.AutoDisabled,
			&item.DeleteCandidateCategory,
			&firstSeen,
			&item.DeleteCandidateCount,
		); err != nil {
			return service.AccountInspectionCandidateBatch{}, err
		}
		if firstSeen.Valid {
			item.DeleteCandidateFirstSeenAt = &firstSeen.Time
		}
		if len(items) >= settings.BatchLimit {
			hasMore = true
			break
		}
		items = append(items, item)
		nextCursor = item.AccountID
	}
	if err := rows.Err(); err != nil {
		return service.AccountInspectionCandidateBatch{}, err
	}

	return service.AccountInspectionCandidateBatch{
		Items:      items,
		Cursor:     settings.Cursor,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Total:      total,
	}, nil
}

func (r *accountInspectionRepository) CountCandidates(ctx context.Context, settings service.AccountInspectionSettings) (int64, error) {
	where, args, err := accountInspectionCandidateWhere(service.NormalizeAccountInspectionSettings(settings))
	if err != nil {
		return 0, err
	}
	return r.countCandidatesWithWhere(ctx, where, args)
}

func (r *accountInspectionRepository) SaveResult(ctx context.Context, result service.AccountInspectionResult) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO account_inspection_results (
			run_id, account_id, name, platform, type, status, category, http_status, error_code, message,
			latency_ms, action, action_error, checked_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
	`, result.RunID, result.AccountID, result.Name, result.Platform, result.Type, result.Status, result.Category,
		result.HTTPStatus, result.ErrorCode, result.Message, result.LatencyMs, result.Action, result.ActionError, result.CheckedAt)
	return err
}

func (r *accountInspectionRepository) UpdateState(ctx context.Context, result service.AccountInspectionResult, patch service.AccountInspectionStatePatch) error {
	insertAutoDisabledSQL := "FALSE"
	insertAutoDisabledReasonSQL := "''"
	insertAutoDisabledAtSQL := "NULL"
	insertRestoredAtSQL := "NULL"
	updateAutoDisabledSQL := "account_inspection_states.auto_disabled"
	updateAutoDisabledReasonSQL := "account_inspection_states.auto_disabled_reason"
	updateAutoDisabledAtSQL := "account_inspection_states.auto_disabled_at"
	updateRestoredAtSQL := "account_inspection_states.restored_at"
	insertDeleteCandidateCategorySQL := "''"
	insertDeleteCandidateFirstSeenAtSQL := "NULL"
	insertDeleteCandidateCountSQL := "0"
	updateDeleteCandidateCategorySQL := "account_inspection_states.delete_candidate_category"
	updateDeleteCandidateFirstSeenAtSQL := "account_inspection_states.delete_candidate_first_seen_at"
	updateDeleteCandidateCountSQL := "account_inspection_states.delete_candidate_count"
	args := []any{
		result.AccountID,
		result.RunID,
		result.Status,
		result.Category,
		result.HTTPStatus,
		result.ErrorCode,
		result.Message,
		result.CheckedAt,
	}
	if patch.AutoDisabled != nil {
		args = append(args, *patch.AutoDisabled, patch.Reason, patch.At)
		insertAutoDisabledSQL = fmt.Sprintf("$%d", len(args)-2)
		insertAutoDisabledReasonSQL = fmt.Sprintf("$%d", len(args)-1)
		updateAutoDisabledSQL = insertAutoDisabledSQL
		updateAutoDisabledReasonSQL = insertAutoDisabledReasonSQL
		if *patch.AutoDisabled {
			insertAutoDisabledAtSQL = fmt.Sprintf("$%d", len(args))
			updateAutoDisabledAtSQL = insertAutoDisabledAtSQL
		} else {
			insertAutoDisabledAtSQL = "NULL"
			insertRestoredAtSQL = fmt.Sprintf("$%d", len(args))
			updateAutoDisabledAtSQL = "NULL"
			updateRestoredAtSQL = insertRestoredAtSQL
		}
	}
	if patch.UpdateDeleteCandidate {
		firstSeen := any(nil)
		if patch.DeleteCandidateFirstSeenAt != nil && patch.DeleteCandidateCategory != "" {
			firstSeen = *patch.DeleteCandidateFirstSeenAt
		}
		count := patch.DeleteCandidateCount
		if patch.DeleteCandidateCategory == "" {
			count = 0
		}
		args = append(args, patch.DeleteCandidateCategory, firstSeen, count)
		insertDeleteCandidateCategorySQL = fmt.Sprintf("$%d", len(args)-2)
		insertDeleteCandidateFirstSeenAtSQL = fmt.Sprintf("$%d", len(args)-1)
		insertDeleteCandidateCountSQL = fmt.Sprintf("$%d", len(args))
		updateDeleteCandidateCategorySQL = insertDeleteCandidateCategorySQL
		updateDeleteCandidateFirstSeenAtSQL = insertDeleteCandidateFirstSeenAtSQL
		updateDeleteCandidateCountSQL = insertDeleteCandidateCountSQL
	}

	query := fmt.Sprintf(`
		INSERT INTO account_inspection_states (
			account_id, last_run_id, last_status, last_category, last_http_status, last_error_code,
			last_message, last_checked_at, auto_disabled, auto_disabled_reason, auto_disabled_at, restored_at,
			delete_candidate_category, delete_candidate_first_seen_at, delete_candidate_count,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, %s, %s, %s, %s, %s, %s, %s, NOW(), NOW())
		ON CONFLICT (account_id) DO UPDATE SET
			last_run_id = EXCLUDED.last_run_id,
			last_status = EXCLUDED.last_status,
			last_category = EXCLUDED.last_category,
			last_http_status = EXCLUDED.last_http_status,
			last_error_code = EXCLUDED.last_error_code,
			last_message = EXCLUDED.last_message,
			last_checked_at = EXCLUDED.last_checked_at,
			auto_disabled = %s,
			auto_disabled_reason = %s,
			auto_disabled_at = %s,
			restored_at = %s,
			delete_candidate_category = %s,
			delete_candidate_first_seen_at = %s,
			delete_candidate_count = %s,
			updated_at = NOW()
	`, insertAutoDisabledSQL, insertAutoDisabledReasonSQL, insertAutoDisabledAtSQL, insertRestoredAtSQL,
		insertDeleteCandidateCategorySQL, insertDeleteCandidateFirstSeenAtSQL, insertDeleteCandidateCountSQL,
		updateAutoDisabledSQL, updateAutoDisabledReasonSQL, updateAutoDisabledAtSQL, updateRestoredAtSQL,
		updateDeleteCandidateCategorySQL, updateDeleteCandidateFirstSeenAtSQL, updateDeleteCandidateCountSQL)
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *accountInspectionRepository) GetAccountAutoDisabled(ctx context.Context, accountID int64) (bool, error) {
	var autoDisabled bool
	err := r.db.QueryRowContext(ctx, `SELECT auto_disabled FROM account_inspection_states WHERE account_id = $1`, accountID).Scan(&autoDisabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return autoDisabled, err
}

func (r *accountInspectionRepository) ListRecentResults(ctx context.Context, runID int64, limit int) ([]service.AccountInspectionResult, error) {
	if limit <= 0 || limit > 200 {
		limit = 30
	}
	args := []any{limit}
	where := ""
	if runID > 0 {
		where = "WHERE run_id = $2"
		args = append(args, runID)
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT id, run_id, account_id, name, platform, type, status, category, http_status, error_code, message,
			latency_ms, action, action_error, checked_at, created_at
		FROM account_inspection_results
		%s
		ORDER BY checked_at DESC, id DESC
		LIMIT $1
	`, where), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanAccountInspectionResults(rows)
}

func (r *accountInspectionRepository) ListLogs(ctx context.Context, limit int) ([]service.AccountInspectionLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 120
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, run_id, level, message, created_at
		FROM account_inspection_logs
		ORDER BY created_at DESC, id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	logs := make([]service.AccountInspectionLog, 0, limit)
	for rows.Next() {
		var item service.AccountInspectionLog
		var runID sql.NullInt64
		if err := rows.Scan(&item.ID, &runID, &item.Level, &item.Message, &item.CreatedAt); err != nil {
			return nil, err
		}
		if runID.Valid {
			item.RunID = &runID.Int64
		}
		logs = append(logs, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	reverseInspectionLogs(logs)
	return logs, nil
}

func (r *accountInspectionRepository) GetSummary(ctx context.Context, runID int64, total int64) (service.AccountInspectionSummary, error) {
	summary := service.AccountInspectionSummary{Total: total, ByCategory: map[string]int{}}
	if runID <= 0 {
		return summary, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT category, status, action, action_error, http_status, COUNT(*)
		FROM account_inspection_results
		WHERE run_id = $1
		GROUP BY category, status, action, action_error, http_status
	`, runID)
	if err != nil {
		return summary, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var category, status, action string
		var actionError sql.NullString
		var httpStatus int
		var count int64
		if err := rows.Scan(&category, &status, &action, &actionError, &httpStatus, &count); err != nil {
			return summary, err
		}
		summary.Checked += count
		summary.ByCategory[category] += int(count)
		switch status {
		case service.AccountHealthStatusAvailable:
			summary.Available += count
		case service.AccountHealthStatusRateLimited:
			summary.RateLimited += count
		case service.AccountHealthStatusUnavailable:
			summary.Unavailable += count
		}
		switch category {
		case service.AccountHealthCategoryQuotaExhausted:
			summary.QuotaExhausted += count
		case service.AccountHealthCategoryAuthInvalid:
			if httpStatus == 401 || httpStatus == 0 {
				summary.Auth401 += count
			} else {
				summary.OtherFailure += count
			}
		case service.AccountHealthCategoryPaymentRequired:
			summary.Payment402 += count
		case service.AccountHealthCategoryUnknownError:
			summary.Unknown += count
		case service.AccountHealthCategoryAvailable, service.AccountHealthCategoryRateLimited:
		default:
			if status == service.AccountHealthStatusUnavailable {
				summary.OtherFailure += count
			}
		}
		if actionError.Valid && strings.TrimSpace(actionError.String) != "" {
			summary.ActionFailed += count
			continue
		}
		switch action {
		case service.AccountInspectionActionDelete:
			summary.Deleted += count
		case service.AccountInspectionActionRetain:
			summary.Retained += count
		case service.AccountInspectionActionPendingDelete:
			summary.PendingDelete += count
		case service.AccountInspectionActionDisable:
			summary.Disabled += count
		case service.AccountInspectionActionRestore:
			summary.Restored += count
		}
	}
	return summary, rows.Err()
}

func (r *accountInspectionRepository) RecordLog(ctx context.Context, runID *int64, level, message string) error {
	if strings.TrimSpace(level) == "" {
		level = service.AccountInspectionLogInfo
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO account_inspection_logs (run_id, level, message, created_at)
		VALUES ($1, $2, $3, NOW())
	`, runID, level, message)
	return err
}

func (r *accountInspectionRepository) Prune(ctx context.Context, keepLogs, keepResults int) error {
	if keepLogs > 0 {
		if _, err := r.db.ExecContext(ctx, `
			DELETE FROM account_inspection_logs
			WHERE id IN (
				SELECT id FROM (
					SELECT id, ROW_NUMBER() OVER (ORDER BY created_at DESC, id DESC) AS rn
					FROM account_inspection_logs
				) ranked
				WHERE rn > $1
			)
		`, keepLogs); err != nil {
			return err
		}
	}
	if keepResults > 0 {
		if _, err := r.db.ExecContext(ctx, `
			DELETE FROM account_inspection_results
			WHERE id IN (
				SELECT id FROM (
					SELECT id, ROW_NUMBER() OVER (ORDER BY checked_at DESC, id DESC) AS rn
					FROM account_inspection_results
				) ranked
				WHERE rn > $1
			)
		`, keepResults); err != nil {
			return err
		}
	}
	return nil
}

func accountInspectionCandidateWhere(settings service.AccountInspectionSettings) (string, []any, error) {
	args := make([]any, 0, 10)
	clauses := []string{"a.deleted_at IS NULL", "a.id > $1"}
	args = append(args, settings.Cursor)

	if settings.Filters.Platform != "" {
		args = append(args, settings.Filters.Platform)
		clauses = append(clauses, fmt.Sprintf("a.platform = $%d", len(args)))
	}
	if settings.Filters.Type != "" {
		args = append(args, settings.Filters.Type)
		clauses = append(clauses, fmt.Sprintf("a.type = $%d", len(args)))
	}
	if settings.Filters.Search != "" {
		args = append(args, "%"+settings.Filters.Search+"%")
		clauses = append(clauses, fmt.Sprintf("a.name ILIKE $%d", len(args)))
	}
	if settings.Filters.Status != "" {
		clauses = append(clauses, accountInspectionStatusClause(settings.Filters.Status))
	}
	if settings.Filters.Group != "" {
		if settings.Filters.Group == "ungrouped" {
			clauses = append(clauses, "NOT EXISTS (SELECT 1 FROM account_groups ag WHERE ag.account_id = a.id)")
		} else {
			groupID, err := strconv.ParseInt(settings.Filters.Group, 10, 64)
			if err != nil || groupID <= 0 {
				return "", nil, fmt.Errorf("invalid group filter")
			}
			args = append(args, groupID)
			clauses = append(clauses, fmt.Sprintf("EXISTS (SELECT 1 FROM account_groups ag WHERE ag.account_id = a.id AND ag.group_id = $%d)", len(args)))
		}
	}
	if settings.Filters.PrivacyMode != "" {
		if settings.Filters.PrivacyMode == service.AccountPrivacyModeUnsetFilter {
			clauses = append(clauses, "(NOT (COALESCE(a.extra, '{}'::jsonb) ? 'privacy_mode') OR COALESCE(a.extra->>'privacy_mode', '') = '')")
		} else {
			args = append(args, settings.Filters.PrivacyMode)
			clauses = append(clauses, fmt.Sprintf("COALESCE(a.extra->>'privacy_mode', '') = $%d", len(args)))
		}
	}
	if !settings.IncludeUnschedulable {
		clauses = append(clauses, "a.schedulable = TRUE")
	}

	if settings.RecheckAfterHours > 0 {
		args = append(args, settings.RecheckAfterHours)
		recheckArg := len(args)
		clauses = append(clauses, fmt.Sprintf(`(
			s.last_checked_at IS NULL OR
			s.last_checked_at <= NOW() - ($%d::int * INTERVAL '1 hour')
		)`, recheckArg))
	}

	return "WHERE " + strings.Join(clauses, " AND "), args, nil
}

func accountInspectionStatusClause(status string) string {
	switch status {
	case service.StatusActive:
		return `a.status = 'active'
			AND a.schedulable = TRUE
			AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= NOW())
			AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= NOW())`
	case "rate_limited":
		return `a.status = 'active'
			AND a.rate_limit_reset_at > NOW()
			AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= NOW())`
	case "temp_unschedulable":
		return `a.status = 'active'
			AND a.temp_unschedulable_until > NOW()`
	case "unschedulable":
		return `a.status = 'active'
			AND a.schedulable = FALSE
			AND (a.rate_limit_reset_at IS NULL OR a.rate_limit_reset_at <= NOW())
			AND (a.temp_unschedulable_until IS NULL OR a.temp_unschedulable_until <= NOW())`
	case service.StatusDisabled, service.StatusError:
		return "a.status = '" + status + "'"
	default:
		return "FALSE"
	}
}

func (r *accountInspectionRepository) countCandidatesWithWhere(ctx context.Context, where string, args []any) (int64, error) {
	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM accounts a
		LEFT JOIN account_inspection_states s ON s.account_id = a.id
		%s
	`, where)
	var total int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&total)
	return total, err
}

type accountInspectionScannable interface {
	Scan(dest ...any) error
}

func scanAccountInspectionRun(row accountInspectionScannable) (*service.AccountInspectionRun, error) {
	run := &service.AccountInspectionRun{}
	var snapshotRaw []byte
	var errText sql.NullString
	if err := row.Scan(
		&run.ID, &run.Status, &snapshotRaw, &run.TotalAccounts, &run.Cursor, &run.NextCursor, &run.HasMore,
		&errText, &run.StartedAt, &run.FinishedAt, &run.CreatedAt, &run.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if errText.Valid {
		run.Error = errText.String
	}
	if len(snapshotRaw) > 0 {
		_ = json.Unmarshal(snapshotRaw, &run.SettingsSnapshot)
	}
	run.SettingsSnapshot = service.NormalizeAccountInspectionSettings(run.SettingsSnapshot)
	return run, nil
}

func scanAccountInspectionResults(rows *sql.Rows) ([]service.AccountInspectionResult, error) {
	results := make([]service.AccountInspectionResult, 0)
	for rows.Next() {
		var item service.AccountInspectionResult
		var accountID sql.NullInt64
		var createdAt time.Time
		if err := rows.Scan(
			&item.ID, &item.RunID, &accountID, &item.Name, &item.Platform, &item.Type, &item.Status, &item.Category,
			&item.HTTPStatus, &item.ErrorCode, &item.Message, &item.LatencyMs, &item.Action, &item.ActionError,
			&item.CheckedAt, &createdAt,
		); err != nil {
			return nil, err
		}
		if accountID.Valid {
			item.AccountID = accountID.Int64
		}
		item.CreatedAt = &createdAt
		results = append(results, item)
	}
	return results, rows.Err()
}

func reverseInspectionLogs(items []service.AccountInspectionLog) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func inspectionNullString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
