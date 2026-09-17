BEGIN;

DROP INDEX IF EXISTS sessions_legacy_device_identifier_idx;
DROP INDEX IF EXISTS sessions_legacy_password_id_idx;

ALTER TABLE sessions
    DROP CONSTRAINT IF EXISTS sessions_auth_kind_check,
    DROP COLUMN IF EXISTS legacy_device_identifier,
    DROP COLUMN IF EXISTS legacy_password_id,
    DROP COLUMN IF EXISTS scopes,
    DROP COLUMN IF EXISTS auth_kind;

COMMIT;
