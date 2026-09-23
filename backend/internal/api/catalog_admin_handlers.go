package api

import (
	"encoding/hex"
	"net/http"
	"strings"

	"legacystore/backend/internal/account"
	"legacystore/backend/internal/architecture"
	"legacystore/backend/internal/compatibility"
)

type adminVersionPayload struct {
	Version       string `json:"version"`
	ReleaseDate   string `json:"release_date"`
	Changelog     string `json:"changelog"`
	IsRecommended *bool  `json:"is_recommended"`
}

type adminArtifactPayload struct {
	FileName           string `json:"file_name"`
	PackageType        string `json:"package_type"`
	SourceType         string `json:"source_type"`
	StoragePath        string `json:"storage_path"`
	PrimaryDownloadURL string `json:"primary_download_url"`
	TorrentURL         string `json:"torrent_url"`
	MagnetURL          string `json:"magnet_url"`
	SizeBytes          int64  `json:"size_bytes"`
	SHA256             string `json:"sha256"`
	MinOS              string `json:"min_os"`
	MaxSupportedOS     string `json:"max_supported_os"`
	MaxTestedOS        string `json:"max_tested_os"`
	HardBlockAboveMax bool     `json:"hard_block_above_max"`
	Architectures     []string `json:"architectures"`
	RequiresRosetta   bool     `json:"requires_rosetta"`
	RequiresJava      bool     `json:"requires_java"`
	InstallNotes      string   `json:"install_notes"`
	ModerationStatus   string `json:"moderation_status"`
}

type adminMirrorPayload struct {
	MirrorType string `json:"mirror_type"`
	URL        string `json:"url"`
	Priority   int    `json:"priority"`
	IsActive   *bool  `json:"is_active"`
}

