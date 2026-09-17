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
	"legacystore/backend/internal/compatibility"
	"legacystore/backend/internal/macmeta"
	"legacystore/backend/internal/storage"
)

func (r *Router) adminInspectUpload(w http.ResponseWriter, req *http.Request) {
	if _, ok := r.requireAdmin(w, req, "trusted", "moder", "admin"); !ok {
		return
	}
	if r.cfg.StorageBackend != "local" {
		writeError(w, http.StatusServiceUnavailable, "local_storage_disabled")
		return
	}

	local, err := storage.NewLocal(r.cfg.LocalStoragePath, r.cfg.MaxUploadBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_unavailable")
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, r.cfg.MaxUploadBytes+(2<<20))
	reader, err := req.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart")
		return
	}

	var saved *storage.SavedFile
	var uploadedName string
	for {
		part, partErr := reader.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		if errors.Is(partErr, multipart.ErrMessageTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large")
			return
		}
		if partErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_multipart")
			return
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		if saved != nil {
			_ = part.Close()
			_ = local.Remove(saved.RelativePath)
			writeError(w, http.StatusBadRequest, "multiple_files_not_allowed")
			return
		}
		uploadedName = filepath.Base(part.FileName())
		if uploadedName == "." || uploadedName == "" {
			_ = part.Close()
			writeError(w, http.StatusBadRequest, "invalid_file_name")
			return
		}
		saved, err = local.SaveQuarantine(part, uploadedName)
		_ = part.Close()
		if errors.Is(err, storage.ErrTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "upload_failed")
			return
		}
	}

	if saved == nil {
		writeError(w, http.StatusBadRequest, "file_required")
		return
	}
	defer local.Remove(saved.RelativePath)

	absolute, err := local.Resolve(saved.RelativePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_unavailable")
		return
	}
	result := macmeta.InspectFile(req.Context(), absolute, uploadedName)
	writeJSON(w, http.StatusOK, map[string]any{
		"inspection": result,
		"upload": map[string]any{
			"sha256":     saved.SHA256,
			"size_bytes": saved.SizeBytes,
		},
	})
}

func (r *Router) adminInspectAppBundle(w http.ResponseWriter, req *http.Request) {
	if _, ok := r.requireAdmin(w, req, "trusted", "moder", "admin"); !ok {
		return
	}
	req.Body = http.MaxBytesReader(w, req.Body, 24<<20)
	reader, err := req.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart")
		return
	}

	var plistData []byte
	var iconData []byte
	var iconName string
	for {
		part, partErr := reader.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		if partErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_multipart")
			return
		}
		switch part.FormName() {
		case "plist":
			plistData, err = io.ReadAll(io.LimitReader(part, (8<<20)+1))
			if err == nil && len(plistData) > 8<<20 {
				err = storage.ErrTooLarge
			}
		case "icon":
			iconName = filepath.Base(part.FileName())
			iconData, err = io.ReadAll(io.LimitReader(part, (16<<20)+1))
			if err == nil && len(iconData) > 16<<20 {
				err = storage.ErrTooLarge
			}
		}
		_ = part.Close()
		if err != nil {
			if errors.Is(err, storage.ErrTooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, "metadata_too_large")
			} else {
				writeError(w, http.StatusBadRequest, "invalid_multipart")
			}
			return
		}
	}
	if len(plistData) == 0 {
		writeError(w, http.StatusBadRequest, "plist_required")
		return
	}
	result := macmeta.InspectAppBundleParts(plistData, iconData, iconName)
	writeJSON(w, http.StatusOK, map[string]any{"inspection": result})
}

