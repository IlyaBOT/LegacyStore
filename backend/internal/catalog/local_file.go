package catalog

import (
	"context"
	"database/sql"
	"errors"
)

type LocalFile struct {
	ID          int64
	FileName    string
	StoragePath string
	SizeBytes   int64
	SHA256      string
}

func (s *Store) LocalArtifactFile(ctx context.Context, artifactID int64) (*LocalFile, error) {
	var file LocalFile
	err := s.db.QueryRowContext(ctx, `
		SELECT id, file_name, COALESCE(storage_path, ''), COALESCE(size_bytes, 0), COALESCE(sha256, '')
		FROM artifacts
		WHERE id = $1
		  AND source_type = 'local'
		  AND moderation_status = 'approved'
		  AND storage_path IS NOT NULL
	`, artifactID).Scan(&file.ID, &file.FileName, &file.StoragePath, &file.SizeBytes, &file.SHA256)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &file, nil
}
