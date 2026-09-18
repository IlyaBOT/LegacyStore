package catalog

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type ImageFile struct {
	UID         string
	StoragePath string
	MIMEType    string
	SizeBytes   int64
	Width       int
	Height      int
	SHA256      string
}

type ReviewImage struct {
	UID       string `json:"uid"`
	URL       string `json:"url"`
	MIMEType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	SortOrder int    `json:"sort_order"`
}

func (s *Store) ImageFileByUID(ctx context.Context, uid string) (*ImageFile, error) {
	var file ImageFile
	err := s.db.QueryRowContext(ctx, `
		SELECT public_uid, storage_path, mime_type, size_bytes, width, height, sha256
		FROM image_assets
		WHERE public_uid = $1
	`, strings.TrimSpace(uid)).Scan(
		&file.UID, &file.StoragePath, &file.MIMEType, &file.SizeBytes,
		&file.Width, &file.Height, &file.SHA256,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &file, nil
}

func (s *Store) reviewImages(ctx context.Context, appID int64) (map[string][]ReviewImage, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.public_uid,
		       ia.public_uid,
		       ia.mime_type,
		       ia.size_bytes,
		       ia.width,
		       ia.height,
		       ri.sort_order
		FROM reviews r
		JOIN review_images ri ON ri.review_id = r.id
		JOIN image_assets ia ON ia.id = ri.image_id
		WHERE r.app_id = $1 AND r.deleted_at IS NULL
		ORDER BY r.id, ri.sort_order
	`, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]ReviewImage)
	for rows.Next() {
		var reviewUID string
		var image ReviewImage
		if err := rows.Scan(
			&reviewUID, &image.UID, &image.MIMEType, &image.SizeBytes,
			&image.Width, &image.Height, &image.SortOrder,
		); err != nil {
			return nil, err
		}
		image.URL = "/api/v1/images/" + image.UID
		result[reviewUID] = append(result[reviewUID], image)
	}
	return result, rows.Err()
}
