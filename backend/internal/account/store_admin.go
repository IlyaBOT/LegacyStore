package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

func (s *Store) Dashboard(ctx context.Context) (map[string]int, error) {
	counts := map[string]int{}
	queries := map[string]string{
		"users":              `SELECT COUNT(*) FROM users`,
		"apps_pending":       `SELECT COUNT(*) FROM apps WHERE moderation_status = 'pending'`,
		"apps_approved":      `SELECT COUNT(*) FROM apps WHERE moderation_status = 'approved'`,
		"artifacts_pending":  `SELECT COUNT(*) FROM artifacts WHERE moderation_status = 'pending'`,
		"reviews":            `SELECT COUNT(*) FROM reviews WHERE deleted_at IS NULL`,
		"moderation_pending": `SELECT COUNT(*) FROM moderation_queue WHERE status = 'pending'`,
	}
	for key, query := range queries {
		var count int
		if err := s.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			return nil, err
		}
		counts[key] = count
	}
	return counts, nil
}

func (s *Store) ListUsers(ctx context.Context, limit int) ([]User, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, email, COALESCE(nickname, ''), COALESCE(avatar_url, ''), email_verified, two_factor_enabled, status, created_at::text, updated_at::text
		FROM users
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.Nickname, &user.AvatarURL, &user.EmailVerified, &user.TwoFactorEnabled, &user.Status, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return nil, err
		}
		roles, err := s.roles(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		user.Roles = roles
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Store) AdminUpdateUser(ctx context.Context, actor User, userID int64, nickname, status string, emailVerified *bool, ip net.IP, userAgent string) (*User, error) {
	if status != "" && status != "active" && status != "disabled" && status != "pending" {
		return nil, ErrInvalidCredential
	}
	if emailVerified == nil {
		_, err := s.db.ExecContext(ctx, `
			UPDATE users
			SET nickname = COALESCE(NULLIF($2, ''), nickname),
			    status = COALESCE(NULLIF($3, ''), status)
			WHERE id = $1
		`, userID, strings.TrimSpace(nickname), status)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := s.db.ExecContext(ctx, `
			UPDATE users
			SET nickname = COALESCE(NULLIF($2, ''), nickname),
			    status = COALESCE(NULLIF($3, ''), status),
			    email_verified = $4
			WHERE id = $1
		`, userID, strings.TrimSpace(nickname), status, *emailVerified)
		if err != nil {
			return nil, err
		}
	}
	_ = s.audit(ctx, actor.ID, "admin.users.update", "user", intString(userID), ip, userAgent)
	return s.UserByID(ctx, userID)
}

func (s *Store) AddRole(ctx context.Context, actor User, userID int64, role string, ip net.IP, userAgent string) error {
	if !validRole(role) {
		return ErrInvalidCredential
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = $2
		ON CONFLICT DO NOTHING
	`, userID, role)
	if err != nil {
		return err
	}
	return s.audit(ctx, actor.ID, "admin.roles.add", "user", intString(userID), ip, userAgent)
}

func (s *Store) RemoveRole(ctx context.Context, actor User, userID int64, role string, ip net.IP, userAgent string) error {
	if role == "admin" && actor.ID == userID {
		return ErrForbidden
	}
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM user_roles
		WHERE user_id = $1 AND role_id = (SELECT id FROM roles WHERE name = $2)
	`, userID, role)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return s.audit(ctx, actor.ID, "admin.roles.remove", "user", intString(userID), ip, userAgent)
}

func (s *Store) ListAdminApps(ctx context.Context, status string, limit int) ([]AdminApp, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	args := []any{limit}
	where := ""
	if status != "" {
		args = append(args, status)
		where = "WHERE a.moderation_status = $2"
	}
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT a.id, a.slug, a.name, COALESCE(a.bundle_id, ''), a.developer_name, a.summary, COALESCE(a.description, ''),
		       COALESCE(c.name, ''), a.moderation_status, a.created_at::text, a.updated_at::text
		FROM apps a
		LEFT JOIN app_categories ac ON ac.app_id = a.id
		LEFT JOIN categories c ON c.id = ac.category_id
		%s
		ORDER BY a.updated_at DESC
		LIMIT $1
	`, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var apps []AdminApp
	for rows.Next() {
		var app AdminApp
		if err := rows.Scan(&app.ID, &app.Slug, &app.Name, &app.BundleID, &app.DeveloperName, &app.Summary, &app.Description, &app.Category, &app.ModerationStatus, &app.CreatedAt, &app.UpdatedAt); err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}
	return apps, rows.Err()
}

func (s *Store) CreateAdminApp(ctx context.Context, actor User, app AdminApp, categorySlug string, ip net.IP, userAgent string) (*AdminApp, error) {
	status := "pending"
	if HasRole(actor, "admin", "moder") {
		status = "approved"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var created AdminApp
	err = tx.QueryRowContext(ctx, `
		INSERT INTO apps (slug, name, bundle_id, developer_name, summary, description, website_url, license_type, created_by, moderation_status)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, NULLIF($6, ''), '', '', $7, $8)
		RETURNING id, slug, name, COALESCE(bundle_id, ''), developer_name, summary, COALESCE(description, ''), moderation_status, created_at::text, updated_at::text
	`, app.Slug, app.Name, app.BundleID, app.DeveloperName, app.Summary, app.Description, actor.ID, status).Scan(
		&created.ID, &created.Slug, &created.Name, &created.BundleID, &created.DeveloperName, &created.Summary, &created.Description, &created.ModerationStatus, &created.CreatedAt, &created.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if categorySlug != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO app_categories (app_id, category_id)
			SELECT $1, id FROM categories WHERE slug = $2
			ON CONFLICT DO NOTHING
		`, created.ID, categorySlug); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO moderation_queue (entity_type, entity_id, submitted_by, status)
		VALUES ('app', $1, $2, $3)
	`, strconv.FormatInt(created.ID, 10), actor.ID, status); err != nil {
		return nil, err
	}
	if err := auditTx(ctx, tx, actor.ID, "admin.apps.create", "app", intString(created.ID), ip, userAgent); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *Store) UpdateAdminApp(ctx context.Context, actor User, appID int64, app AdminApp, ip net.IP, userAgent string) (*AdminApp, error) {
	var updated AdminApp
	err := s.db.QueryRowContext(ctx, `
		UPDATE apps
		SET slug = COALESCE(NULLIF($2, ''), slug),
		    name = COALESCE(NULLIF($3, ''), name),
		    bundle_id = COALESCE(NULLIF($4, ''), bundle_id),
		    developer_name = COALESCE(NULLIF($5, ''), developer_name),
		    summary = COALESCE(NULLIF($6, ''), summary),
		    description = COALESCE(NULLIF($7, ''), description),
		    moderation_status = COALESCE(NULLIF($8, ''), moderation_status)
		WHERE id = $1
		RETURNING id, slug, name, COALESCE(bundle_id, ''), developer_name, summary, COALESCE(description, ''), moderation_status, created_at::text, updated_at::text
	`, appID, app.Slug, app.Name, app.BundleID, app.DeveloperName, app.Summary, app.Description, app.ModerationStatus).Scan(
		&updated.ID, &updated.Slug, &updated.Name, &updated.BundleID, &updated.DeveloperName, &updated.Summary, &updated.Description, &updated.ModerationStatus, &updated.CreatedAt, &updated.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = s.audit(ctx, actor.ID, "admin.apps.update", "app", intString(appID), ip, userAgent)
	return &updated, nil
}

func (s *Store) DeleteAdminApp(ctx context.Context, actor User, appID int64, ip net.IP, userAgent string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM apps WHERE id = $1`, appID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return s.audit(ctx, actor.ID, "admin.apps.delete", "app", intString(appID), ip, userAgent)
}

