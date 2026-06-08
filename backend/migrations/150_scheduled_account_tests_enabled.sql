INSERT INTO settings (key, value)
VALUES ('scheduled_account_tests_enabled', 'false')
ON CONFLICT (key) DO NOTHING;
