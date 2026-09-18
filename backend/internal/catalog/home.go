package catalog

import (
	"context"

	"legacystore/backend/internal/compatibility"
)

func (s *Store) Home(ctx context.Context, target compatibility.Target) (*HomeFeed, error) {
	load := func(sortMode string, limit int) ([]AppSummary, error) {
		return s.Apps(ctx, Filters{
			Page:   1,
			Limit:  limit,
			Sort:   sortMode,
			Target: target,
		})
	}

	popular, err := load("popular", 6)
	if err != nil {
		return nil, err
	}
	topDownloads, err := load("downloads", 6)
	if err != nil {
		return nil, err
	}
	newReleases, err := load("new", 6)
	if err != nil {
		return nil, err
	}

	feed := &HomeFeed{
		Popular:      popular,
		TopDownloads: topDownloads,
		NewReleases:  newReleases,
		Slides:       make([]HomeSlide, 0, 4),
	}

	rankings := []struct {
		metric string
		sort   string
	}{
		{metric: "Most Popular", sort: "popular"},
		{metric: "Most Downloaded", sort: "downloads"},
		{metric: "Top This Week", sort: "downloads-week"},
		{metric: "Top Today", sort: "downloads-day"},
	}
	for _, ranking := range rankings {
		candidates, loadErr := load(ranking.sort, 1)
		if loadErr != nil {
			return nil, loadErr
		}
		if len(candidates) > 0 {
			feed.Slides = append(feed.Slides, HomeSlide{Metric: ranking.metric, App: candidates[0]})
		}
	}

	return feed, nil
}

func (s *Store) RecordDownload(ctx context.Context, artifactID int64) error {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO download_events (artifact_id, app_id)
		SELECT ar.id, v.app_id
		FROM artifacts ar
		JOIN app_versions v ON v.id = ar.app_version_id
		WHERE ar.id = $1
		  AND ar.moderation_status = 'approved'
	`, artifactID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func appOrderBy(sortMode string) string {
	switch sortMode {
	case "popular":
		return "COALESCE(stats.popularity_score, 0) DESC, COALESCE(stats.positive_review_count, 0) DESC, COALESCE(stats.download_count, 0) DESC, a.updated_at DESC, a.name"
	case "downloads":
		return "COALESCE(stats.download_count, 0) DESC, a.updated_at DESC, a.name"
	case "downloads-week":
		return `(
			SELECT COUNT(*) FROM download_events de
			WHERE de.app_id = a.id
			  AND de.completed_at IS NOT NULL
			  AND de.completed_at >= now() - interval '7 days'
		) DESC,
		a.updated_at DESC,
		a.name`
	case "downloads-day":
		return `(
			SELECT COUNT(*) FROM download_events de
			WHERE de.app_id = a.id
			  AND de.completed_at IS NOT NULL
			  AND de.completed_at >= now() - interval '24 hours'
		) DESC,
		a.updated_at DESC,
		a.name`
	case "new":
		return "stats.latest_release_at DESC NULLS LAST, a.created_at DESC, a.name"
	default:
		return "a.name"
	}
}
