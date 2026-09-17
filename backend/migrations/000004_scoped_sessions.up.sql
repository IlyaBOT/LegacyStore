BEGIN;

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS auth_kind text NOT NULL DEFAULT 'web',
    ADD COLUMN IF NOT EXISTS scopes text[] NOT NULL DEFAULT ARRAY[]::text[],
    ADD COLUMN IF NOT EXISTS legacy_password_id bigint REFERENCES legacy_passwords(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS legacy_device_identifier text;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'sessions_auth_kind_check'
    ) THEN
        ALTER TABLE sessions
            ADD CONSTRAINT sessions_auth_kind_check
            CHECK (auth_kind IN ('web', 'legacy'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS sessions_legacy_password_id_idx
    ON sessions (legacy_password_id)
    WHERE legacy_password_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS sessions_legacy_device_identifier_idx
    ON sessions (user_id, legacy_device_identifier)
    WHERE legacy_device_identifier IS NOT NULL;

COMMIT;
