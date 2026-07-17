ALTER TABLE api_keys
	ADD COLUMN IF NOT EXISTS routing_mode VARCHAR(16) NOT NULL DEFAULT 'fixed',
	ADD COLUMN IF NOT EXISTS auto_route_group_ids JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE api_keys
SET routing_mode = 'fixed'
WHERE routing_mode IS NULL OR routing_mode = '';

UPDATE api_keys
SET auto_route_group_ids = '[]'::jsonb
WHERE auto_route_group_ids IS NULL;

DO $$
BEGIN
	IF NOT EXISTS (
		SELECT 1
		FROM pg_constraint
		WHERE conname = 'chk_api_keys_routing_mode'
	) THEN
		ALTER TABLE api_keys
			ADD CONSTRAINT chk_api_keys_routing_mode CHECK (routing_mode IN ('fixed', 'auto'));
	END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_api_keys_routing_mode
	ON api_keys (routing_mode)
	WHERE deleted_at IS NULL;
