package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"legacystore/backend/internal/catalog"
	"legacystore/backend/internal/storage"
)

func (r *Router) localArtifactFile(w http.ResponseWriter, req *http.Request) {
	if !r.requireStore(w) {
		return
	}
	if r.cfg.StorageBackend != "local" {
		writeError(w, http.StatusNotFound, "artifact_not_found")
		return
	}
	artifactID, err := strconv.ParseInt(req.PathValue("artifact_id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_artifact_id")
		return
	}
	metadata, err := r.store.LocalArtifactFile(req.Context(), artifactID)
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "artifact_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "artifact_file_failed")
		return
	}

	local, err := storage.NewLocal(r.cfg.LocalStoragePath, r.cfg.MaxUploadBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_unavailable")
		return
	}
	file, err := local.Open(metadata.StoragePath)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, storage.ErrInvalidPath) {
		writeError(w, http.StatusNotFound, "artifact_file_missing")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "artifact_file_failed")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "artifact_file_failed")
		return
	}
	if metadata.SizeBytes > 0 && info.Size() != metadata.SizeBytes {
		writeError(w, http.StatusConflict, "artifact_size_mismatch")
		return
	}

	name := safeDownloadName(metadata.FileName)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if metadata.SHA256 != "" {
		w.Header().Set("X-Checksum-SHA256", metadata.SHA256)
	}
	w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
	w.Header().Set("Accept-Ranges", "bytes")

	// Only a complete non-range GET can be proven to have transferred the whole
	// file in this request. Range requests remain supported, but are deliberately
	// not counted because a parser or resume probe may fetch only a fragment.
	if req.Method == http.MethodGet && strings.TrimSpace(req.Header.Get("Range")) == "" {
		telemetry, countable := r.requestTelemetry(req)
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
		w.WriteHeader(http.StatusOK)
		written, copyErr := io.Copy(w, file)
		if copyErr == nil && written == info.Size() && countable {
			_, _ = r.store.RecordCompletedDownload(req.Context(), artifactID, telemetry)
		}
		return
	}

	http.ServeContent(w, req, name, info.ModTime(), file)
}

func safeDownloadName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\"", "'")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, "\\", "_")
	if value == "" {
		return "LegacyStore-download"
	}
	return value
}
