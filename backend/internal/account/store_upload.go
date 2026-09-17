package account

import (
	"context"
	"net"
	"strings"
)

// CreateUploadedArtifact persists metadata for a newly uploaded binary. Binary
// uploads always enter moderation as pending, including uploads by moderators
// and administrators. Publication is therefore an explicit second action.
func (s *Store) CreateUploadedArtifact(ctx context.Context, actor User, item AdminArtifact, ip net.IP, userAgent string) (*AdminArtifact, error) {
	if strings.TrimSpace(item.MinOS) == "" {
		item.MinOS = "10.4"
	}
	item.SourceType = "local"
	item.ModerationStatus = "pending"

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var versionExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM app_versions WHERE id = $1)`, item.AppVersionID).Scan(&versionExists); err != nil {
		return nil, err
	}
	if !versionExists {
		return nil, ErrNotFound
	}

	var created AdminArtifact
	err = tx.QueryRowContext(ctx, `
		INSERT INTO artifacts (
			app_version_id, file_name, package_type, source_type, storage_path, primary_download_url,
			torrent_url, magnet_url, size_bytes, sha256, min_os, max_supported_os, max_tested_os,
			hard_block_above_max, arch_i386, arch_x86_64, supports_32bit, supports_64bit,
			requires_rosetta, requires_java, install_notes, moderation_status
		)
		VALUES (
			$1, $2, $3, 'local', $4, NULL, NULL, NULL, $5, lower($6), $7,
			NULLIF($8, ''), NULLIF($9, ''), $10, $11, $12, $13, $14, $15, $16,
			NULLIF($17, ''), 'pending'
		)
		RETURNING id, app_version_id, file_name, package_type, source_type,
		          COALESCE(storage_path, ''), COALESCE(primary_download_url, ''), COALESCE(torrent_url, ''),
		          COALESCE(magnet_url, ''), COALESCE(size_bytes, 0), COALESCE(sha256, ''), min_os,
		          COALESCE(max_supported_os, ''), COALESCE(max_tested_os, ''), hard_block_above_max,
		          arch_i386, arch_x86_64, supports_32bit, supports_64bit, requires_rosetta, requires_java,
		          COALESCE(install_notes, ''), moderation_status, created_at::text, updated_at::text
	`, item.AppVersionID, strings.TrimSpace(item.FileName), strings.TrimSpace(item.PackageType), strings.TrimSpace(item.StoragePath),
		item.SizeBytes, strings.TrimSpace(item.SHA256), strings.TrimSpace(item.MinOS), strings.TrimSpace(item.MaxSupportedOS),
		strings.TrimSpace(item.MaxTestedOS), item.HardBlockAboveMax, item.ArchI386, item.ArchX8664,
		item.Supports32Bit, item.Supports64Bit, item.RequiresRosetta, item.RequiresJava, strings.TrimSpace(item.InstallNotes)).Scan(
		&created.ID, &created.AppVersionID, &created.FileName, &created.PackageType, &created.SourceType,
		&created.StoragePath, &created.PrimaryDownloadURL, &created.TorrentURL, &created.MagnetURL,
		&created.SizeBytes, &created.SHA256, &created.MinOS, &created.MaxSupportedOS, &created.MaxTestedOS,
		&created.HardBlockAboveMax, &created.ArchI386, &created.ArchX8664, &created.Supports32Bit, &created.Supports64Bit,
		&created.RequiresRosetta, &created.RequiresJava, &created.InstallNotes, &created.ModerationStatus, &created.CreatedAt, &created.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO moderation_queue (entity_type, entity_id, submitted_by, status)
		VALUES ('artifact', $1, $2, 'pending')
	`, intString(created.ID), actor.ID); err != nil {
		return nil, err
	}
	if err := auditTx(ctx, tx, actor.ID, "admin.uploads.create", "artifact", intString(created.ID), ip, userAgent); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &created, nil
}
