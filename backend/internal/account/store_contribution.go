package account

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/lib/pq"

	"legacystore/backend/internal/architecture"
)

func (s *Store) CreateStagedUpload(ctx context.Context, actor User, input StageUploadInput, ip net.IP, userAgent string) (*StagedUpload, error) {
	if input.AppID <= 0 || input.SubmittedBy != actor.ID ||
		strings.TrimSpace(input.OriginalFilename) == "" ||
		strings.TrimSpace(input.StoragePath) == "" ||
		len(strings.TrimSpace(input.SHA256)) != 64 ||
		input.SizeBytes <= 0 {
		return nil, ErrInvalidCredential
	}
	switch input.PackageType {
	case "app", "dmg", "zip", "pkg", "iso", "other":
	default:
		return nil, ErrInvalidCredential
	}
	if len(input.DetectedArchitectures) > 0 {
		normalized, err := architecture.Normalize(input.DetectedArchitectures)
		if err != nil {
			return nil, ErrInvalidCredential
		}
		input.DetectedArchitectures = normalized
	}
	if len(input.Warnings) == 0 {
		input.Warnings = json.RawMessage("[]")
	}
	if !json.Valid(input.Warnings) {
		return nil, ErrInvalidCredential
	}

	var appExists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM apps WHERE id = $1)`, input.AppID).Scan(&appExists); err != nil {
		return nil, err
	}
	if !appExists {
		return nil, ErrNotFound
	}

	for attempt := 0; attempt < 3; attempt++ {
		uid, err := newPublicUID()
		if err != nil {
			return nil, err
		}
		row := s.db.QueryRowContext(ctx, `
			INSERT INTO staged_uploads (
				public_uid, app_id, submitted_by, original_filename, storage_path,
				sha256, size_bytes, package_type, detected_name, detected_bundle_id,
				detected_version, detected_category_slug, detected_min_os,
				detected_architectures, warnings
			)
			VALUES (
				$1, $2, $3, $4, $5, lower($6), $7, $8,
				NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''),
				NULLIF($13, ''), $14, $15::jsonb
			)
			RETURNING public_uid, app_id, submitted_by, original_filename, storage_path,
			          sha256, size_bytes, package_type, COALESCE(detected_name, ''),
			          COALESCE(detected_bundle_id, ''), COALESCE(detected_version, ''),
			          COALESCE(detected_category_slug, ''), COALESCE(detected_min_os, ''),
			          detected_architectures, warnings::text, status, expires_at::text,
			          COALESCE(committed_at::text, ''), created_at::text, updated_at::text
		`,
			uid, input.AppID, actor.ID, strings.TrimSpace(input.OriginalFilename), strings.TrimSpace(input.StoragePath),
			strings.ToLower(strings.TrimSpace(input.SHA256)), input.SizeBytes, input.PackageType,
			strings.TrimSpace(input.DetectedName), strings.TrimSpace(input.DetectedBundleID),
			strings.TrimSpace(input.DetectedVersion), strings.TrimSpace(input.DetectedCategorySlug),
			strings.TrimSpace(input.DetectedMinOS), pq.Array(input.DetectedArchitectures), string(input.Warnings),
		)
		staged, scanErr := scanStagedUpload(row)
		if scanErr == nil {
			_ = s.audit(ctx, actor.ID, "contributions.stage", "staged_upload", staged.UID, ip, userAgent)
			return &staged, nil
		}
		if !strings.Contains(strings.ToLower(scanErr.Error()), "duplicate") {
			return nil, scanErr
		}
	}
	return nil, errors.New("could not allocate staged upload uid")
}

func (s *Store) GetStagedUpload(ctx context.Context, actor User, uid string) (*StagedUpload, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT public_uid, app_id, submitted_by, original_filename, storage_path,
		       sha256, size_bytes, package_type, COALESCE(detected_name, ''),
		       COALESCE(detected_bundle_id, ''), COALESCE(detected_version, ''),
		       COALESCE(detected_category_slug, ''), COALESCE(detected_min_os, ''),
		       detected_architectures, warnings::text, status, expires_at::text,
		       COALESCE(committed_at::text, ''), created_at::text, updated_at::text
		FROM staged_uploads
		WHERE public_uid = $1
	`, strings.TrimSpace(uid))
	staged, err := scanStagedUpload(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if staged.SubmittedBy != actor.ID && !HasRole(actor, "moder", "admin") {
		return nil, ErrForbidden
	}
	return &staged, nil
}

