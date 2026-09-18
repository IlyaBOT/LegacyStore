package api

import (
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"legacystore/backend/internal/account"
	"legacystore/backend/internal/media"
	"legacystore/backend/internal/storage"
)

type processedUpload struct {
	Result *media.Result
	Input  account.ImageAssetInput
}

func (r *Router) uploadAvatar(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	uploads, _, ok := r.processImageMultipart(w, req, 1, media.AvatarPolicy())
	if !ok {
		return
	}
	if len(uploads) != 1 {
		writeError(w, http.StatusBadRequest, "image_required")
		return
	}
	asset, err := r.users.SetAvatarImage(req.Context(), *user, uploads[0].Input, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"avatar": asset})
}

func (r *Router) deleteAvatar(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	if err := r.users.DeleteAvatarImage(req.Context(), *user, clientIP(req), req.UserAgent()); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) uploadReviewImages(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	uploads, _, ok := r.processImageMultipart(w, req, 3, media.ReviewPolicy())
	if !ok {
		return
	}
	if len(uploads) == 0 {
		writeError(w, http.StatusBadRequest, "image_required")
		return
	}
	inputs := make([]account.ImageAssetInput, 0, len(uploads))
	for _, upload := range uploads {
		inputs = append(inputs, upload.Input)
	}
	assets, err := r.users.AddReviewImages(req.Context(), *user, req.PathValue("id"), inputs, clientIP(req), req.UserAgent())
	if err != nil {
		if errors.Is(err, account.ErrInvalidCredential) {
			writeError(w, http.StatusBadRequest, "review_image_limit")
			return
		}
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"images": assets})
}

func (r *Router) deleteReviewImage(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	if err := r.users.DeleteReviewImage(
		req.Context(), *user, req.PathValue("id"), req.PathValue("image_uid"), clientIP(req), req.UserAgent(),
	); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) adminUploadIcon(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "trusted", "moder", "admin")
	if !ok {
		return
	}
	appID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	uploads, fields, ok := r.processImageMultipart(w, req, 1, media.IconPolicy())
	if !ok {
		return
	}
	if len(uploads) != 1 {
		writeError(w, http.StatusBadRequest, "image_required")
		return
	}
	versionID := parseOptionalInt64(fields["app_version_id"])
	minOS := defaultString(fields["min_os"], "10.4")
	maxOS := defaultString(fields["max_os"], "15")
	if !validVersionRange(minOS, maxOS) {
		writeError(w, http.StatusBadRequest, "invalid_os_range")
		return
	}
	icon, asset, err := r.users.CreateUploadedIcon(
		req.Context(), *actor, appID, versionID, minOS, maxOS, uploads[0].Input, clientIP(req), req.UserAgent(),
	)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"icon": icon, "image": asset})
}

func (r *Router) adminUploadScreenshots(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "trusted", "moder", "admin")
	if !ok {
		return
	}
	appID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	uploads, fields, ok := r.processImageMultipart(w, req, 3, media.ScreenshotPolicy())
	if !ok {
		return
	}
	if len(uploads) == 0 {
		writeError(w, http.StatusBadRequest, "image_required")
		return
	}
	versionID := parseOptionalInt64(fields["app_version_id"])
	minOS := defaultString(fields["min_os"], "10.4")
	maxOS := defaultString(fields["max_os"], "15")
	if !validVersionRange(minOS, maxOS) {
		writeError(w, http.StatusBadRequest, "invalid_os_range")
		return
	}
	inputs := make([]account.ImageAssetInput, 0, len(uploads))
	for _, upload := range uploads {
		inputs = append(inputs, upload.Input)
	}
	shots, assets, err := r.users.CreateUploadedScreenshots(
		req.Context(), *actor, appID, versionID, minOS, maxOS, fields["caption"], inputs, clientIP(req), req.UserAgent(),
	)
	if err != nil {
		if errors.Is(err, account.ErrInvalidCredential) {
			writeError(w, http.StatusBadRequest, "screenshot_limit")
			return
		}
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"screenshots": shots, "images": assets})
}

func (r *Router) processImageMultipart(w http.ResponseWriter, req *http.Request, maxFiles int, policy media.Policy) ([]processedUpload, map[string]string, bool) {
	if r.cfg.StorageBackend != "local" {
		writeError(w, http.StatusServiceUnavailable, "local_storage_disabled")
		return nil, nil, false
	}
	if maxFiles < 1 {
		maxFiles = 1
	}
	maxBody := int64(maxFiles)*(media.MaxInputBytes+(64<<10)) + (64 << 10)
	req.Body = http.MaxBytesReader(w, req.Body, maxBody)
	reader, err := req.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart")
		return nil, nil, false
	}
	local, err := storage.NewLocal(r.cfg.LocalStoragePath, r.cfg.MaxUploadBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_unavailable")
		return nil, nil, false
	}

	fields := make(map[string]string)
	uploads := make([]processedUpload, 0, maxFiles)
	for {
		part, partErr := reader.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		if errors.Is(partErr, multipart.ErrMessageTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "image_upload_too_large")
			return nil, nil, false
		}
		if partErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_multipart")
			return nil, nil, false
		}

		if part.FileName() == "" {
			value, readErr := io.ReadAll(io.LimitReader(part, 8193))
			_ = part.Close()
			if readErr != nil || len(value) > 8192 {
				writeError(w, http.StatusBadRequest, "field_too_large")
				return nil, nil, false
			}
			if part.FormName() != "" {
				fields[part.FormName()] = strings.TrimSpace(string(value))
			}
			continue
		}
		if part.FormName() != "file" && part.FormName() != "files" && part.FormName() != "images" {
			_ = part.Close()
			continue
		}
		if len(uploads) >= maxFiles {
			_ = part.Close()
			writeError(w, http.StatusBadRequest, "too_many_images")
			return nil, nil, false
		}

		name := filepath.Base(part.FileName())
		result, processErr := media.Process(part, name, policy)
		_ = part.Close()
		if processErr != nil {
			writeMediaError(w, processErr)
			return nil, nil, false
		}
		saved, saveErr := local.SaveImage(result.Data, result.Extension, result.SHA256)
		if saveErr != nil {
			writeError(w, http.StatusInternalServerError, "image_storage_failed")
			return nil, nil, false
		}
		uploads = append(uploads, processedUpload{
			Result: result,
			Input: account.ImageAssetInput{
				StoragePath:      saved.RelativePath,
				MIMEType:         result.MIMEType,
				FileExt:          result.Extension,
				OriginalFilename: result.OriginalFilename,
				SizeBytes:        saved.SizeBytes,
				Width:            result.Width,
				Height:           result.Height,
				SHA256:           result.SHA256,
			},
		})
	}
	return uploads, fields, true
}

func writeMediaError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, media.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "image_upload_too_large")
	case errors.Is(err, media.ErrDimensionsTooLarge):
		writeError(w, http.StatusUnprocessableEntity, "image_dimensions_too_large")
	case errors.Is(err, media.ErrUnsupported):
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_image_type")
	case errors.Is(err, media.ErrUnsafeSVG):
		writeError(w, http.StatusBadRequest, "unsafe_svg")
	case errors.Is(err, media.ErrCompressedTooLarge):
		writeError(w, http.StatusUnprocessableEntity, "image_compression_failed")
	default:
		writeError(w, http.StatusBadRequest, "invalid_image")
	}
}

func parseOptionalInt64(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed < 0 {
		return 0
	}
	return parsed
}
