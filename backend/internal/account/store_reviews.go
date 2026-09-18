package account

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (s *Store) CreateReview(ctx context.Context, user User, appSlug string, rating int, title, body string, meta ReviewContext, ip net.IP, userAgent string) (*Review, error) {
	title, body, meta, err := normalizeReviewInput(rating, title, body, meta)
	if err != nil {
		return nil, err
	}
	appID, err := s.appIDBySlug(ctx, appSlug)
	if err != nil {
		return nil, err
	}
	versionID, err := s.reviewVersionID(ctx, appID, meta.AppVersion)
	if err != nil {
		return nil, err
	}
	uid, err := newPublicUID()
	if err != nil {
		return nil, err
	}

	var review Review
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO reviews (
			public_uid, app_id, user_id, rating, title, body,
			app_version_id, app_version, os_version, os_arch,
			device_model, client_version, source
		)
		VALUES (
			$1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''),
			$7, NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''),
			NULLIF($11, ''), NULLIF($12, ''), $13
		)
		ON CONFLICT (app_id, user_id) WHERE deleted_at IS NULL
		DO UPDATE SET
			rating = EXCLUDED.rating,
			title = EXCLUDED.title,
			body = EXCLUDED.body,
			app_version_id = EXCLUDED.app_version_id,
			app_version = EXCLUDED.app_version,
			os_version = EXCLUDED.os_version,
			os_arch = EXCLUDED.os_arch,
			device_model = EXCLUDED.device_model,
			client_version = EXCLUDED.client_version,
			source = EXCLUDED.source,
			updated_at = now()
		RETURNING
			id, public_uid, rating, COALESCE(title, ''), COALESCE(body, ''),
			COALESCE(app_version, ''), COALESCE(os_version, ''), COALESCE(os_arch, ''),
			COALESCE(device_model, ''), COALESCE(client_version, ''), source,
			created_at::text, updated_at::text
	`,
		uid, appID, user.ID, rating, title, body,
		versionID, meta.AppVersion, meta.OSVersion, meta.OSArch,
		meta.DeviceModel, meta.ClientVersion, meta.Source,
	).Scan(
		&review.ID, &review.UID, &review.Rating, &review.Title, &review.Body,
		&review.AppVersion, &review.OSVersion, &review.OSArch,
		&review.DeviceModel, &review.ClientVersion, &review.Source,
		&review.CreatedAt, &review.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	review.AppSlug = appSlug
	review.UserID = user.ID
	review.Author = displayName(user)
	review.AvatarURL = user.AvatarURL
	_ = s.audit(ctx, user.ID, "reviews.write", "review", review.UID, ip, userAgent)
	return &review, nil
}

func (s *Store) UpdateReview(ctx context.Context, user User, reviewRef string, rating int, title, body string, meta ReviewContext, ip net.IP, userAgent string) (*Review, error) {
	title, body, meta, err := normalizeReviewInput(rating, title, body, meta)
	if err != nil {
		return nil, err
	}
	reviewID, appID, err := s.reviewIDByRef(ctx, reviewRef)
	if err != nil {
		return nil, err
	}
	allowed, err := s.canChangeReview(ctx, user, reviewID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	versionID, err := s.reviewVersionID(ctx, appID, meta.AppVersion)
	if err != nil {
		return nil, err
	}

	var review Review
	err = s.db.QueryRowContext(ctx, `
		UPDATE reviews
		SET rating = $2,
		    title = NULLIF($3, ''),
		    body = NULLIF($4, ''),
		    app_version_id = $5,
		    app_version = NULLIF($6, ''),
		    os_version = NULLIF($7, ''),
		    os_arch = NULLIF($8, ''),
		    device_model = NULLIF($9, ''),
		    client_version = NULLIF($10, ''),
		    source = $11
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING
			id, public_uid, rating, COALESCE(title, ''), COALESCE(body, ''),
			COALESCE(app_version, ''), COALESCE(os_version, ''), COALESCE(os_arch, ''),
			COALESCE(device_model, ''), COALESCE(client_version, ''), source,
			created_at::text, updated_at::text
	`,
		reviewID, rating, title, body, versionID, meta.AppVersion, meta.OSVersion,
		meta.OSArch, meta.DeviceModel, meta.ClientVersion, meta.Source,
	).Scan(
		&review.ID, &review.UID, &review.Rating, &review.Title, &review.Body,
		&review.AppVersion, &review.OSVersion, &review.OSArch,
		&review.DeviceModel, &review.ClientVersion, &review.Source,
		&review.CreatedAt, &review.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	review.UserID = user.ID
	review.Author = displayName(user)
	review.AvatarURL = user.AvatarURL
	_ = s.audit(ctx, user.ID, "reviews.update", "review", review.UID, ip, userAgent)
	return &review, nil
}

func (s *Store) DeleteReview(ctx context.Context, user User, reviewRef string, ip net.IP, userAgent string) error {
	reviewID, _, err := s.reviewIDByRef(ctx, reviewRef)
	if err != nil {
		return err
	}
	allowed, err := s.canChangeReview(ctx, user, reviewID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	var uid string
	err = s.db.QueryRowContext(ctx, `
		UPDATE reviews
		SET deleted_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING public_uid
	`, reviewID).Scan(&uid)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return s.audit(ctx, user.ID, "reviews.delete", "review", uid, ip, userAgent)
}

func (s *Store) LikeReview(ctx context.Context, user User, reviewRef string) error {
	reviewID, _, err := s.reviewIDByRef(ctx, reviewRef)
	if err != nil {
		return err
	}
	if !s.reviewExists(ctx, reviewID) {
		return ErrNotFound
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO review_likes (review_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, reviewID, user.ID)
	return err
}

func (s *Store) UnlikeReview(ctx context.Context, userID int64, reviewRef string) error {
	reviewID, _, err := s.reviewIDByRef(ctx, reviewRef)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM review_likes WHERE review_id = $1 AND user_id = $2`, reviewID, userID)
	return err
}