func (s *Store) CommitStagedUpload(ctx context.Context, actor User, uid string, submission ReleaseSubmission, ip net.IP, userAgent string) (*ReleaseSubmissionResult, error) {
	submission.Version = strings.TrimSpace(submission.Version)
	submission.MinOS = strings.TrimSpace(submission.MinOS)
	if submission.Version == "" || submission.MinOS == "" {
		return nil, ErrInvalidCredential
	}
	architectures, err := architecture.Normalize(submission.Architectures)
	if err != nil {
		return nil, ErrInvalidCredential
	}
	submission.Architectures = architectures

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, `
		SELECT public_uid, app_id, submitted_by, original_filename, storage_path,
		       sha256, size_bytes, package_type, COALESCE(detected_name, ''),
		       COALESCE(detected_bundle_id, ''), COALESCE(detected_version, ''),
		       COALESCE(detected_category_slug, ''), COALESCE(detected_min_os, ''),
		       detected_architectures, warnings::text, status, expires_at::text,
		       COALESCE(committed_at::text, ''), created_at::text, updated_at::text
		FROM staged_uploads
		WHERE public_uid = $1
		FOR UPDATE
	`, strings.TrimSpace(uid))
	staged, err := scanStagedUpload(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if staged.SubmittedBy != actor.ID && !HasRole(actor, "moder", "admin") {
		return nil, ErrForbidden
	}
	if staged.Status != "staged" {
		return nil, ErrInvalidCredential
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, staged.ExpiresAt)
	if err == nil && time.Now().After(expiresAt) {
		return nil, ErrInvalidCredential
	}

	var versionID int64
	err = tx.QueryRowContext(ctx, `
		WITH inserted AS (
			INSERT INTO app_versions (app_id, version, release_date, changelog, is_recommended)
			VALUES ($1, $2, NULLIF($3, '')::date, NULLIF($4, ''), $5)
			ON CONFLICT (app_id, version) DO NOTHING
			RETURNING id
		)
		SELECT id FROM inserted
		UNION ALL
		SELECT id FROM app_versions WHERE app_id = $1 AND version = $2
		LIMIT 1
	`, staged.AppID, submission.Version, strings.TrimSpace(submission.ReleaseDate), strings.TrimSpace(submission.Changelog), submission.IsRecommended).Scan(&versionID)
	if err != nil {
		return nil, err
	}

	status := "pending"
	if HasRole(actor, "moder", "admin") {
		status = "approved"
	}

	var artifact AdminArtifact
	err = tx.QueryRowContext(ctx, `
		INSERT INTO artifacts (
			app_version_id, file_name, package_type, source_type, storage_path,
			size_bytes, sha256, min_os, max_supported_os, max_tested_os,
			hard_block_above_max, architectures, requires_rosetta, requires_java,
			install_notes, moderation_status
		)
		VALUES (
			$1, $2, $3, 'local', $4, $5, $6, $7,
			NULLIF($8, ''), NULLIF($9, ''), $10, $11, $12, $13,
			NULLIF($14, ''), $15
		)
		RETURNING id, app_version_id, file_name, package_type, source_type,
		          COALESCE(storage_path, ''), COALESCE(primary_download_url, ''),
		          COALESCE(torrent_url, ''), COALESCE(magnet_url, ''),
		          COALESCE(size_bytes, 0), COALESCE(sha256, ''), min_os,
		          COALESCE(max_supported_os, ''), COALESCE(max_tested_os, ''),
		          hard_block_above_max, architectures, requires_rosetta, requires_java,
		          COALESCE(install_notes, ''), moderation_status,
		          created_at::text, updated_at::text
	`, versionID, staged.OriginalFilename, staged.PackageType, staged.StoragePath,
		staged.SizeBytes, staged.SHA256, submission.MinOS,
		strings.TrimSpace(submission.MaxSupportedOS), strings.TrimSpace(submission.MaxTestedOS),
		submission.HardBlockAboveMax, pq.Array(submission.Architectures),
		submission.RequiresRosetta, submission.RequiresJava, strings.TrimSpace(submission.InstallNotes), status,
	).Scan(
		&artifact.ID, &artifact.AppVersionID, &artifact.FileName, &artifact.PackageType, &artifact.SourceType,
		&artifact.StoragePath, &artifact.PrimaryDownloadURL, &artifact.TorrentURL, &artifact.MagnetURL,
		&artifact.SizeBytes, &artifact.SHA256, &artifact.MinOS, &artifact.MaxSupportedOS, &artifact.MaxTestedOS,
		&artifact.HardBlockAboveMax, pq.Array(&artifact.Architectures),
		&artifact.RequiresRosetta, &artifact.RequiresJava, &artifact.InstallNotes,
		&artifact.ModerationStatus, &artifact.CreatedAt, &artifact.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO moderation_queue (entity_type, entity_id, submitted_by, status)
		VALUES ('artifact', $1, $2, $3)
	`, intString(artifact.ID), actor.ID, status); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE staged_uploads
		SET status = 'committed', committed_at = now()
		WHERE public_uid = $1
	`, staged.UID); err != nil {
		return nil, err
	}
	if err := auditTx(ctx, tx, actor.ID, "contributions.commit", "artifact", intString(artifact.ID), ip, userAgent); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &ReleaseSubmissionResult{
		AppID:            staged.AppID,
		VersionID:        versionID,
		Version:          submission.Version,
		Artifact:         artifact,
		ModerationStatus: status,
	}, nil
}

func scanStagedUpload(row rowScanner) (StagedUpload, error) {
	var item StagedUpload
	var warnings string
	err := row.Scan(
		&item.UID, &item.AppID, &item.SubmittedBy, &item.OriginalFilename, &item.StoragePath,
		&item.SHA256, &item.SizeBytes, &item.PackageType, &item.DetectedName,
		&item.DetectedBundleID, &item.DetectedVersion, &item.DetectedCategorySlug,
		&item.DetectedMinOS, pq.Array(&item.DetectedArchitectures), &warnings,
		&item.Status, &item.ExpiresAt, &item.CommittedAt, &item.CreatedAt, &item.UpdatedAt,
	)
	if warnings == "" {
		warnings = "[]"
	}
	item.Warnings = json.RawMessage(warnings)
	return item, err
}
