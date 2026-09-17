BEGIN;

ALTER TABLE users
    DROP COLUMN IF EXISTS password_changed_at,
    DROP COLUMN IF EXISTS email_changed_at;

DROP TABLE IF EXISTS two_factor_recovery_codes;
DROP TABLE IF EXISTS password_recovery_tokens;

COMMIT;
