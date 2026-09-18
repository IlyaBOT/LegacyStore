package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type ClientTelemetry struct {
	UserID           int64
	UsernameSnapshot string
	IPAddress        string
	Source           string
	UserAgent        string
	ClientVersion    string
	OSVersion        string
	OSArch           string
	DeviceModel      string
}

func (s *Store) RecordCompletedDownload(ctx context.Context, artifactID int64, telemetry ClientTelemetry) (bool, error) {
	telemetry = normalizeTelemetry(telemetry)
	if telemetry.UserID == 0 && telemetry.IPAddress == "" {
		return false, nil
	}

	var result sql.Result
	var err error
	if telemetry.UserID > 0 {
		result, err = s.db.ExecContext(ctx, `
			INSERT INTO download_events (
				artifact_id, app_id, app_version_id, user_id, username_snapshot,
				ip_address, source, user_agent, client_version, os_version,
				os_arch, device_model, completed_at
			)
			SELECT
				ar.id, v.app_id, ar.app_version_id, $2, NULLIF($3, ''),
				NULLIF($4, '')::inet, $5, NULLIF($6, ''), NULLIF($7, ''),
				NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''), now()
			FROM artifacts ar
			JOIN app_versions v ON v.id = ar.app_version_id
			WHERE ar.id = $1 AND ar.moderation_status = 'approved'
			ON CONFLICT (app_version_id, user_id)
			WHERE user_id IS NOT NULL AND app_version_id IS NOT NULL AND completed_at IS NOT NULL
			DO NOTHING
		`,
			artifactID, telemetry.UserID, telemetry.UsernameSnapshot, telemetry.IPAddress,
			telemetry.Source, telemetry.UserAgent, telemetry.ClientVersion,
			telemetry.OSVersion, telemetry.OSArch, telemetry.DeviceModel,
		)
	} else {
		result, err = s.db.ExecContext(ctx, `
			INSERT INTO download_events (
				artifact_id, app_id, app_version_id, ip_address, source,
				user_agent, client_version, os_version, os_arch, device_model, completed_at
			)
			SELECT
				ar.id, v.app_id, ar.app_version_id, NULLIF($2, '')::inet, $3,
				NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''),
				NULLIF($7, ''), NULLIF($8, ''), now()
			FROM artifacts ar
			JOIN app_versions v ON v.id = ar.app_version_id
			WHERE ar.id = $1 AND ar.moderation_status = 'approved'
			ON CONFLICT (app_version_id, ip_address)
			WHERE user_id IS NULL AND app_version_id IS NOT NULL AND ip_address IS NOT NULL AND completed_at IS NOT NULL
			DO NOTHING
		`,
			artifactID, telemetry.IPAddress, telemetry.Source, telemetry.UserAgent,
			telemetry.ClientVersion, telemetry.OSVersion, telemetry.OSArch, telemetry.DeviceModel,
		)
	}
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) RecordAppView(ctx context.Context, slug string, telemetry ClientTelemetry) (bool, error) {
	telemetry = normalizeTelemetry(telemetry)
	if telemetry.UserID == 0 && telemetry.IPAddress == "" {
		return false, nil
	}

	var result sql.Result
	var err error
	if telemetry.UserID > 0 {
		result, err = s.db.ExecContext(ctx, `
			INSERT INTO app_view_events (
				app_id, user_id, username_snapshot, ip_address, source,
				user_agent, client_version, os_version, os_arch, device_model
			)
			SELECT
				a.id, $2, NULLIF($3, ''), NULLIF($4, '')::inet, $5,
				NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''),
				NULLIF($9, ''), NULLIF($10, '')
			FROM apps a
			WHERE a.slug = $1 AND a.moderation_status = 'approved'
			ON CONFLICT (app_id, user_id)
			WHERE user_id IS NOT NULL
			DO UPDATE SET
				last_viewed_at = now(),
				ip_address = EXCLUDED.ip_address,
				source = EXCLUDED.source,
				user_agent = EXCLUDED.user_agent,
				client_version = EXCLUDED.client_version,
				os_version = EXCLUDED.os_version,
				os_arch = EXCLUDED.os_arch,
				device_model = EXCLUDED.device_model
		`,
			slug, telemetry.UserID, telemetry.UsernameSnapshot, telemetry.IPAddress,
			telemetry.Source, telemetry.UserAgent, telemetry.ClientVersion,
			telemetry.OSVersion, telemetry.OSArch, telemetry.DeviceModel,
		)
	} else {
		result, err = s.db.ExecContext(ctx, `
			INSERT INTO app_view_events (
				app_id, ip_address, source, user_agent, client_version,
				os_version, os_arch, device_model
			)
			SELECT
				a.id, NULLIF($2, '')::inet, $3, NULLIF($4, ''),
				NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, '')
			FROM apps a
			WHERE a.slug = $1 AND a.moderation_status = 'approved'
			ON CONFLICT (app_id, ip_address)
			WHERE user_id IS NULL AND ip_address IS NOT NULL
			DO UPDATE SET
				last_viewed_at = now(),
				source = EXCLUDED.source,
				user_agent = EXCLUDED.user_agent,
				client_version = EXCLUDED.client_version,
				os_version = EXCLUDED.os_version,
				os_arch = EXCLUDED.os_arch,
				device_model = EXCLUDED.device_model
		`,
			slug, telemetry.IPAddress, telemetry.Source, telemetry.UserAgent,
			telemetry.ClientVersion, telemetry.OSVersion, telemetry.OSArch, telemetry.DeviceModel,
		)
	}
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *Store) AppIDBySlug(ctx context.Context, slug string) (int64, error) {
	var appID int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id FROM apps
		WHERE slug = $1 AND moderation_status = 'approved'
	`, slug).Scan(&appID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return appID, err
}

func (s *Store) VersionDownloadCounts(ctx context.Context, appID int64) (map[string]int64, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT v.version, COUNT(de.id)
		FROM app_versions v
		LEFT JOIN download_events de
		  ON de.app_version_id = v.id
		 AND de.completed_at IS NOT NULL
		WHERE v.app_id = $1
		GROUP BY v.id, v.version
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var version string
		var count int64
		if err := rows.Scan(&version, &count); err != nil {
			return nil, err
		}
		counts[version] = count
	}
	return counts, rows.Err()
}

func normalizeTelemetry(value ClientTelemetry) ClientTelemetry {
	value.UsernameSnapshot = truncateTelemetry(value.UsernameSnapshot, 160)
	value.IPAddress = strings.TrimSpace(value.IPAddress)
	value.Source = strings.TrimSpace(value.Source)
	value.UserAgent = truncateTelemetry(value.UserAgent, 512)
	value.ClientVersion = truncateTelemetry(value.ClientVersion, 64)
	value.OSVersion = truncateTelemetry(value.OSVersion, 32)
	value.OSArch = strings.TrimSpace(value.OSArch)
	value.DeviceModel = truncateTelemetry(value.DeviceModel, 160)

	switch value.Source {
	case "web", "legacy", "native":
	default:
		value.Source = "web"
	}
	if value.OSArch != "i386" && value.OSArch != "x86_64" {
		value.OSArch = ""
	}
	return value
}

func truncateTelemetry(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
