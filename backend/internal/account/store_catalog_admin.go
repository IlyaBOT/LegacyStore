package account

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strings"

	"github.com/lib/pq"

	"legacystore/backend/internal/architecture"
)

func (s *Store) ListAdminVersions(ctx context.Context, appID int64) ([]AdminVersion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, app_id, version, COALESCE(release_date::text, ''), COALESCE(changelog, ''),
		       is_recommended, created_at::text, updated_at::text
		FROM app_versions
		WHERE app_id = $1
		ORDER BY is_recommended DESC, release_date DESC NULLS LAST, id DESC
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AdminVersion, 0)
	for rows.Next() {
		var item AdminVersion
		if err := rows.Scan(&item.ID, &item.AppID, &item.Version, &item.ReleaseDate, &item.Changelog, &item.IsRecommended, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateAdminVersion(ctx context.Context, actor User, appID int64, version, releaseDate, changelog string, recommended bool, ip net.IP, userAgent string) (*AdminVersion, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return nil, ErrInvalidCredential
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM apps WHERE id = $1)`, appID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrNotFound
	}

	if recommended {
		if _, err := tx.ExecContext(ctx, `UPDATE app_versions SET is_recommended = false WHERE app_id = $1`, appID); err != nil {
			return nil, err
		}
	}

	var item AdminVersion
	err = tx.QueryRowContext(ctx, `
		INSERT INTO app_versions (app_id, version, release_date, changelog, is_recommended)
		VALUES ($1, $2, NULLIF($3, '')::date, NULLIF($4, ''), $5)
		RETURNING id, app_id, version, COALESCE(release_date::text, ''), COALESCE(changelog, ''),
		          is_recommended, created_at::text, updated_at::text
	`, appID, version, strings.TrimSpace(releaseDate), strings.TrimSpace(changelog), recommended).Scan(
		&item.ID, &item.AppID, &item.Version, &item.ReleaseDate, &item.Changelog, &item.IsRecommended, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := auditTx(ctx, tx, actor.ID, "admin.versions.create", "app_version", intString(item.ID), ip, userAgent); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) UpdateAdminVersion(ctx context.Context, actor User, versionID int64, version, releaseDate, changelog string, recommended *bool, ip net.IP, userAgent string) (*AdminVersion, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var appID int64
	if err := tx.QueryRowContext(ctx, `SELECT app_id FROM app_versions WHERE id = $1`, versionID).Scan(&appID); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}

	if recommended != nil && *recommended {
		if _, err := tx.ExecContext(ctx, `UPDATE app_versions SET is_recommended = false WHERE app_id = $1`, appID); err != nil {
			return nil, err
		}
	}

	if recommended == nil {
		var item AdminVersion
		err = tx.QueryRowContext(ctx, `
			UPDATE app_versions
			SET version = COALESCE(NULLIF($2, ''), version),
			    release_date = COALESCE(NULLIF($3, '')::date, release_date),
			    changelog = COALESCE(NULLIF($4, ''), changelog)
			WHERE id = $1
			RETURNING id, app_id, version, COALESCE(release_date::text, ''), COALESCE(changelog, ''),
			          is_recommended, created_at::text, updated_at::text
		`, versionID, strings.TrimSpace(version), strings.TrimSpace(releaseDate), strings.TrimSpace(changelog)).Scan(
			&item.ID, &item.AppID, &item.Version, &item.ReleaseDate, &item.Changelog, &item.IsRecommended, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if err := auditTx(ctx, tx, actor.ID, "admin.versions.update", "app_version", intString(versionID), ip, userAgent); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &item, nil
	}

	var item AdminVersion
	err = tx.QueryRowContext(ctx, `
		UPDATE app_versions
		SET version = COALESCE(NULLIF($2, ''), version),
		    release_date = COALESCE(NULLIF($3, '')::date, release_date),
		    changelog = COALESCE(NULLIF($4, ''), changelog),
		    is_recommended = $5
		WHERE id = $1
		RETURNING id, app_id, version, COALESCE(release_date::text, ''), COALESCE(changelog, ''),
		          is_recommended, created_at::text, updated_at::text
	`, versionID, strings.TrimSpace(version), strings.TrimSpace(releaseDate), strings.TrimSpace(changelog), *recommended).Scan(
		&item.ID, &item.AppID, &item.Version, &item.ReleaseDate, &item.Changelog, &item.IsRecommended, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if err := auditTx(ctx, tx, actor.ID, "admin.versions.update", "app_version", intString(versionID), ip, userAgent); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Store) DeleteAdminVersion(ctx context.Context, actor User, versionID int64, ip net.IP, userAgent string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM app_versions WHERE id = $1`, versionID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return s.audit(ctx, actor.ID, "admin.versions.delete", "app_version", intString(versionID), ip, userAgent)
}

func (s *Store) ListAdminArtifacts(ctx context.Context, versionID int64) ([]AdminArtifact, error) {
	rows, err := s.db.QueryContext(ctx, adminArtifactSelect()+`
		WHERE app_version_id = $1
		ORDER BY id
	`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]AdminArtifact, 0)
	for rows.Next() {
		item, err := scanAdminArtifact(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateAdminArtifact(ctx context.Context, actor User, item AdminArtifact, ip net.IP, userAgent string) (*AdminArtifact, error) {
	if strings.TrimSpace(item.MinOS) == "" {
		item.MinOS = "10.4"
	}
	architectures, err := architecture.Normalize(item.Architectures)
	if err != nil {
		return nil, ErrInvalidCredential
	}
	item.Architectures = architectures

	status := "pending"
	if HasRole(actor, "moder", "admin") {
		status = "approved"
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var created AdminArtifact
	err = tx.QueryRowContext(ctx, `
		INSERT INTO artifacts (
			app_version_id, file_name, package_type, source_type, storage_path, primary_download_url,
			torrent_url, magnet_url, size_bytes, sha256, min_os, max_supported_os, max_tested_os,
			hard_block_above_max, architectures, requires_rosetta, requires_java, install_notes, moderation_status
		)
		VALUES (
			$1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''),
			$9, NULLIF(lower($10), ''), $11, NULLIF($12, ''), NULLIF($13, ''), $14, $15,
			$16, $17, NULLIF($18, ''), $19
		)
		RETURNING id, app_version_id, file_name, package_type, source_type,
		          COALESCE(storage_path, ''), COALESCE(primary_download_url, ''), COALESCE(torrent_url, ''),
		          COALESCE(magnet_url, ''), COALESCE(size_bytes, 0), COALESCE(sha256, ''), min_os,
		          COALESCE(max_supported_os, ''), COALESCE(max_tested_os, ''), hard_block_above_max,
		          architectures, requires_rosetta, requires_java,
		          COALESCE(install_notes, ''), moderation_status, created_at::text, updated_at::text
	`, item.AppVersionID, strings.TrimSpace(item.FileName), item.PackageType, item.SourceType, strings.TrimSpace(item.StoragePath),
		strings.TrimSpace(item.PrimaryDownloadURL), strings.TrimSpace(item.TorrentURL), strings.TrimSpace(item.MagnetURL),
		item.SizeBytes, strings.TrimSpace(item.SHA256), item.MinOS, strings.TrimSpace(item.MaxSupportedOS), strings.TrimSpace(item.MaxTestedOS),
		item.HardBlockAboveMax, pq.Array(item.Architectures), item.RequiresRosetta, item.RequiresJava,
		strings.TrimSpace(item.InstallNotes), status).Scan(
		&created.ID, &created.AppVersionID, &created.FileName, &created.PackageType, &created.SourceType,
		&created.StoragePath, &created.PrimaryDownloadURL, &created.TorrentURL, &created.MagnetURL,
		&created.SizeBytes, &created.SHA256, &created.MinOS, &created.MaxSupportedOS, &created.MaxTestedOS,
		&created.HardBlockAboveMax, pq.Array(&created.Architectures),
		&created.RequiresRosetta, &created.RequiresJava, &created.InstallNotes, &created.ModerationStatus,
		&created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO moderation_queue (entity_type, entity_id, submitted_by, status)
		VALUES ('artifact', $1, $2, $3)
	`, intString(created.ID), actor.ID, status); err != nil {
		return nil, err
	}
	if err := auditTx(ctx, tx, actor.ID, "admin.artifacts.create", "artifact", intString(created.ID), ip, userAgent); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &created, nil
}

func (s *Store) UpdateAdminArtifact(ctx context.Context, actor User, artifactID int64, item AdminArtifact, ip net.IP, userAgent string) (*AdminArtifact, error) {
	architectures := item.Architectures
	if len(architectures) > 0 {
		normalized, err := architecture.Normalize(architectures)
		if err != nil {
			return nil, ErrInvalidCredential
		}
		architectures = normalized
	}

	var updated AdminArtifact
	err := s.db.QueryRowContext(ctx, `
		UPDATE artifacts
		SET file_name = COALESCE(NULLIF($2, ''), file_name),
		    package_type = COALESCE(NULLIF($3, ''), package_type),
		    source_type = COALESCE(NULLIF($4, ''), source_type),
		    storage_path = COALESCE(NULLIF($5, ''), storage_path),
		    primary_download_url = COALESCE(NULLIF($6, ''), primary_download_url),
		    torrent_url = COALESCE(NULLIF($7, ''), torrent_url),
		    magnet_url = COALESCE(NULLIF($8, ''), magnet_url),
		    size_bytes = CASE WHEN $9 >= 0 THEN $9 ELSE size_bytes END,
		    sha256 = COALESCE(NULLIF(lower($10), ''), sha256),
		    min_os = COALESCE(NULLIF($11, ''), min_os),
		    max_supported_os = COALESCE(NULLIF($12, ''), max_supported_os),
		    max_tested_os = COALESCE(NULLIF($13, ''), max_tested_os),
		    hard_block_above_max = $14,
		    architectures = CASE WHEN cardinality($15::text[]) > 0 THEN $15::text[] ELSE architectures END,
		    requires_rosetta = $16,
		    requires_java = $17,
		    install_notes = COALESCE(NULLIF($18, ''), install_notes),
		    moderation_status = COALESCE(NULLIF($19, ''), moderation_status)
		WHERE id = $1
		RETURNING id, app_version_id, file_name, package_type, source_type,
		          COALESCE(storage_path, ''), COALESCE(primary_download_url, ''), COALESCE(torrent_url, ''),
		          COALESCE(magnet_url, ''), COALESCE(size_bytes, 0), COALESCE(sha256, ''), min_os,
		          COALESCE(max_supported_os, ''), COALESCE(max_tested_os, ''), hard_block_above_max,
		          architectures, requires_rosetta, requires_java,
		          COALESCE(install_notes, ''), moderation_status, created_at::text, updated_at::text
	`, artifactID, strings.TrimSpace(item.FileName), item.PackageType, item.SourceType, strings.TrimSpace(item.StoragePath),
		strings.TrimSpace(item.PrimaryDownloadURL), strings.TrimSpace(item.TorrentURL), strings.TrimSpace(item.MagnetURL),
		item.SizeBytes, strings.TrimSpace(item.SHA256), strings.TrimSpace(item.MinOS), strings.TrimSpace(item.MaxSupportedOS),
		strings.TrimSpace(item.MaxTestedOS), item.HardBlockAboveMax, pq.Array(architectures),
		item.RequiresRosetta, item.RequiresJava, strings.TrimSpace(item.InstallNotes), strings.TrimSpace(item.ModerationStatus)).Scan(
		&updated.ID, &updated.AppVersionID, &updated.FileName, &updated.PackageType, &updated.SourceType,
		&updated.StoragePath, &updated.PrimaryDownloadURL, &updated.TorrentURL, &updated.MagnetURL,
		&updated.SizeBytes, &updated.SHA256, &updated.MinOS, &updated.MaxSupportedOS, &updated.MaxTestedOS,
		&updated.HardBlockAboveMax, pq.Array(&updated.Architectures),
		&updated.RequiresRosetta, &updated.RequiresJava, &updated.InstallNotes, &updated.ModerationStatus,
		&updated.CreatedAt, &updated.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = s.audit(ctx, actor.ID, "admin.artifacts.update", "artifact", intString(artifactID), ip, userAgent)
	return &updated, nil
}

func (s *Store) DeleteAdminArtifact(ctx context.Context, actor User, artifactID int64, ip net.IP, userAgent string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM artifacts WHERE id = $1`, artifactID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return s.audit(ctx, actor.ID, "admin.artifacts.delete", "artifact", intString(artifactID), ip, userAgent)
}

func (s *Store) ListAdminMirrors(ctx context.Context, artifactID int64) ([]AdminMirror, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, artifact_id, mirror_type, url, priority, is_active, created_at::text, updated_at::text
		FROM artifact_mirrors
		WHERE artifact_id = $1
		ORDER BY priority, id
	`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AdminMirror, 0)
	for rows.Next() {
		var item AdminMirror
		if err := rows.Scan(&item.ID, &item.ArtifactID, &item.MirrorType, &item.URL, &item.Priority, &item.IsActive, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateAdminMirror(ctx context.Context, actor User, artifactID int64, mirrorType, url string, priority int, active bool, ip net.IP, userAgent string) (*AdminMirror, error) {
	var item AdminMirror
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO artifact_mirrors (artifact_id, mirror_type, url, priority, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, artifact_id, mirror_type, url, priority, is_active, created_at::text, updated_at::text
	`, artifactID, mirrorType, strings.TrimSpace(url), priority, active).Scan(
		&item.ID, &item.ArtifactID, &item.MirrorType, &item.URL, &item.Priority, &item.IsActive, &item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = s.audit(ctx, actor.ID, "admin.mirrors.create", "artifact_mirror", intString(item.ID), ip, userAgent)
	return &item, nil
}

func (s *Store) UpdateAdminMirror(ctx context.Context, actor User, mirrorID int64, mirrorType, url string, priority int, active *bool, ip net.IP, userAgent string) (*AdminMirror, error) {
	var item AdminMirror
	if active == nil {
		err := s.db.QueryRowContext(ctx, `
			UPDATE artifact_mirrors
			SET mirror_type = COALESCE(NULLIF($2, ''), mirror_type),
			    url = COALESCE(NULLIF($3, ''), url),
			    priority = CASE WHEN $4 >= 0 THEN $4 ELSE priority END
			WHERE id = $1
			RETURNING id, artifact_id, mirror_type, url, priority, is_active, created_at::text, updated_at::text
		`, mirrorID, mirrorType, strings.TrimSpace(url), priority).Scan(
			&item.ID, &item.ArtifactID, &item.MirrorType, &item.URL, &item.Priority, &item.IsActive, &item.CreatedAt, &item.UpdatedAt,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
	} else {
		err := s.db.QueryRowContext(ctx, `
			UPDATE artifact_mirrors
			SET mirror_type = COALESCE(NULLIF($2, ''), mirror_type),
			    url = COALESCE(NULLIF($3, ''), url),
			    priority = CASE WHEN $4 >= 0 THEN $4 ELSE priority END,
			    is_active = $5
			WHERE id = $1
			RETURNING id, artifact_id, mirror_type, url, priority, is_active, created_at::text, updated_at::text
		`, mirrorID, mirrorType, strings.TrimSpace(url), priority, *active).Scan(
			&item.ID, &item.ArtifactID, &item.MirrorType, &item.URL, &item.Priority, &item.IsActive, &item.CreatedAt, &item.UpdatedAt,
		)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
	}
	_ = s.audit(ctx, actor.ID, "admin.mirrors.update", "artifact_mirror", intString(mirrorID), ip, userAgent)
	return &item, nil
}

func (s *Store) DeleteAdminMirror(ctx context.Context, actor User, mirrorID int64, ip net.IP, userAgent string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM artifact_mirrors WHERE id = $1`, mirrorID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return s.audit(ctx, actor.ID, "admin.mirrors.delete", "artifact_mirror", intString(mirrorID), ip, userAgent)
}

func (s *Store) CreateAdminIcon(ctx context.Context, actor User, item AdminIcon, ip net.IP, userAgent string) (*AdminIcon, error) {
	if item.MinOS == "" {
		item.MinOS = "10.4"
	}
	if item.MaxOS == "" {
		item.MaxOS = "15"
	}
	var created AdminIcon
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO icons (app_id, app_version_id, image_url, min_os, max_os, width, height)
		VALUES ($1, NULLIF($2, 0), $3, $4, $5, NULLIF($6, 0), NULLIF($7, 0))
		RETURNING id, app_id, COALESCE(app_version_id, 0), image_url, min_os, max_os,
		          COALESCE(width, 0), COALESCE(height, 0), created_at::text, updated_at::text
	`, item.AppID, item.AppVersionID, strings.TrimSpace(item.ImageURL), item.MinOS, item.MaxOS, item.Width, item.Height).Scan(
		&created.ID, &created.AppID, &created.AppVersionID, &created.ImageURL, &created.MinOS, &created.MaxOS,
		&created.Width, &created.Height, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = s.audit(ctx, actor.ID, "admin.icons.create", "icon", intString(created.ID), ip, userAgent)
	return &created, nil
}

func (s *Store) DeleteAdminIcon(ctx context.Context, actor User, iconID int64, ip net.IP, userAgent string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var imageAssetID sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		DELETE FROM icons
		WHERE id = $1
		RETURNING image_asset_id
	`, iconID).Scan(&imageAssetID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if imageAssetID.Valid {
		_, _ = tx.ExecContext(ctx, `DELETE FROM image_assets WHERE id = $1 AND kind = 'icon'`, imageAssetID.Int64)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.audit(ctx, actor.ID, "admin.icons.delete", "icon", intString(iconID), ip, userAgent)
}

func (s *Store) CreateAdminScreenshot(ctx context.Context, actor User, item AdminScreenshot, ip net.IP, userAgent string) (*AdminScreenshot, error) {
	if item.MinOS == "" {
		item.MinOS = "10.4"
	}
	if item.MaxOS == "" {
		item.MaxOS = "15"
	}
	var created AdminScreenshot
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO screenshots (app_id, app_version_id, image_url, min_os, max_os, caption, sort_order)
		VALUES ($1, NULLIF($2, 0), $3, $4, $5, NULLIF($6, ''), $7)
		RETURNING id, app_id, COALESCE(app_version_id, 0), image_url, min_os, max_os,
		          COALESCE(caption, ''), sort_order, created_at::text, updated_at::text
	`, item.AppID, item.AppVersionID, strings.TrimSpace(item.ImageURL), item.MinOS, item.MaxOS,
		strings.TrimSpace(item.Caption), item.SortOrder).Scan(
		&created.ID, &created.AppID, &created.AppVersionID, &created.ImageURL, &created.MinOS, &created.MaxOS,
		&created.Caption, &created.SortOrder, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	_ = s.audit(ctx, actor.ID, "admin.screenshots.create", "screenshot", intString(created.ID), ip, userAgent)
	return &created, nil
}

func (s *Store) UpdateAdminScreenshot(ctx context.Context, actor User, screenshotID int64, item AdminScreenshot, ip net.IP, userAgent string) (*AdminScreenshot, error) {
	var updated AdminScreenshot
	err := s.db.QueryRowContext(ctx, `
		UPDATE screenshots
		SET app_version_id = CASE WHEN $2 < 0 THEN app_version_id ELSE NULLIF($2, 0) END,
		    image_url = COALESCE(NULLIF($3, ''), image_url),
		    min_os = COALESCE(NULLIF($4, ''), min_os),
		    max_os = COALESCE(NULLIF($5, ''), max_os),
		    caption = COALESCE(NULLIF($6, ''), caption),
		    sort_order = CASE WHEN $7 < 0 THEN sort_order ELSE $7 END
		WHERE id = $1
		RETURNING id, app_id, COALESCE(app_version_id, 0), image_url, min_os, max_os,
		          COALESCE(caption, ''), sort_order, created_at::text, updated_at::text
	`, screenshotID, item.AppVersionID, strings.TrimSpace(item.ImageURL), strings.TrimSpace(item.MinOS), strings.TrimSpace(item.MaxOS),
		strings.TrimSpace(item.Caption), item.SortOrder).Scan(
		&updated.ID, &updated.AppID, &updated.AppVersionID, &updated.ImageURL, &updated.MinOS, &updated.MaxOS,
		&updated.Caption, &updated.SortOrder, &updated.CreatedAt, &updated.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = s.audit(ctx, actor.ID, "admin.screenshots.update", "screenshot", intString(screenshotID), ip, userAgent)
	return &updated, nil
}

func (s *Store) DeleteAdminScreenshot(ctx context.Context, actor User, screenshotID int64, ip net.IP, userAgent string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var imageAssetID sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		DELETE FROM screenshots
		WHERE id = $1
		RETURNING image_asset_id
	`, screenshotID).Scan(&imageAssetID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if imageAssetID.Valid {
		_, _ = tx.ExecContext(ctx, `DELETE FROM image_assets WHERE id = $1 AND kind = 'screenshot'`, imageAssetID.Int64)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.audit(ctx, actor.ID, "admin.screenshots.delete", "screenshot", intString(screenshotID), ip, userAgent)
}

func adminArtifactSelect() string {
	return `
		SELECT id, app_version_id, file_name, package_type, source_type,
		       COALESCE(storage_path, ''), COALESCE(primary_download_url, ''), COALESCE(torrent_url, ''),
		       COALESCE(magnet_url, ''), COALESCE(size_bytes, 0), COALESCE(sha256, ''), min_os,
		       COALESCE(max_supported_os, ''), COALESCE(max_tested_os, ''), hard_block_above_max,
		       architectures, requires_rosetta, requires_java,
		       COALESCE(install_notes, ''), moderation_status, created_at::text, updated_at::text
		FROM artifacts
	`
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAdminArtifact(row rowScanner) (AdminArtifact, error) {
	var item AdminArtifact
	err := row.Scan(
		&item.ID, &item.AppVersionID, &item.FileName, &item.PackageType, &item.SourceType,
		&item.StoragePath, &item.PrimaryDownloadURL, &item.TorrentURL, &item.MagnetURL,
		&item.SizeBytes, &item.SHA256, &item.MinOS, &item.MaxSupportedOS, &item.MaxTestedOS,
		&item.HardBlockAboveMax, pq.Array(&item.Architectures),
		&item.RequiresRosetta, &item.RequiresJava, &item.InstallNotes, &item.ModerationStatus, &item.CreatedAt, &item.UpdatedAt,
	)
	return item, err
}
