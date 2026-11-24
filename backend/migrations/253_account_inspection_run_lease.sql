ALTER TABLE account_inspection_runs ADD COLUMN IF NOT EXISTS lease_expires_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_account_inspection_runs_active ON account_inspection_runs (id)
    WHERE status IN ('queued', 'running', 'stopping');
