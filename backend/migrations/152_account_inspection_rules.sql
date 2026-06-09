ALTER TABLE account_inspection_states
    ADD COLUMN IF NOT EXISTS delete_candidate_category VARCHAR(50) NOT NULL DEFAULT '';

ALTER TABLE account_inspection_states
    ADD COLUMN IF NOT EXISTS delete_candidate_first_seen_at TIMESTAMPTZ;

ALTER TABLE account_inspection_states
    ADD COLUMN IF NOT EXISTS delete_candidate_count INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_account_inspection_states_delete_candidate
    ON account_inspection_states (delete_candidate_category, delete_candidate_first_seen_at);

UPDATE settings
SET value = (
    '{
      "batch_limit": 50,
      "batch_sleep_seconds": 30,
      "low_resource_mode": true,
      "max_accounts_per_run": 1000,
      "auto_pause_consecutive_errors": 3,
      "delete_auth_invalid_min_consecutive": 1,
      "delete_auth_invalid_after_hours": 0,
      "delete_quota_exhausted": false,
      "delete_quota_exhausted_min_consecutive": 1,
      "delete_quota_exhausted_after_hours": 168,
      "delete_payment_required": false,
      "delete_payment_required_min_consecutive": 1,
      "delete_payment_required_after_hours": 168
    }'::jsonb || value::jsonb
)::text,
updated_at = NOW()
WHERE key = 'account_inspection_settings'
  AND value IS NOT NULL
  AND trim(value) <> '';