func (r *Router) adminUploadArtifact(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "trusted", "moder", "admin")
	if !ok {
		return
	}
	if r.cfg.StorageBackend != "local" {
		writeError(w, http.StatusServiceUnavailable, "local_storage_disabled")
		return
	}
	versionID, ok := pathID(w, req, "id")
	if !ok {
		return
	}

	local, err := storage.NewLocal(r.cfg.LocalStoragePath, r.cfg.MaxUploadBytes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "storage_unavailable")
		return
	}

	req.Body = http.MaxBytesReader(w, req.Body, r.cfg.MaxUploadBytes+(2<<20))
	reader, err := req.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_multipart")
		return
	}

	fields := make(map[string]string)
	var saved *storage.SavedFile
	var uploadedName string
	for {
		part, partErr := reader.NextPart()
		if errors.Is(partErr, io.EOF) {
			break
		}
		if errors.Is(partErr, multipart.ErrMessageTooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large")
			return
		}
		if partErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_multipart")
			return
		}
		name := part.FormName()
		if name == "file" {
			if saved != nil {
				_ = part.Close()
				_ = local.Remove(saved.RelativePath)
				writeError(w, http.StatusBadRequest, "multiple_files_not_allowed")
				return
			}
			uploadedName = filepath.Base(part.FileName())
			if uploadedName == "." || uploadedName == "" {
				_ = part.Close()
				writeError(w, http.StatusBadRequest, "invalid_file_name")
				return
			}
			saved, err = local.SaveQuarantine(part, uploadedName)
			_ = part.Close()
			if errors.Is(err, storage.ErrTooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, "upload_too_large")
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, "upload_failed")
				return
			}
			continue
		}
		if name != "" {
			value, readErr := io.ReadAll(io.LimitReader(part, 8193))
			_ = part.Close()
			if readErr != nil {
				if saved != nil {
					_ = local.Remove(saved.RelativePath)
				}
				writeError(w, http.StatusBadRequest, "invalid_multipart")
				return
			}
			if len(value) > 8192 {
				if saved != nil {
					_ = local.Remove(saved.RelativePath)
				}
				writeError(w, http.StatusBadRequest, "field_too_large")
				return
			}
			fields[name] = strings.TrimSpace(string(value))
		}
	}

	if saved == nil {
		writeError(w, http.StatusBadRequest, "file_required")
		return
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = local.Remove(saved.RelativePath)
		}
	}()

	packageType := strings.ToLower(fields["package_type"])
	if packageType == "" {
		packageType = packageTypeFromName(uploadedName)
	}
	if !validUploadPackageType(packageType) {
		writeError(w, http.StatusBadRequest, "invalid_package_type")
		return
	}

	artifact := account.AdminArtifact{
		AppVersionID:      versionID,
		FileName:          uploadedName,
		PackageType:       packageType,
		SourceType:        "local",
		StoragePath:       saved.RelativePath,
		SizeBytes:         saved.SizeBytes,
		SHA256:            saved.SHA256,
		MinOS:             defaultString(fields["min_os"], "10.4"),
		MaxSupportedOS:    fields["max_supported_os"],
		MaxTestedOS:       fields["max_tested_os"],
		HardBlockAboveMax: parseFormBool(fields["hard_block_above_max"]),
		ArchI386:          parseFormBool(fields["arch_i386"]),
		ArchX8664:         parseFormBool(fields["arch_x86_64"]),
		Supports32Bit:     parseFormBool(fields["supports_32bit"]),
		Supports64Bit:     parseFormBool(fields["supports_64bit"]),
		RequiresRosetta:   parseFormBool(fields["requires_rosetta"]),
		RequiresJava:      parseFormBool(fields["requires_java"]),
		InstallNotes:      fields["install_notes"],
	}
	if !artifact.ArchI386 && !artifact.ArchX8664 {
		writeError(w, http.StatusBadRequest, "architecture_required")
		return
	}
	if !artifact.Supports32Bit && !artifact.Supports64Bit {
		writeError(w, http.StatusBadRequest, "bitness_required")
		return
	}
	if !validOSRange(artifact.MinOS, artifact.MaxSupportedOS, artifact.MaxTestedOS) {
		writeError(w, http.StatusBadRequest, "invalid_os_range")
		return
	}

	created, err := r.users.CreateUploadedArtifact(req.Context(), *actor, artifact, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	cleanup = false
	writeJSON(w, http.StatusOK, map[string]any{
		"artifact": created,
		"upload": map[string]any{
			"status":     "quarantined",
			"sha256":     saved.SHA256,
			"size_bytes": saved.SizeBytes,
		},
	})
}

func validOSRange(minOS, maxSupportedOS, maxTestedOS string) bool {
	minVersion, err := compatibility.ParseVersion(minOS)
	if err != nil {
		return false
	}
	if maxSupportedOS != "" {
		maxVersion, err := compatibility.ParseVersion(maxSupportedOS)
		if err != nil || minVersion.Compare(maxVersion) > 0 {
			return false
		}
	}
	if maxTestedOS != "" {
		testedVersion, err := compatibility.ParseVersion(maxTestedOS)
		if err != nil || minVersion.Compare(testedVersion) > 0 {
			return false
		}
	}
	return true
}

func packageTypeFromName(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".app":
		return "app"
	case ".dmg":
		return "dmg"
	case ".pkg", ".mpkg":
		return "pkg"
	case ".zip":
		return "zip"
	case ".iso":
		return "iso"
	default:
		return "other"
	}
}

func validUploadPackageType(value string) bool {
	switch value {
	case "app", "dmg", "zip", "pkg", "iso", "other":
		return true
	default:
		return false
	}
}

func parseFormBool(value string) bool {
	parsed, _ := strconv.ParseBool(strings.TrimSpace(value))
	return parsed
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
