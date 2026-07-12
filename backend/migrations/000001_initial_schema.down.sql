BEGIN;

DROP TRIGGER IF EXISTS user_roles_single_admin ON user_roles;

DROP TRIGGER IF EXISTS moderation_queue_set_updated_at ON moderation_queue;
DROP TRIGGER IF EXISTS review_replies_set_updated_at ON review_replies;
DROP TRIGGER IF EXISTS reviews_set_updated_at ON reviews;
DROP TRIGGER IF EXISTS screenshots_set_updated_at ON screenshots;
DROP TRIGGER IF EXISTS icons_set_updated_at ON icons;
DROP TRIGGER IF EXISTS artifact_mirrors_set_updated_at ON artifact_mirrors;
DROP TRIGGER IF EXISTS artifacts_set_updated_at ON artifacts;
DROP TRIGGER IF EXISTS app_versions_set_updated_at ON app_versions;
DROP TRIGGER IF EXISTS apps_set_updated_at ON apps;
DROP TRIGGER IF EXISTS categories_set_updated_at ON categories;
DROP TRIGGER IF EXISTS users_set_updated_at ON users;

DROP FUNCTION IF EXISTS enforce_single_admin_role();
DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS moderation_queue;
DROP TABLE IF EXISTS review_likes;
DROP TABLE IF EXISTS review_replies;
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS screenshots;
DROP TABLE IF EXISTS icons;
DROP TABLE IF EXISTS artifact_mirrors;
DROP TABLE IF EXISTS artifacts;
DROP TABLE IF EXISTS app_versions;
DROP TABLE IF EXISTS app_categories;
DROP TABLE IF EXISTS apps;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS users;

COMMIT;