type adminIconPayload struct {
	AppVersionID int64  `json:"app_version_id"`
	ImageURL     string `json:"image_url"`
	MinOS        string `json:"min_os"`
	MaxOS        string `json:"max_os"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

type adminScreenshotPayload struct {
	AppVersionID int64  `json:"app_version_id"`
	ImageURL     string `json:"image_url"`
	MinOS        string `json:"min_os"`
	MaxOS        string `json:"max_os"`
	Caption      string `json:"caption"`
	SortOrder    int    `json:"sort_order"`
}

func (r *Router) adminVersions(w http.ResponseWriter, req *http.Request) {
	if _, ok := r.requireAdmin(w, req, "uploader", "trusted", "moder", "admin"); !ok {
		return
	}
	appID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	items, err := r.users.ListAdminVersions(req.Context(), appID)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": items})
}

func (r *Router) adminCreateVersion(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "uploader", "trusted", "moder", "admin")
	if !ok {
		return
	}
	appID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminVersionPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if strings.TrimSpace(payload.Version) == "" {
		writeError(w, http.StatusBadRequest, "version_required")
		return
	}
	recommended := false
	if payload.IsRecommended != nil {
		recommended = *payload.IsRecommended
	}
	item, err := r.users.CreateAdminVersion(req.Context(), *actor, appID, payload.Version, payload.ReleaseDate, payload.Changelog, recommended, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"version": item})
}

func (r *Router) adminUpdateVersion(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "moder", "admin")
	if !ok {
		return
	}
	versionID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminVersionPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	item, err := r.users.UpdateAdminVersion(req.Context(), *actor, versionID, payload.Version, payload.ReleaseDate, payload.Changelog, payload.IsRecommended, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": item})
}

func (r *Router) adminDeleteVersion(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "moder", "admin")
	if !ok {
		return
	}
	versionID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.DeleteAdminVersion(req.Context(), *actor, versionID, clientIP(req), req.UserAgent()); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) adminArtifacts(w http.ResponseWriter, req *http.Request) {
	if _, ok := r.requireAdmin(w, req, "uploader", "trusted", "moder", "admin"); !ok {
		return
	}
	versionID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	items, err := r.users.ListAdminArtifacts(req.Context(), versionID)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"artifacts": items})
}

func (r *Router) adminCreateArtifact(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "uploader", "trusted", "moder", "admin")
	if !ok {
		return
	}
	versionID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminArtifactPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if code := validateArtifactPayload(payload, true); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	item := artifactFromPayload(payload)
	item.AppVersionID = versionID
	created, err := r.users.CreateAdminArtifact(req.Context(), *actor, item, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"artifact": created})
}

func (r *Router) adminUpdateArtifact(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "moder", "admin")
	if !ok {
		return
	}
	artifactID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminArtifactPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if code := validateArtifactPayload(payload, false); code != "" {
		writeError(w, http.StatusBadRequest, code)
		return
	}
	updated, err := r.users.UpdateAdminArtifact(req.Context(), *actor, artifactID, artifactFromPayload(payload), clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"artifact": updated})
}

func (r *Router) adminDeleteArtifact(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "moder", "admin")
	if !ok {
		return
	}
	artifactID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.DeleteAdminArtifact(req.Context(), *actor, artifactID, clientIP(req), req.UserAgent()); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) adminMirrors(w http.ResponseWriter, req *http.Request) {
	if _, ok := r.requireAdmin(w, req, "moder", "admin"); !ok {
		return
	}
	artifactID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	items, err := r.users.ListAdminMirrors(req.Context(), artifactID)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mirrors": items})
}

func (r *Router) adminCreateMirror(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "moder", "admin")
	if !ok {
		return
	}
	artifactID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminMirrorPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if !validMirrorType(payload.MirrorType) || strings.TrimSpace(payload.URL) == "" {
		writeError(w, http.StatusBadRequest, "invalid_mirror")
		return
	}
	active := true
	if payload.IsActive != nil {
		active = *payload.IsActive
	}
	if payload.Priority < 0 {
		payload.Priority = 100
	}
	item, err := r.users.CreateAdminMirror(req.Context(), *actor, artifactID, payload.MirrorType, payload.URL, payload.Priority, active, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"mirror": item})
}

func (r *Router) adminUpdateMirror(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "moder", "admin")
	if !ok {
		return
	}
	mirrorID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminMirrorPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if payload.MirrorType != "" && !validMirrorType(payload.MirrorType) {
		writeError(w, http.StatusBadRequest, "invalid_mirror_type")
		return
	}
	item, err := r.users.UpdateAdminMirror(req.Context(), *actor, mirrorID, payload.MirrorType, payload.URL, payload.Priority, payload.IsActive, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mirror": item})
}

func (r *Router) adminDeleteMirror(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "moder", "admin")
	if !ok {
		return
	}
	mirrorID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.DeleteAdminMirror(req.Context(), *actor, mirrorID, clientIP(req), req.UserAgent()); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) adminCreateIcon(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "uploader", "trusted", "moder", "admin")
	if !ok {
		return
	}
	appID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminIconPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if strings.TrimSpace(payload.ImageURL) == "" || !validVersionRange(payload.MinOS, payload.MaxOS) {
		writeError(w, http.StatusBadRequest, "invalid_icon")
		return
	}
	item, err := r.users.CreateAdminIcon(req.Context(), *actor, account.AdminIcon{
		AppID: appID, AppVersionID: payload.AppVersionID, ImageURL: payload.ImageURL,
		MinOS: payload.MinOS, MaxOS: payload.MaxOS, Width: payload.Width, Height: payload.Height,
	}, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"icon": item})
}

func (r *Router) adminDeleteIcon(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "moder", "admin")
	if !ok {
		return
	}
	iconID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.DeleteAdminIcon(req.Context(), *actor, iconID, clientIP(req), req.UserAgent()); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) adminCreateScreenshot(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "moder", "admin")
	if !ok {
		return
	}
	appID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminScreenshotPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if strings.TrimSpace(payload.ImageURL) == "" || !validVersionRange(payload.MinOS, payload.MaxOS) {
		writeError(w, http.StatusBadRequest, "invalid_screenshot")
		return
	}
	item, err := r.users.CreateAdminScreenshot(req.Context(), *actor, account.AdminScreenshot{
		AppID: appID, AppVersionID: payload.AppVersionID, ImageURL: payload.ImageURL, MinOS: payload.MinOS,
		MaxOS: payload.MaxOS, Caption: payload.Caption, SortOrder: payload.SortOrder,
	}, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"screenshot": item})
}

func (r *Router) adminUpdateScreenshot(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "moder", "admin")
	if !ok {
		return
	}
	screenshotID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminScreenshotPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if !validVersionRange(payload.MinOS, payload.MaxOS) {
		writeError(w, http.StatusBadRequest, "invalid_os_range")
		return
	}
	item, err := r.users.UpdateAdminScreenshot(req.Context(), *actor, screenshotID, account.AdminScreenshot{
		AppVersionID: payload.AppVersionID, ImageURL: payload.ImageURL, MinOS: payload.MinOS, MaxOS: payload.MaxOS,
		Caption: payload.Caption, SortOrder: payload.SortOrder,
	}, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"screenshot": item})
}

func (r *Router) adminDeleteScreenshot(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "moder", "admin")
	if !ok {
		return
	}
	screenshotID, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.DeleteAdminScreenshot(req.Context(), *actor, screenshotID, clientIP(req), req.UserAgent()); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func artifactFromPayload(payload adminArtifactPayload) account.AdminArtifact {
	return account.AdminArtifact{
		FileName: payload.FileName, PackageType: payload.PackageType, SourceType: payload.SourceType,
		StoragePath: payload.StoragePath, PrimaryDownloadURL: payload.PrimaryDownloadURL,
		TorrentURL: payload.TorrentURL, MagnetURL: payload.MagnetURL, SizeBytes: payload.SizeBytes,
		SHA256: strings.ToLower(strings.TrimSpace(payload.SHA256)), MinOS: payload.MinOS,
		MaxSupportedOS: payload.MaxSupportedOS, MaxTestedOS: payload.MaxTestedOS,
		HardBlockAboveMax: payload.HardBlockAboveMax, Architectures: payload.Architectures,
		RequiresRosetta: payload.RequiresRosetta, RequiresJava: payload.RequiresJava,
		InstallNotes: payload.InstallNotes, ModerationStatus: payload.ModerationStatus,
	}
}

func validateArtifactPayload(payload adminArtifactPayload, create bool) string {
	if create && strings.TrimSpace(payload.FileName) == "" {
		return "file_name_required"
	}
	if payload.PackageType != "" && !validPackageType(payload.PackageType) {
		return "invalid_package_type"
	}
	if create && payload.PackageType == "" {
		return "package_type_required"
	}
	if payload.SourceType != "" && !validSourceType(payload.SourceType) {
		return "invalid_source_type"
	}
	if create && payload.SourceType == "" {
		return "source_type_required"
	}
	if payload.SHA256 != "" {
		raw, err := hex.DecodeString(payload.SHA256)
		if err != nil || len(raw) != 32 {
			return "invalid_sha256"
		}
	}
	if len(payload.Architectures) > 0 {
		if _, err := architecture.Normalize(payload.Architectures); err != nil {
			return "invalid_architecture"
		}
	}
	if create && len(payload.Architectures) == 0 {
		return "architecture_required"
	}
	if !validVersionRange(payload.MinOS, payload.MaxSupportedOS) || !validVersionRange(payload.MinOS, payload.MaxTestedOS) {
		return "invalid_os_range"
	}
	if payload.ModerationStatus != "" && payload.ModerationStatus != "pending" && payload.ModerationStatus != "approved" && payload.ModerationStatus != "rejected" {
		return "invalid_moderation_status"
	}
	return ""
}

func validPackageType(value string) bool {
	switch value {
	case "app", "dmg", "zip", "pkg", "iso", "other":
		return true
	default:
		return false
	}
}

func validSourceType(value string) bool {
	switch value {
	case "local", "external_direct", "external_page":
		return true
	default:
		return false
	}
}

func validMirrorType(value string) bool {
	switch value {
	case "http", "official", "external_page", "web_seed":
		return true
	default:
		return false
	}
}

func validVersionRange(minVersion, maxVersion string) bool {
	if minVersion == "" || maxVersion == "" {
		if minVersion != "" {
			_, err := compatibility.ParseVersion(minVersion)
			return err == nil
		}
		if maxVersion != "" {
			_, err := compatibility.ParseVersion(maxVersion)
			return err == nil
		}
		return true
	}
	min, err := compatibility.ParseVersion(minVersion)
	if err != nil {
		return false
	}
	max, err := compatibility.ParseVersion(maxVersion)
	if err != nil {
		return false
	}
	return min.Compare(max) <= 0
}
