BEGIN;

DROP INDEX IF EXISTS reviews_one_active_per_user_app;
DROP INDEX IF EXISTS legacy_devices_identifier_unique;
DROP TABLE IF EXISTS legacy_devices;
DROP TABLE IF EXISTS legacy_passwords;
DROP TABLE IF EXISTS sessions;

ALTER TABLE users
    DROP COLUMN IF EXISTS two_factor_confirmed_at,
    DROP COLUMN IF EXISTS two_factor_secret;

COMMIT;
