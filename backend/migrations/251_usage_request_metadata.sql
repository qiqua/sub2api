-- No defaults/backfill: historical request metadata was never recorded.
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS http_method VARCHAR(16),
    ADD COLUMN IF NOT EXISTS status_code INTEGER,
    ADD COLUMN IF NOT EXISTS attempt_count INTEGER;

COMMENT ON COLUMN usage_logs.http_method IS 'Observed inbound HTTP method; NULL when not captured';
COMMENT ON COLUMN usage_logs.status_code IS 'Committed client-facing HTTP status; NULL when not captured';
COMMENT ON COLUMN usage_logs.attempt_count IS 'Observed upstream request/response.create dispatches for this turn including retries; NULL when not captured';