func (s *Store) ListModeration(ctx context.Context, status string) ([]ModerationItem, error) {
	if status == "" {
		status = "pending"
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, COALESCE(submitted_by, 0), status, COALESCE(moderator_id, 0),
		       COALESCE(moderator_comment, ''), created_at::text, updated_at::text
		FROM moderation_queue
		WHERE status = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []ModerationItem
	for rows.Next() {
		var item ModerationItem
		if err := rows.Scan(&item.ID, &item.EntityType, &item.EntityID, &item.SubmittedBy, &item.Status, &item.ModeratorID, &item.ModeratorComment, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) Moderate(ctx context.Context, actor User, queueID int64, approve bool, comment string, ip net.IP, userAgent string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var entityType, entityID string
	err = tx.QueryRowContext(ctx, `SELECT entity_type, entity_id FROM moderation_queue WHERE id = $1 AND status = 'pending'`, queueID).Scan(&entityType, &entityID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	status := "rejected"
	action := "moderation.reject"
	if approve {
		status = "approved"
		action = "moderation.approve"
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE moderation_queue
		SET status = $2, moderator_id = $3, moderator_comment = NULLIF($4, '')
		WHERE id = $1
	`, queueID, status, actor.ID, strings.TrimSpace(comment)); err != nil {
		return err
	}
	if entityType == "app" {
		if _, err := tx.ExecContext(ctx, `UPDATE apps SET moderation_status = $2 WHERE id = $1`, entityID, status); err != nil {
			return err
		}
	}
	if entityType == "artifact" {
		if _, err := tx.ExecContext(ctx, `UPDATE artifacts SET moderation_status = $2 WHERE id = $1`, entityID, status); err != nil {
			return err
		}
	}
	if err := auditTx(ctx, tx, actor.ID, action, entityType, entityID, ip, userAgent); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AuditLog(ctx context.Context) ([]map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT COALESCE(actor_user_id::text, ''), action, COALESCE(entity_type, ''), COALESCE(entity_id, ''),
		       COALESCE(ip_address::text, ''), COALESCE(user_agent, ''), created_at::text
		FROM audit_log
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []map[string]string
	for rows.Next() {
		item := map[string]string{}
		var actor, action, entityType, entityID, ip, ua, created string
		if err := rows.Scan(&actor, &action, &entityType, &entityID, &ip, &ua, &created); err != nil {
			return nil, err
		}
		item["actor_user_id"] = actor
		item["action"] = action
		item["entity_type"] = entityType
		item["entity_id"] = entityID
		item["ip_address"] = ip
		item["user_agent"] = ua
		item["created_at"] = created
		items = append(items, item)
	}
	return items, rows.Err()
}

func validRole(role string) bool {
	switch role {
	case "guest", "user", "trusted", "moder", "admin":
		return true
	default:
		return false
	}
}

func (s *Store) audit(ctx context.Context, actorID int64, action, entityType, entityID string, ip net.IP, userAgent string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO audit_log (actor_user_id, action, entity_type, entity_id, ip_address, user_agent)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6)
	`, actorID, action, entityType, entityID, nullableIP(ip), truncate(userAgent, 512))
	return err
}

func auditTx(ctx context.Context, tx *sql.Tx, actorID int64, action, entityType, entityID string, ip net.IP, userAgent string) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO audit_log (actor_user_id, action, entity_type, entity_id, ip_address, user_agent)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6)
	`, actorID, action, entityType, entityID, nullableIP(ip), truncate(userAgent, 512))
	return err
}

func intString(id int64) string {
	return strconv.FormatInt(id, 10)
}
