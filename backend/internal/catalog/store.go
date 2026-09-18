package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"

	"legacystore/backend/internal/compatibility"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Categories(ctx context.Context) ([]Category, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT slug, name, sort_order
		FROM categories
		ORDER BY sort_order, name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var category Category
		if err := rows.Scan(&category.Slug, &category.Name, &category.SortOrder); err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func (s *Store) Apps(ctx context.Context, filters Filters) ([]AppSummary, error) {
	filters = normalizeFilters(filters)
	args := []any{}
	conditions := []string{"a.moderation_status = 'approved'"}
	orderBy := appOrderBy(filters.Sort)

	if filters.Category != "" {
		args = append(args, filters.Category)
		conditions = append(conditions, fmt.Sprintf("c.slug = $%d", len(args)))
	}

	if filters.Query != "" {
		args = append(args, "%"+filters.Query+"%")
		queryArg := len(args)
		conditions = append(conditions, fmt.Sprintf(`(
			a.name ILIKE $%d OR
			a.developer_name ILIKE $%d OR
			COALESCE(a.bundle_id, '') ILIKE $%d OR
			a.summary ILIKE $%d OR
			COALESCE(a.description, '') ILIKE $%d OR
			c.name ILIKE $%d
		)`, queryArg, queryArg, queryArg, queryArg, queryArg, queryArg))
	}

	offset := (filters.Page - 1) * filters.Limit
	args = append(args, filters.Limit, offset)
	limitArg := len(args) - 1
	offsetArg := len(args)

	query := fmt.Sprintf(`
		SELECT
			a.id,
			a.slug,
			a.name,
			a.summary,
			c.name,
			COALESCE((
				SELECT image_url
				FROM icons
				WHERE app_id = a.id
				ORDER BY app_version_id NULLS FIRST, id
				LIMIT 1
			), '') AS icon,
			COALESCE((
				SELECT image_url
				FROM screenshots
				WHERE app_id = a.id
				ORDER BY sort_order, id
				LIMIT 1
			), '') AS hero_image,
			COALESCE(stats.average_rating, 0) AS rating,
			COALESCE(stats.review_count, 0) AS rating_count,
			COALESCE(stats.positive_review_count, 0) AS positive_reviews,
			COALESCE(stats.download_count, 0) AS downloads,
			COALESCE(stats.view_count, 0) AS views,
			COALESCE(stats.popularity_score, 0) AS popularity_score
		FROM apps a
		JOIN app_categories ac ON ac.app_id = a.id
		JOIN categories c ON c.id = ac.category_id
		LEFT JOIN app_engagement_stats stats ON stats.app_id = a.id
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, strings.Join(conditions, " AND "), orderBy, limitArg, offsetArg)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []AppSummary
	for rows.Next() {
		var row appRow
		if err := rows.Scan(
			&row.ID, &row.Slug, &row.Name, &row.Summary, &row.Category,
			&row.Icon, &row.HeroImage, &row.Rating, &row.RatingCount,
			&row.PositiveReviews, &row.Downloads, &row.Views, &row.PopularityScore,
		); err != nil {
			return nil, err
		}

		artifacts, err := s.artifactsForApp(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		selected, result := chooseArtifact(filters.Target, artifacts)
		if filters.CompatibleOnly && result.Status == "blocked" {
			continue
		}

		summary := AppSummary{
			Slug:          row.Slug,
			Name:          row.Name,
			Category:      row.Category,
			Summary:       row.Summary,
			Icon:          row.Icon,
			HeroImage:     row.HeroImage,
			Rating:          roundRating(row.Rating),
			RatingCount:     row.RatingCount,
			PositiveReviews: row.PositiveReviews,
			Downloads:       row.Downloads,
			Views:           row.Views,
			PopularityScore: row.PopularityScore,
			ArchBadges:      archBadges(selected),
			Compatibility: result,
		}
		if selected != nil {
			summary.RecommendedVersion = selected.Version
		}
		apps = append(apps, summary)
	}

	return apps, rows.Err()
}

func (s *Store) AppBySlug(ctx context.Context, slug string, target compatibility.Target) (*AppDetail, error) {
	var row appDetailRow
	err := s.db.QueryRowContext(ctx, `
		SELECT
			a.id,
			a.slug,
			a.name,
			COALESCE(a.bundle_id, ''),
			a.developer_name,
			a.summary,
			COALESCE(a.description, ''),
			c.name,
			COALESCE(a.website_url, ''),
			COALESCE(a.license_type, ''),
			COALESCE((
				SELECT image_url
				FROM icons
				WHERE app_id = a.id
				ORDER BY app_version_id NULLS FIRST, id
				LIMIT 1
			), '') AS icon,
			COALESCE(stats.average_rating, 0) AS rating,
			COALESCE(stats.review_count, 0) AS rating_count,
			COALESCE(stats.download_count, 0) AS downloads,
			COALESCE(stats.view_count, 0) AS views
		FROM apps a
		JOIN app_categories ac ON ac.app_id = a.id
		JOIN categories c ON c.id = ac.category_id
		LEFT JOIN app_engagement_stats stats ON stats.app_id = a.id
		WHERE a.slug = $1 AND a.moderation_status = 'approved'
		LIMIT 1
	`, slug).Scan(
		&row.ID,
		&row.Slug,
		&row.Name,
		&row.BundleID,
		&row.DeveloperName,
		&row.Summary,
		&row.Description,
		&row.Category,
		&row.WebsiteURL,
		&row.LicenseType,
		&row.Icon,
		&row.Rating,
		&row.RatingCount,
		&row.Downloads,
		&row.Views,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	artifacts, err := s.artifactsForApp(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	selected, result := chooseArtifact(target, artifacts)

	screenshots, err := s.screenshots(ctx, row.ID)
	if err != nil {
		return nil, err
	}

	versions := versionsFromArtifacts(target, artifacts)
	versionDownloads, err := s.VersionDownloadCounts(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	for i := range versions {
		versions[i].Downloads = versionDownloads[versions[i].Version]
	}

	detail := &AppDetail{
		Slug:          row.Slug,
		Name:          row.Name,
		BundleID:      row.BundleID,
		DeveloperName: row.DeveloperName,
		Summary:       row.Summary,
		Description:   row.Description,
		Category:      row.Category,
		WebsiteURL:    row.WebsiteURL,
		LicenseType:   row.LicenseType,
		Icon:          row.Icon,
		Screenshots:   screenshots,
		Rating:        roundRating(row.Rating),
		RatingCount:   row.RatingCount,
		Downloads:     row.Downloads,
		Views:         row.Views,
		Versions:      versions,
		Compatibility: result,
	}
	if selected != nil {
		resp := artifactResponse(*selected)
		resp.CompatibilityLabel = result.Label
		resp.CompatibilityStatus = result.Status
		detail.RecommendedArtifact = &resp
	}

	return detail, nil
}

func (s *Store) Versions(ctx context.Context, slug string, target compatibility.Target) ([]VersionResponse, error) {
	var appID int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM apps WHERE slug = $1 AND moderation_status = 'approved'`, slug).Scan(&appID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	artifacts, err := s.artifactsForApp(ctx, appID)
	if err != nil {
		return nil, err
	}
	versions := versionsFromArtifacts(target, artifacts)
	counts, err := s.VersionDownloadCounts(ctx, appID)
	if err != nil {
		return nil, err
	}
	for i := range versions {
		versions[i].Downloads = counts[versions[i].Version]
	}
	return versions, nil
}

func (s *Store) Download(ctx context.Context, artifactID int64, target compatibility.Target) (*DownloadMetadata, error) {
	artifact, err := s.artifactByID(ctx, artifactID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	resp := artifactResponse(*artifact)
	result := compatibility.Evaluate(target, artifact.compatibilityArtifact())
	resp.CompatibilityLabel = result.Label
	resp.CompatibilityStatus = result.Status

	metadata := &DownloadMetadata{ArtifactResponse: resp}
	mirrors, err := s.mirrors(ctx, artifactID)
	if err != nil {
		return nil, err
	}
	metadata.Mirrors = mirrors

	return metadata, nil
}

func (s *Store) Reviews(ctx context.Context, slug string) ([]Review, error) {
	var appID int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM apps WHERE slug = $1 AND moderation_status = 'approved'`, slug).Scan(&appID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT r.public_uid,
		       r.user_id,
		       COALESCE(NULLIF(u.nickname, ''), u.email) AS author,
		       COALESCE(u.avatar_url, '') AS avatar_url,
		       r.rating,
		       COALESCE(r.title, ''),
		       COALESCE(r.body, ''),
		       COALESCE(r.app_version, ''),
		       COALESCE(r.os_version, ''),
		       COALESCE(r.os_arch, ''),
		       COALESCE(r.device_model, ''),
		       COALESCE(r.client_version, ''),
		       r.source,
		       (SELECT COUNT(*) FROM review_likes rl WHERE rl.review_id = r.id) AS likes,
		       (SELECT COUNT(*) FROM review_replies rr WHERE rr.review_id = r.id AND rr.deleted_at IS NULL) AS replies,
		       r.created_at::text,
		       r.updated_at::text
		FROM reviews r
		JOIN users u ON u.id = r.user_id
		WHERE r.app_id = $1 AND r.deleted_at IS NULL
		ORDER BY r.created_at DESC
		LIMIT 50
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reviews := make([]Review, 0)
	for rows.Next() {
		var review Review
		if err := rows.Scan(
			&review.UID, &review.UserID, &review.Author, &review.AvatarURL,
			&review.Rating, &review.Title, &review.Body, &review.AppVersion,
			&review.OSVersion, &review.OSArch, &review.DeviceModel, &review.ClientVersion,
			&review.Source, &review.Likes, &review.Replies, &review.CreatedAt, &review.UpdatedAt,
		); err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, rows.Err()
}

func (s *Store) artifactsForApp(ctx context.Context, appID int64) ([]artifactRow, error) {
	rows, err := s.db.QueryContext(ctx, artifactQuery()+`
		WHERE v.app_id = $1 AND ar.moderation_status = 'approved'
		ORDER BY v.is_recommended DESC, v.release_date DESC NULLS LAST, v.id DESC, ar.id ASC
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var artifacts []artifactRow
	for rows.Next() {
		artifact, err := scanArtifact(rows)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func (s *Store) artifactByID(ctx context.Context, artifactID int64) (*artifactRow, error) {
	row := s.db.QueryRowContext(ctx, artifactQuery()+`
		WHERE ar.id = $1 AND ar.moderation_status = 'approved'
	`, artifactID)
	artifact, err := scanArtifact(row)
	if err != nil {
		return nil, err
	}
	return &artifact, nil
}

func (s *Store) screenshots(ctx context.Context, appID int64) ([]Screenshot, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT image_url, COALESCE(caption, ''), sort_order
		FROM screenshots
		WHERE app_id = $1
		ORDER BY sort_order, id
		LIMIT 8
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var screenshots []Screenshot
	for rows.Next() {
		var screenshot Screenshot
		if err := rows.Scan(&screenshot.ImageURL, &screenshot.Caption, &screenshot.SortOrder); err != nil {
			return nil, err
		}
		screenshots = append(screenshots, screenshot)
	}
	return screenshots, rows.Err()
}

func (s *Store) mirrors(ctx context.Context, artifactID int64) ([]Mirror, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT mirror_type, url, priority
		FROM artifact_mirrors
		WHERE artifact_id = $1 AND is_active
		ORDER BY priority, id
	`, artifactID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var mirrors []Mirror
	for rows.Next() {
		var mirror Mirror
		if err := rows.Scan(&mirror.Type, &mirror.URL, &mirror.Priority); err != nil {
			return nil, err
		}
		mirrors = append(mirrors, mirror)
	}
	return mirrors, rows.Err()
}

func chooseArtifact(target compatibility.Target, artifacts []artifactRow) (*artifactRow, compatibility.Result) {
	if len(artifacts) == 0 {
		return nil, compatibility.Result{Status: "blocked", Level: "blocked", Label: "Не совместимо", Reasons: []compatibility.Reason{{
			Code:    "unknown_compatibility",
			Message: "Для приложения нет опубликованных файлов.",
		}}}
	}

	sorted := append([]artifactRow(nil), artifacts...)
	sort.SliceStable(sorted, func(i, j int) bool {
		cmp := compatibility.CompareVersions(sorted[i].Version, sorted[j].Version)
		if cmp != 0 {
			return cmp > 0
		}
		if sorted[i].IsRecommended != sorted[j].IsRecommended {
			return sorted[i].IsRecommended
		}
		return sorted[i].ID < sorted[j].ID
	})

	var fallback *artifactRow
	var fallbackResult compatibility.Result
	for i := range sorted {
		result := compatibility.Evaluate(target, sorted[i].compatibilityArtifact())
		if fallback == nil {
			fallback = &sorted[i]
			fallbackResult = result
		}
		if result.Status == "compatible" || result.Status == "probably_compatible" || result.Status == "untested" {
			return &sorted[i], result
		}
	}

	return fallback, fallbackResult
}

func versionsFromArtifacts(target compatibility.Target, artifacts []artifactRow) []VersionResponse {
	byVersion := make(map[string]*VersionResponse)
	for _, artifact := range artifacts {
		version := byVersion[artifact.Version]
		if version == nil {
			version = &VersionResponse{
				Version:       artifact.Version,
				ReleaseDate:   artifact.ReleaseDate,
				Changelog:     artifact.Changelog,
				IsRecommended: artifact.IsRecommended,
			}
			byVersion[artifact.Version] = version
		}
		resp := artifactResponse(artifact)
		result := compatibility.Evaluate(target, artifact.compatibilityArtifact())
		resp.CompatibilityLabel = result.Label
		resp.CompatibilityStatus = result.Status
		version.Artifacts = append(version.Artifacts, resp)
	}

	versions := make([]VersionResponse, 0, len(byVersion))
	for _, version := range byVersion {
		versions = append(versions, *version)
	}
	sort.SliceStable(versions, func(i, j int) bool {
		return compatibility.CompareVersions(versions[i].Version, versions[j].Version) > 0
	})
	return versions
}

func artifactResponse(row artifactRow) ArtifactResponse {
	resp := ArtifactResponse{
		ID:                row.ID,
		Version:           row.Version,
		FileName:          row.FileName,
		PackageType:       row.PackageType,
		SourceType:        row.SourceType,
		DownloadURL:       row.PrimaryDownloadURL,
		TorrentURL:        row.TorrentURL,
		MagnetURL:         row.MagnetURL,
		SizeBytes:         row.SizeBytes,
		SHA256:            row.SHA256,
		MinOS:             row.MinOS,
		MaxSupportedOS:    row.MaxSupportedOS,
		MaxTestedOS:       row.MaxTestedOS,
		HardBlockAboveMax: row.HardBlockAboveMax,
		Archs:             artifactArchs(row),
		Supports32Bit:     row.Supports32Bit,
		Supports64Bit:     row.Supports64Bit,
		RequiresJava:      row.RequiresJava,
		InstallNotes:      row.InstallNotes,
	}
	if row.SourceType == "external_page" {
		resp.ExternalPageURL = row.PrimaryDownloadURL
		resp.DownloadURL = ""
	}
	return resp
}

func artifactArchs(row artifactRow) []string {
	var archs []string
	if row.ArchI386 {
		archs = append(archs, "i386")
	}
	if row.ArchX8664 {
		archs = append(archs, "x86_64")
	}
	return archs
}

func archBadges(row *artifactRow) []string {
	if row == nil {
		return nil
	}
	if row.ArchI386 && row.ArchX8664 {
		return []string{"Intel", "i386", "x86_64"}
	}
	if row.ArchI386 {
		return []string{"Intel", "i386"}
	}
	return []string{"Intel", "x86_64"}
}

func normalizeFilters(filters Filters) Filters {
	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.Limit < 1 {
		filters.Limit = 24
	}
	if filters.Limit > 50 {
		filters.Limit = 50
	}
	if filters.Target.OSVersion == "" {
		filters.Target.OSVersion = "10.9.5"
	}
	if filters.Target.Arch == "" {
		filters.Target.Arch = "x86_64"
	}
	return filters
}

func roundRating(rating float64) float64 {
	return math.Round(rating*10) / 10
}
