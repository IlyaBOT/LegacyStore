package account

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
)

type ImageAsset struct {
	ID               int64  `json:"-"`
	UID              string `json:"uid"`
	URL              string `json:"url"`
	Kind             string `json:"kind"`
	StoragePath      string `json:"-"`
	MIMEType         string `json:"mime_type"`
	FileExt          string `json:"file_ext"`
	OriginalFilename string `json:"original_filename,omitempty"`
	SizeBytes        int64  `json:"size_bytes"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	SHA256           string `json:"sha256"`
	CreatedBy        int64  `json:"created_by,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
}

type ImageAssetInput struct {
	Kind             string
	StoragePath      string
	MIMEType         string
	FileExt          string
	OriginalFilename string
	SizeBytes        int64
	Width            int
	Height           int
	SHA256           string
	CreatedBy        int64
}

func (s *Store) SetAvatarImage(ctx context.Context, user User, input ImageAssetInput, ip net.IP, userAgent string) (*ImageAsset, error) {
	input.Kind = "avatar"
	input.CreatedBy = user.ID

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var oldAssetID sql.NullInt64
	if err := tx.QueryRowContext(ctx, `SELECT avatar_image_id FROM users WHERE id = $1 FOR UPDATE`, user.ID).Scan(&oldAssetID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	asset, err := insertImageAsset(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE users
		SET avatar_image_id = $2,
		    avatar_url = $3
		WHERE id = $1
	`, user.ID, asset.ID, asset.URL)
	if err != nil {
		return nil, err
	}
	if oldAssetID.Valid && oldAssetID.Int64 != asset.ID {
		_, _ = tx.ExecContext(ctx, `DELETE FROM image_assets WHERE id = $1 AND kind = 'avatar'`, oldAssetID.Int64)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	_ = s.audit(ctx, user.ID, "profile.avatar.upload", "image", asset.UID, ip, userAgent)
	return asset, nil
}

func (s *Store) DeleteAvatarImage(ctx context.Context, user User, ip net.IP, userAgent string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var assetID sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		UPDATE users
		SET avatar_image_id = NULL, avatar_url = NULL
		WHERE id = $1
		RETURNING avatar_image_id
	`, user.ID).Scan(&assetID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	// RETURNING sees the new NULL value. Delete any orphaned avatar rows for this creator.
	_, _ = tx.ExecContext(ctx, `
		DELETE FROM image_assets ia
		WHERE ia.kind = 'avatar' AND ia.created_by = $1
		  AND NOT EXISTS (SELECT 1 FROM users u WHERE u.avatar_image_id = ia.id)
	`, user.ID)
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.audit(ctx, user.ID, "profile.avatar.delete", "user", intString(user.ID), ip, userAgent)
}

func (s *Store) AddReviewImages(ctx context.Context, user User, reviewRef string, inputs []ImageAssetInput, ip net.IP, userAgent string) ([]ImageAsset, error) {
	if len(inputs) == 0 || len(inputs) > 3 {
		return nil, ErrInvalidCredential
	}
	reviewID, _, err := s.reviewIDByRef(ctx, reviewRef)
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var existing int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM review_images WHERE review_id = $1`, reviewID).Scan(&existing); err != nil {
		return nil, err
	}
	if existing+len(inputs) > 3 {
		return nil, ErrInvalidCredential
	}

	created := make([]ImageAsset, 0, len(inputs))
	for i, input := range inputs {
		input.Kind = "review"
		input.CreatedBy = user.ID
		asset, err := insertImageAsset(ctx, tx, input)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO review_images (review_id, image_id, sort_order)
			VALUES ($1, $2, $3)
		`, reviewID, asset.ID, existing+i); err != nil {
			return nil, err
		}
		created = append(created, *asset)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	_ = s.audit(ctx, user.ID, "reviews.images.upload", "review", reviewRef, ip, userAgent)
	return created, nil
}

func (s *Store) DeleteReviewImage(ctx context.Context, user User, reviewRef, imageUID string, ip net.IP, userAgent string) error {
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var imageID int64
	err = tx.QueryRowContext(ctx, `
		DELETE FROM review_images ri
		USING image_assets ia
		WHERE ri.review_id = $1
		  AND ri.image_id = ia.id
		  AND ia.public_uid = $2
		RETURNING ri.image_id
	`, reviewID, strings.TrimSpace(imageUID)).Scan(&imageID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, _ = tx.ExecContext(ctx, `DELETE FROM image_assets WHERE id = $1 AND kind = 'review'`, imageID)
	if _, err := tx.ExecContext(ctx, `
		WITH ordered AS (
			SELECT image_id, row_number() OVER (ORDER BY sort_order, image_id) - 1 AS new_order
			FROM review_images
			WHERE review_id = $1
		)
		UPDATE review_images ri
		SET sort_order = ordered.new_order
		FROM ordered
		WHERE ri.review_id = $1 AND ri.image_id = ordered.image_id
	`, reviewID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.audit(ctx, user.ID, "reviews.images.delete", "review", reviewRef, ip, userAgent)
}

func (s *Store) CreateUploadedIcon(ctx context.Context, actor User, appID, appVersionID int64, minOS, maxOS string, input ImageAssetInput, ip net.IP, userAgent string) (*AdminIcon, *ImageAsset, error) {
	if minOS == "" {
		minOS = "10.4"
	}
	if maxOS == "" {
		maxOS = "15"
	}
	input.Kind = "icon"
	input.CreatedBy = actor.ID

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	if err := validateVersionBelongsToApp(ctx, tx, appID, appVersionID); err != nil {
		return nil, nil, err
	}
	asset, err := insertImageAsset(ctx, tx, input)
	if err != nil {
		return nil, nil, err
	}
	var created AdminIcon
	err = tx.QueryRowContext(ctx, `
		INSERT INTO icons (app_id, app_version_id, image_url, image_asset_id, min_os, max_os, width, height)
		VALUES ($1, NULLIF($2, 0), $3, $4, $5, $6, $7, $8)
		RETURNING id, app_id, COALESCE(app_version_id, 0), image_url, min_os, max_os,
		          COALESCE(width, 0), COALESCE(height, 0), created_at::text, updated_at::text
	`, appID, appVersionID, asset.URL, asset.ID, minOS, maxOS, asset.Width, asset.Height).Scan(
		&created.ID, &created.AppID, &created.AppVersionID, &created.ImageURL, &created.MinOS, &created.MaxOS,
		&created.Width, &created.Height, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	_ = s.audit(ctx, actor.ID, "admin.icons.upload", "icon", intString(created.ID), ip, userAgent)
	return &created, asset, nil
}

func (s *Store) CreateUploadedScreenshots(ctx context.Context, actor User, appID, appVersionID int64, minOS, maxOS, caption string, inputs []ImageAssetInput, ip net.IP, userAgent string) ([]AdminScreenshot, []ImageAsset, error) {
	if len(inputs) == 0 || len(inputs) > 3 {
		return nil, nil, ErrInvalidCredential
	}
	if minOS == "" {
		minOS = "10.4"
	}
	if maxOS == "" {
		maxOS = "15"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	if err := validateVersionBelongsToApp(ctx, tx, appID, appVersionID); err != nil {
		return nil, nil, err
	}

	var existing int
	if appVersionID > 0 {
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM screenshots
			WHERE app_id = $1 AND app_version_id = $2
		`, appID, appVersionID).Scan(&existing)
	} else {
		err = tx.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM screenshots
			WHERE app_id = $1 AND app_version_id IS NULL
		`, appID).Scan(&existing)
	}
	if err != nil {
		return nil, nil, err
	}
	if existing+len(inputs) > 3 {
		return nil, nil, ErrInvalidCredential
	}

	shots := make([]AdminScreenshot, 0, len(inputs))
	assets := make([]ImageAsset, 0, len(inputs))
	for i, input := range inputs {
		input.Kind = "screenshot"
		input.CreatedBy = actor.ID
		asset, err := insertImageAsset(ctx, tx, input)
		if err != nil {
			return nil, nil, err
		}
		var shot AdminScreenshot
		err = tx.QueryRowContext(ctx, `
			INSERT INTO screenshots (
				app_id, app_version_id, image_url, image_asset_id,
				min_os, max_os, caption, sort_order
			)
			VALUES ($1, NULLIF($2, 0), $3, $4, $5, $6, NULLIF($7, ''), $8)
			RETURNING id, app_id, COALESCE(app_version_id, 0), image_url, min_os, max_os,
			          COALESCE(caption, ''), sort_order, created_at::text, updated_at::text
		`, appID, appVersionID, asset.URL, asset.ID, minOS, maxOS, strings.TrimSpace(caption), existing+i).Scan(
			&shot.ID, &shot.AppID, &shot.AppVersionID, &shot.ImageURL, &shot.MinOS, &shot.MaxOS,
			&shot.Caption, &shot.SortOrder, &shot.CreatedAt, &shot.UpdatedAt,
		)
		if err != nil {
			return nil, nil, err
		}
		shots = append(shots, shot)
		assets = append(assets, *asset)
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	_ = s.audit(ctx, actor.ID, "admin.screenshots.upload", "app", intString(appID), ip, userAgent)
	return shots, assets, nil
}

type dbQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertImageAsset(ctx context.Context, tx *sql.Tx, input ImageAssetInput) (*ImageAsset, error) {
	if input.SizeBytes <= 0 || input.SizeBytes > 1<<20 || input.Width <= 0 || input.Height <= 0 ||
		input.Width > 2048 || input.Height > 2048 || strings.TrimSpace(input.StoragePath) == "" ||
		len(strings.TrimSpace(input.SHA256)) != 64 {
		return nil, ErrInvalidCredential
	}
	switch input.Kind {
	case "avatar", "review", "screenshot", "icon":
	default:
		return nil, ErrInvalidCredential
	}
	switch input.MIMEType {
	case "image/jpeg", "image/png", "image/svg+xml":
	default:
		return nil, ErrInvalidCredential
	}

	for attempts := 0; attempts < 3; attempts++ {
		uid, err := newPublicUID()
		if err != nil {
			return nil, err
		}
		var asset ImageAsset
		err = tx.QueryRowContext(ctx, `
			INSERT INTO image_assets (
				public_uid, kind, storage_path, mime_type, file_ext,
				original_filename, size_bytes, width, height, sha256, created_by
			)
			VALUES (
				$1, $2, $3, $4, $5,
				NULLIF($6, ''), $7, $8, $9, $10, NULLIF($11, 0)
			)
			RETURNING id, public_uid, kind, storage_path, mime_type, file_ext,
			          COALESCE(original_filename, ''), size_bytes, width, height, sha256,
			          COALESCE(created_by, 0), created_at::text
		`,
			uid, input.Kind, input.StoragePath, input.MIMEType, input.FileExt,
			strings.TrimSpace(input.OriginalFilename), input.SizeBytes, input.Width, input.Height,
			strings.ToLower(strings.TrimSpace(input.SHA256)), input.CreatedBy,
		).Scan(
			&asset.ID, &asset.UID, &asset.Kind, &asset.StoragePath, &asset.MIMEType, &asset.FileExt,
			&asset.OriginalFilename, &asset.SizeBytes, &asset.Width, &asset.Height, &asset.SHA256,
			&asset.CreatedBy, &asset.CreatedAt,
		)
		if err == nil {
			asset.URL = imageAssetURL(asset.UID)
			return &asset, nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "image_assets_public_uid") &&
			!strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			return nil, err
		}
	}
	return nil, fmt.Errorf("could not allocate unique image uid")
}

func validateVersionBelongsToApp(ctx context.Context, tx *sql.Tx, appID, versionID int64) error {
	var exists bool
	if versionID > 0 {
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM app_versions WHERE id = $1 AND app_id = $2
			)
		`, versionID, appID).Scan(&exists); err != nil {
			return err
		}
	} else {
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM apps WHERE id = $1)`, appID).Scan(&exists); err != nil {
			return err
		}
	}
	if !exists {
		return ErrNotFound
	}
	return nil
}

func imageAssetURL(uid string) string {
	return "/api/v1/images/" + strings.TrimSpace(uid)
}
