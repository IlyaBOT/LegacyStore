package account

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strings"

	"github.com/lib/pq"
)

type AdminArtifactEntry struct {
	AdminArtifact
	AppName string `json:"app_name"`
	AppSlug string `json:"app_slug"`
	Version string `json:"version"`
}

type AdminReviewEntry struct {
	ID        int64  `json:"id"`
	UID       string `json:"uid"`
	AppID     int64  `json:"app_id"`
	AppSlug   string `json:"app_slug"`
	UserID    int64  `json:"user_id"`
	Author    string `json:"author"`
	Rating    int    `json:"rating"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Deleted   bool   `json:"deleted"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Store) AdminCreateUser(ctx context.Context, actor User, email, nickname, password, role string, ip net.IP, userAgent string) (*User, error) {
	email = normalizeEmail(email)
	nickname = strings.TrimSpace(nickname)
	role = strings.TrimSpace(role)
	if !validEmail(email) || len(password) < 8 || !validRole(role) || role == "admin" {
		return nil, ErrInvalidCredential
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var user User
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, nickname, email_verified, status)
		VALUES ($1, $2, NULLIF($3, ''), false, 'active')
		RETURNING id, email, COALESCE(nickname, ''), COALESCE(avatar_url, ''), email_verified, two_factor_enabled,
		          status, created_at::text, updated_at::text
	`, email, hash, nickname).Scan(
		&user.ID, &user.Email, &user.Nickname, &user.AvatarURL, &user.EmailVerified, &user.TwoFactorEnabled,
		&user.Status, &user.CreatedAt, &user.UpdatedAt,
	)
	if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
		return nil, ErrEmailExists
	}
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"user", role} {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id FROM roles WHERE name = $2
			ON CONFLICT DO NOTHING
		`, user.ID, name); err != nil {
			return nil, err
		}
	}
	if err := auditTx(ctx, tx, actor.ID, "admin.users.create", "user", intString(user.ID), ip, userAgent); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.UserByID(ctx, user.ID)
}

func (s *Store) AdminDeleteUser(ctx context.Context, actor User, userID int64, ip net.IP, userAgent string) error {
	if actor.ID == userID {
		return ErrForbidden
	}
	var isAdmin bool
	if err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_roles ur
			JOIN roles r ON r.id = ur.role_id
			WHERE ur.user_id = $1 AND r.name = 'admin'
		)
	`, userID).Scan(&isAdmin); err != nil {
		return err
	}
	if isAdmin {
		return ErrForbidden
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return s.audit(ctx, actor.ID, "admin.users.delete", "user", intString(userID), ip, userAgent)
}

func (s *Store) ListAdminArtifactsAll(ctx context.Context, limit int) ([]AdminArtifactEntry, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT ar.id, ar.app_version_id, ar.file_name, ar.package_type, ar.source_type,
		       COALESCE(ar.storage_path, ''), COALESCE(ar.primary_download_url, ''), COALESCE(ar.torrent_url, ''),
		       COALESCE(ar.magnet_url, ''), COALESCE(ar.size_bytes, 0), COALESCE(ar.sha256, ''), ar.min_os,
		       COALESCE(ar.max_supported_os, ''), COALESCE(ar.max_tested_os, ''), ar.hard_block_above_max,
		       ar.architectures, ar.requires_rosetta, ar.requires_java, COALESCE(ar.install_notes, ''),
		       ar.moderation_status, ar.created_at::text, ar.updated_at::text,
		       a.name, a.slug, v.version
		FROM artifacts ar
		JOIN app_versions v ON v.id = ar.app_version_id
		JOIN apps a ON a.id = v.app_id
		ORDER BY ar.created_at DESC, ar.id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AdminArtifactEntry, 0)
	for rows.Next() {
		var item AdminArtifactEntry
		if err := rows.Scan(
			&item.ID, &item.AppVersionID, &item.FileName, &item.PackageType, &item.SourceType,
			&item.StoragePath, &item.PrimaryDownloadURL, &item.TorrentURL, &item.MagnetURL,
			&item.SizeBytes, &item.SHA256, &item.MinOS, &item.MaxSupportedOS, &item.MaxTestedOS,
			&item.HardBlockAboveMax, pq.Array(&item.Architectures), &item.RequiresRosetta, &item.RequiresJava,
			&item.InstallNotes, &item.ModerationStatus, &item.CreatedAt, &item.UpdatedAt,
			&item.AppName, &item.AppSlug, &item.Version,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListAdminReviews(ctx context.Context, limit int) ([]AdminReviewEntry, error) {
	if limit < 1 || limit > 200 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.public_uid, r.app_id, a.slug, r.user_id,
		       COALESCE(NULLIF(u.nickname, ''), u.email), r.rating, COALESCE(r.title, ''), COALESCE(r.body, ''),
		       (r.deleted_at IS NOT NULL), r.created_at::text, r.updated_at::text
		FROM reviews r
		JOIN apps a ON a.id = r.app_id
		JOIN users u ON u.id = r.user_id
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AdminReviewEntry, 0)
	for rows.Next() {
		var item AdminReviewEntry
		if err := rows.Scan(&item.ID, &item.UID, &item.AppID, &item.AppSlug, &item.UserID, &item.Author, &item.Rating, &item.Title, &item.Body, &item.Deleted, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) AdminCreateReview(ctx context.Context, actor User, appSlug string, userID int64, rating int, title, body string, ip net.IP, userAgent string) (*AdminReviewEntry, error) {
	if rating < 1 || rating > 5 || userID <= 0 {
		return nil, ErrInvalidCredential
	}
	uid, err := newPublicUID()
	if err != nil {
		return nil, err
	}
	var item AdminReviewEntry
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO reviews (app_id, user_id, rating, title, body, public_uid, source)
		SELECT a.id, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6, 'web'
		FROM apps a
		WHERE a.slug = $1
		RETURNING id, public_uid, app_id, user_id, rating, COALESCE(title, ''), COALESCE(body, ''),
		          (deleted_at IS NOT NULL), created_at::text, updated_at::text
	`, strings.TrimSpace(appSlug), userID, rating, strings.TrimSpace(title), strings.TrimSpace(body), uid).Scan(
		&item.ID, &item.UID, &item.AppID, &item.UserID, &item.Rating, &item.Title, &item.Body, &item.Deleted, &item.CreatedAt, &item.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	item.AppSlug = strings.TrimSpace(appSlug)
	if user, getErr := s.UserByID(ctx, userID); getErr == nil {
		item.Author = user.Nickname
		if item.Author == "" {
			item.Author = user.Email
		}
	}
	_ = s.audit(ctx, actor.ID, "admin.reviews.create", "review", intString(item.ID), ip, userAgent)
	return &item, nil
}

func (s *Store) AdminUpdateReview(ctx context.Context, actor User, reviewID int64, rating int, title, body string, ip net.IP, userAgent string) error {
	if rating < 1 || rating > 5 {
		return ErrInvalidCredential
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE reviews
		SET rating = $2, title = NULLIF($3, ''), body = NULLIF($4, ''), deleted_at = NULL
		WHERE id = $1
	`, reviewID, rating, strings.TrimSpace(title), strings.TrimSpace(body))
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return s.audit(ctx, actor.ID, "admin.reviews.update", "review", intString(reviewID), ip, userAgent)
}

func (s *Store) AdminDeleteReview(ctx context.Context, actor User, reviewID int64, ip net.IP, userAgent string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE reviews SET deleted_at = now() WHERE id = $1`, reviewID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return s.audit(ctx, actor.ID, "admin.reviews.delete", "review", intString(reviewID), ip, userAgent)
}
