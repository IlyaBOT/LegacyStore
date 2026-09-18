package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"legacystore/backend/internal/catalog"
	"legacystore/backend/internal/storage"
)

func (r *Router) imageAsset(w http.ResponseWriter, req *http.Request) {
	if !r.requireStore(w) {
		return
	}
	if r.cfg.StorageBackend != "local" {
		writeError(w, http.StatusNotFound, "image_not_found")
		return
	}
	asset, err := r.store.ImageFileByUID(req.Context(), req.PathValue("uid"))
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "image_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "image_failed")
		return
	}

	etag := fmt.Sprintf("%q", asset.SHA256)
	if strings.TrimSpace(req.Header.Get("If-None-Match")) == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	local, err := storage.NewLocal(r.cfg.LocalStoragePath, r.cfg.MaxUploadBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_unavailable")
		return
	}
	file, err := local.Open(asset.StoragePath)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, storage.ErrInvalidPath) {
		writeError(w, http.StatusNotFound, "image_file_missing")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "image_failed")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "image_failed")
		return
	}
	if info.Size() != asset.SizeBytes {
		writeError(w, http.StatusConflict, "image_size_mismatch")
		return
	}

	w.Header().Set("Content-Type", asset.MIMEType)
	w.Header().Set("Content-Length", strconv.FormatInt(asset.SizeBytes, 10))
	w.Header().Set("Content-Disposition", "inline")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	w.Header().Set("ETag", etag)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if asset.MIMEType == "image/svg+xml" {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	}
	http.ServeContent(w, req, asset.UID, info.ModTime(), file)
}
