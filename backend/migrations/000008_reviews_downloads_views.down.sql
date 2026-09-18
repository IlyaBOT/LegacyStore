BEGIN;

DROP TABLE IF EXISTS app_view_events;

DROP INDEX IF EXISTS download_events_completed_app_idx;
DROP INDEX IF EXISTS download_events_anonymous_ip_unique;
DROP INDEX IF EXISTS download_events_logged_unique;

ALTER TABLE download_events
    DROP CONSTRAINT IF EXISTS download_events_os_arch_check,
    DROP CONSTRAINT IF EXISTS download_events_source_check,
    DROP COLUMN IF EXISTS completed_at,
    DROP COLUMN IF EXISTS device_model,
    DROP COLUMN IF EXISTS os_arch,
    DROP COLUMN IF EXISTS os_version,
    DROP COLUMN IF EXISTS client_version,
    DROP COLUMN IF EXISTS user_agent,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS ip_address,
    DROP COLUMN IF EXISTS username_snapshot,
    DROP COLUMN IF EXISTS user_id,
    DROP COLUMN IF EXISTS app_version_id;

DROP INDEX IF EXISTS reviews_app_version_id_idx;
DROP INDEX IF EXISTS reviews_public_uid_unique;

ALTER TABLE reviews
    DROP CONSTRAINT IF EXISTS reviews_source_check,
    DROP CONSTRAINT IF EXISTS reviews_os_arch_check,
    DROP CONSTRAINT IF EXISTS reviews_title_length_check,
    DROP CONSTRAINT IF EXISTS reviews_body_length_check,
    DROP CONSTRAINT IF EXISTS reviews_public_uid_length_check,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS client_version,
    DROP COLUMN IF EXISTS device_model,
    DROP COLUMN IF EXISTS os_arch,
    DROP COLUMN IF EXISTS os_version,
    DROP COLUMN IF EXISTS app_version,
    DROP COLUMN IF EXISTS app_version_id,
    DROP COLUMN IF EXISTS public_uid;

COMMIT;
