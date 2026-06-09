CREATE TABLE IF NOT EXISTS account_inspection_runs (
    id BIGSERIAL PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    settings_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    total_accounts BIGINT NOT NULL DEFAULT 0,
    cursor BIGINT NOT NULL DEFAULT 0,
    next_cursor BIGINT NOT NULL DEFAULT 0,
    has_more BOOLEAN NOT NULL DEFAULT FALSE,
    error TEXT,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_inspection_runs_created_at
    ON account_inspection_runs (created_at DESC);

CREATE TABLE IF NOT EXISTS account_inspection_states (
    account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    last_run_id BIGINT REFERENCES account_inspection_runs(id) ON DELETE SET NULL,
    last_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    last_category VARCHAR(50) NOT NULL DEFAULT '',
    last_http_status INTEGER NOT NULL DEFAULT 0,
    last_error_code VARCHAR(100) NOT NULL DEFAULT '',
    last_message TEXT NOT NULL DEFAULT '',
    last_checked_at TIMESTAMPTZ,
    auto_disabled BOOLEAN NOT NULL DEFAULT FALSE,
    auto_disabled_reason VARCHAR(100) NOT NULL DEFAULT '',
    auto_disabled_at TIMESTAMPTZ,
    restored_at TIMESTAMPTZ,
    delete_candidate_category VARCHAR(50) NOT NULL DEFAULT '',
    delete_candidate_first_seen_at TIMESTAMPTZ,
    delete_candidate_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE account_inspection_states
    ADD COLUMN IF NOT EXISTS delete_candidate_category VARCHAR(50) NOT NULL DEFAULT '';

ALTER TABLE account_inspection_states
    ADD COLUMN IF NOT EXISTS delete_candidate_first_seen_at TIMESTAMPTZ;

ALTER TABLE account_inspection_states
    ADD COLUMN IF NOT EXISTS delete_candidate_count INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_account_inspection_states_last_checked
    ON account_inspection_states (last_checked_at);

CREATE INDEX IF NOT EXISTS idx_account_inspection_states_auto_disabled
    ON account_inspection_states (auto_disabled);

CREATE INDEX IF NOT EXISTS idx_account_inspection_states_delete_candidate
    ON account_inspection_states (delete_candidate_category, delete_candidate_first_seen_at);

CREATE TABLE IF NOT EXISTS account_inspection_results (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES account_inspection_runs(id) ON DELETE CASCADE,
    account_id BIGINT,
    name VARCHAR(255) NOT NULL DEFAULT '',
    platform VARCHAR(50) NOT NULL DEFAULT '',
    type VARCHAR(50) NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL,
    category VARCHAR(50) NOT NULL,
    http_status INTEGER NOT NULL DEFAULT 0,
    error_code VARCHAR(100) NOT NULL DEFAULT '',
    message TEXT NOT NULL DEFAULT '',
    latency_ms BIGINT NOT NULL DEFAULT 0,
    action VARCHAR(20) NOT NULL DEFAULT 'none',
    action_error TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_inspection_results_run_id_created
    ON account_inspection_results (run_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_account_inspection_results_account_id
    ON account_inspection_results (account_id);

CREATE INDEX IF NOT EXISTS idx_account_inspection_results_category
    ON account_inspection_results (category);

CREATE TABLE IF NOT EXISTS account_inspection_logs (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT REFERENCES account_inspection_runs(id) ON DELETE CASCADE,
    level VARCHAR(20) NOT NULL DEFAULT 'info',
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_account_inspection_logs_created_at
    ON account_inspection_logs (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_account_inspection_logs_run_id
    ON account_inspection_logs (run_id);

INSERT INTO settings (key, value)
VALUES ('account_inspection_settings', '{
  "enabled": false,
  "filters": {},
  "model_id": "",
  "batch_limit": 50,
  "concurrency": 1,
  "batch_sleep_seconds": 30,
  "recheck_after_hours": 168,
  "low_resource_mode": true,
  "max_accounts_per_run": 1000,
  "auto_pause_consecutive_errors": 3,
  "include_unschedulable": true,
  "delete_auth_invalid": true,
  "delete_auth_invalid_min_consecutive": 1,
  "delete_auth_invalid_after_hours": 0,
  "delete_quota_exhausted": false,
  "delete_quota_exhausted_min_consecutive": 1,
  "delete_quota_exhausted_after_hours": 168,
  "delete_payment_required": false,
  "delete_payment_required_min_consecutive": 1,
  "delete_payment_required_after_hours": 168,
  "disable_quota_exhausted": true,
  "restore_auto_disabled": true,
  "cursor": 0
}')
ON CONFLICT (key) DO NOTHING;