func (s *Store) CreateReviewReply(ctx context.Context, user User, reviewRef, body string, ip net.IP, userAgent string) (*ReviewReply, error) {
	body = strings.TrimSpace(body)
	if body == "" || utf8.RuneCountInString(body) > 1000 {
		return nil, ErrInvalidCredential
	}
	reviewID, _, err := s.reviewIDByRef(ctx, reviewRef)
	if err != nil {
		return nil, err
	}
	if !s.reviewExists(ctx, reviewID) {
		return nil, ErrNotFound
	}
	var reply ReviewReply
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO review_replies (review_id, user_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, review_id, user_id, body, created_at::text
	`, reviewID, user.ID, body).Scan(&reply.ID, &reply.ReviewID, &reply.UserID, &reply.Body, &reply.CreatedAt)
	if err != nil {
		return nil, err
	}
	reply.Author = displayName(user)
	_ = s.audit(ctx, user.ID, "reviews.reply", "review", reviewRef, ip, userAgent)
	return &reply, nil
}

func normalizeReviewInput(rating int, title, body string, meta ReviewContext) (string, string, ReviewContext, error) {
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)
	meta.AppVersion = strings.TrimSpace(meta.AppVersion)
	meta.OSVersion = truncate(strings.TrimSpace(meta.OSVersion), 32)
	meta.OSArch = strings.TrimSpace(meta.OSArch)
	meta.DeviceModel = truncate(strings.TrimSpace(meta.DeviceModel), 160)
	meta.ClientVersion = truncate(strings.TrimSpace(meta.ClientVersion), 64)
	meta.Source = strings.TrimSpace(meta.Source)

	if rating < 1 || rating > 5 || body == "" || utf8.RuneCountInString(body) > 300 || utf8.RuneCountInString(title) > 120 {
		return "", "", meta, ErrInvalidCredential
	}
	if len(meta.AppVersion) > 64 {
		return "", "", meta, ErrInvalidCredential
	}
	if meta.OSArch != "" && meta.OSArch != "i386" && meta.OSArch != "x86_64" {
		return "", "", meta, ErrInvalidCredential
	}
	switch meta.Source {
	case "web", "legacy", "native":
	default:
		meta.Source = "web"
	}
	return title, body, meta, nil
}

func (s *Store) reviewVersionID(ctx context.Context, appID int64, version string) (any, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, nil
	}
	var versionID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM app_versions
		WHERE app_id = $1 AND version = $2
	`, appID, version).Scan(&versionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrInvalidCredential
	}
	if err != nil {
		return nil, err
	}
	return versionID, nil
}

func (s *Store) reviewIDByRef(ctx context.Context, reviewRef string) (int64, int64, error) {
	reviewRef = strings.TrimSpace(reviewRef)
	if reviewRef == "" {
		return 0, 0, ErrNotFound
	}

	var reviewID, appID int64
	if numericID, err := strconv.ParseInt(reviewRef, 10, 64); err == nil {
		err = s.db.QueryRowContext(ctx, `
			SELECT id, app_id
			FROM reviews
			WHERE id = $1 AND deleted_at IS NULL
		`, numericID).Scan(&reviewID, &appID)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, ErrNotFound
		}
		return reviewID, appID, err
	}

	err := s.db.QueryRowContext(ctx, `
		SELECT id, app_id
		FROM reviews
		WHERE public_uid = $1 AND deleted_at IS NULL
	`, reviewRef).Scan(&reviewID, &appID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, 0, ErrNotFound
	}
	return reviewID, appID, err
}

func (s *Store) appIDBySlug(ctx context.Context, slug string) (int64, error) {
	var appID int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM apps WHERE slug = $1 AND moderation_status = 'approved'`, slug).Scan(&appID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return appID, err
}

func (s *Store) reviewExists(ctx context.Context, reviewID int64) bool {
	var exists bool
	_ = s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM reviews WHERE id = $1 AND deleted_at IS NULL)`, reviewID).Scan(&exists)
	return exists
}

func (s *Store) canChangeReview(ctx context.Context, user User, reviewID int64) (bool, error) {
	if HasRole(user, "admin", "moder") {
		return true, nil
	}
	var ownerID int64
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM reviews WHERE id = $1 AND deleted_at IS NULL`, reviewID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return ownerID == user.ID, nil
}

func displayName(user User) string {
	if strings.TrimSpace(user.Nickname) != "" {
		return user.Nickname
	}
	return user.Email
}
