package catalog

import (
	"context"
	"time"
)

type ManifestPayload struct {
	SchemaVersion int           `json:"schema_version"`
	GeneratedAt   string        `json:"generated_at"`
	Categories    []Category    `json:"categories"`
	Apps          []ManifestApp `json:"apps"`
}

type ManifestApp struct {
	Slug          string            `json:"slug"`
	Name          string            `json:"name"`
	BundleID      string            `json:"bundle_id,omitempty"`
	DeveloperName string            `json:"developer_name"`
	Summary       string            `json:"summary"`
	Category      string            `json:"category"`
	Icon          string            `json:"icon,omitempty"`
	UpdatedAt     string            `json:"updated_at"`
	Versions      []ManifestVersion `json:"versions"`
}

type ManifestVersion struct {
	Version       string             `json:"version"`
	ReleaseDate   string             `json:"release_date,omitempty"`
	Changelog     string             `json:"changelog,omitempty"`
	IsRecommended bool               `json:"is_recommended"`
	Artifacts     []ManifestArtifact `json:"artifacts"`
}

type ManifestArtifact struct {
	ID                int64    `json:"id"`
	FileName          string   `json:"file_name"`
	PackageType       string   `json:"package_type"`
	SourceType        string   `json:"source_type"`
	DownloadURL       string   `json:"download_url,omitempty"`
	TorrentURL        string   `json:"torrent_url,omitempty"`
	MagnetURL         string   `json:"magnet_url,omitempty"`
	SizeBytes         int64    `json:"size_bytes"`
	SHA256            string   `json:"sha256"`
	MinOS             string   `json:"min_os"`
	MaxSupportedOS    string   `json:"max_supported_os,omitempty"`
	MaxTestedOS       string   `json:"max_tested_os,omitempty"`
	HardBlockAboveMax bool     `json:"hard_block_above_max,omitempty"`
	Architectures     []string `json:"architectures"`
	RequiresRosetta   bool     `json:"requires_rosetta,omitempty"`
	RequiresJava      bool     `json:"requires_java,omitempty"`
	InstallNotes      string   `json:"install_notes,omitempty"`
}

func (s *Store) Manifest(ctx context.Context) (*ManifestPayload, error) {
	categories, err := s.Categories(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.slug, a.name, COALESCE(a.bundle_id, ''), a.developer_name, a.summary,
		       c.name,
		       COALESCE((
		           SELECT image_url FROM icons WHERE app_id = a.id
		           ORDER BY app_version_id NULLS FIRST, id LIMIT 1
		       ), ''),
		       a.updated_at::text
		FROM apps a
		JOIN app_categories ac ON ac.app_id = a.id
		JOIN categories c ON c.id = ac.category_id
		WHERE a.moderation_status = 'approved'
		ORDER BY a.slug, c.slug
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	apps := make([]ManifestApp, 0)
	for rows.Next() {
		var appID int64
		var app ManifestApp
		if err := rows.Scan(&appID, &app.Slug, &app.Name, &app.BundleID, &app.DeveloperName, &app.Summary, &app.Category, &app.Icon, &app.UpdatedAt); err != nil {
			return nil, err
		}
		artifacts, err := s.artifactsForApp(ctx, appID)
		if err != nil {
			return nil, err
		}
		app.Versions = manifestVersions(artifacts)
		apps = append(apps, app)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &ManifestPayload{
		SchemaVersion: 2,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Categories:    categories,
		Apps:          apps,
	}, nil
}

func manifestVersions(artifacts []artifactRow) []ManifestVersion {
	versions := make([]ManifestVersion, 0)
	index := make(map[int64]int)
	for _, artifact := range artifacts {
		position, ok := index[artifact.VersionID]
		if !ok {
			position = len(versions)
			index[artifact.VersionID] = position
			versions = append(versions, ManifestVersion{
				Version:       artifact.Version,
				ReleaseDate:   artifact.ReleaseDate,
				Changelog:     artifact.Changelog,
				IsRecommended: artifact.IsRecommended,
				Artifacts:     make([]ManifestArtifact, 0),
			})
		}
		versions[position].Artifacts = append(versions[position].Artifacts, ManifestArtifact{
			ID:                artifact.ID,
			FileName:          artifact.FileName,
			PackageType:       artifact.PackageType,
			SourceType:        artifact.SourceType,
			DownloadURL:       artifact.PrimaryDownloadURL,
			TorrentURL:        artifact.TorrentURL,
			MagnetURL:         artifact.MagnetURL,
			SizeBytes:         artifact.SizeBytes,
			SHA256:            artifact.SHA256,
			MinOS:             artifact.MinOS,
			MaxSupportedOS:    artifact.MaxSupportedOS,
			MaxTestedOS:       artifact.MaxTestedOS,
			HardBlockAboveMax: artifact.HardBlockAboveMax,
			Architectures:     append([]string(nil), artifact.Architectures...),
			RequiresRosetta:   artifact.RequiresRosetta,
			RequiresJava:      artifact.RequiresJava,
			InstallNotes:      artifact.InstallNotes,
		})
	}
	return versions
}
