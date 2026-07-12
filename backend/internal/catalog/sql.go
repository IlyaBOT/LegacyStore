package catalog

import (
	"database/sql"

	"legacystore/backend/internal/compatibility"
)

type appRow struct {
	ID          int64
	Slug        string
	Name        string
	Summary     string
	Category    string
	Icon        string
	Rating      float64
	RatingCount int
}

type appDetailRow struct {
	appRow
	BundleID      string
	DeveloperName string
	Description   string
	WebsiteURL    string
	LicenseType   string
}

type artifactRow struct {
	ID                 int64
	VersionID          int64
	Version            string
	ReleaseDate        string
	Changelog          string
	IsRecommended      bool
	FileName           string
	PackageType        string
	SourceType         string
	StoragePath        string
	PrimaryDownloadURL string
	TorrentURL         string
	MagnetURL          string
	SizeBytes          int64
	SHA256             string
	MinOS              string
	MaxSupportedOS     string
	MaxTestedOS        string
	HardBlockAboveMax  bool
	ArchI386           bool
	ArchX8664          bool
	Supports32Bit      bool
	Supports64Bit      bool
	RequiresRosetta    bool
	RequiresJava       bool
	InstallNotes       string
}

type scanner interface {
	Scan(dest ...any) error
}

func artifactQuery() string {
	return `
		SELECT
			ar.id,
			v.id,
			v.version,
			COALESCE(v.release_date::text, ''),
			COALESCE(v.changelog, ''),
			v.is_recommended,
			ar.file_name,
			ar.package_type,
			ar.source_type,
			COALESCE(ar.storage_path, ''),
			COALESCE(ar.primary_download_url, ''),
			COALESCE(ar.torrent_url, ''),
			COALESCE(ar.magnet_url, ''),
			COALESCE(ar.size_bytes, 0),
			COALESCE(ar.sha256, ''),
			ar.min_os,
			COALESCE(ar.max_supported_os, ''),
			COALESCE(ar.max_tested_os, ''),
			ar.hard_block_above_max,
			ar.arch_i386,
			ar.arch_x86_64,
			ar.supports_32bit,
			ar.supports_64bit,
			ar.requires_rosetta,
			ar.requires_java,
			COALESCE(ar.install_notes, '')
		FROM app_versions v
		JOIN artifacts ar ON ar.app_version_id = v.id
	`
}

func scanArtifact(row scanner) (artifactRow, error) {
	var artifact artifactRow
	err := row.Scan(
		&artifact.ID,
		&artifact.VersionID,
		&artifact.Version,
		&artifact.ReleaseDate,
		&artifact.Changelog,
		&artifact.IsRecommended,
		&artifact.FileName,
		&artifact.PackageType,
		&artifact.SourceType,
		&artifact.StoragePath,
		&artifact.PrimaryDownloadURL,
		&artifact.TorrentURL,
		&artifact.MagnetURL,
		&artifact.SizeBytes,
		&artifact.SHA256,
		&artifact.MinOS,
		&artifact.MaxSupportedOS,
		&artifact.MaxTestedOS,
		&artifact.HardBlockAboveMax,
		&artifact.ArchI386,
		&artifact.ArchX8664,
		&artifact.Supports32Bit,
		&artifact.Supports64Bit,
		&artifact.RequiresRosetta,
		&artifact.RequiresJava,
		&artifact.InstallNotes,
	)
	return artifact, err
}

func (a artifactRow) compatibilityArtifact() compatibility.Artifact {
	return compatibility.Artifact{
		MinOS:             a.MinOS,
		MaxSupportedOS:    a.MaxSupportedOS,
		MaxTestedOS:       a.MaxTestedOS,
		HardBlockAboveMax: a.HardBlockAboveMax,
		ArchI386:          a.ArchI386,
		ArchX8664:         a.ArchX8664,
		Supports32Bit:     a.Supports32Bit,
		Supports64Bit:     a.Supports64Bit,
		RequiresRosetta:   a.RequiresRosetta,
		RequiresJava:      a.RequiresJava,
	}
}

var _ scanner = (*sql.Row)(nil)
var _ scanner = (*sql.Rows)(nil)
