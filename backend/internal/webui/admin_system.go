package webui

import (
	"net/http"
	"strconv"
	"strings"

	"legacystore/backend/internal/account"
)

func (h *Handler) registerAdminSystemRoutes() {
	h.mux.HandleFunc("POST /admin/system/users/create", h.adminCreateUser)
	h.mux.HandleFunc("POST /admin/system/users/{id}/update", h.adminUpdateUser)
	h.mux.HandleFunc("POST /admin/system/users/{id}/delete", h.adminDeleteUser)
	h.mux.HandleFunc("POST /admin/system/apps/create", h.adminCreateApp)
	h.mux.HandleFunc("POST /admin/system/apps/{id}/update", h.adminUpdateApp)
	h.mux.HandleFunc("POST /admin/system/apps/{id}/delete", h.adminDeleteApp)
	h.mux.HandleFunc("POST /admin/system/uploads/create", h.adminCreateArtifact)
	h.mux.HandleFunc("POST /admin/system/uploads/{id}/update", h.adminUpdateArtifact)
	h.mux.HandleFunc("POST /admin/system/uploads/{id}/delete", h.adminDeleteArtifact)
	h.mux.HandleFunc("POST /admin/system/reviews/create", h.adminCreateReview)
	h.mux.HandleFunc("POST /admin/system/reviews/{id}/update", h.adminUpdateReview)
	h.mux.HandleFunc("POST /admin/system/reviews/{id}/delete", h.adminDeleteReview)
}

func (h *Handler) renderAdminSystem(w http.ResponseWriter, req *http.Request) {
	data := h.baseData(req, "Управление системой", "system")
	data.SystemTab = req.URL.Query().Get("tab")
	if data.SystemTab == "" {
		data.SystemTab = "users"
	}
	var err error
	switch data.SystemTab {
	case "users":
		data.Users, err = h.users.ListUsers(req.Context(), 100)
	case "apps":
		data.AdminApps, err = h.users.ListAdminApps(req.Context(), "", 100)
	case "uploads":
		data.Artifacts, err = h.users.ListAdminArtifactsAll(req.Context(), 100)
	case "reviews":
		data.Reviews, err = h.users.ListAdminReviews(req.Context(), 100)
	case "audit":
		data.Audit, err = h.users.AuditLog(req.Context())
	default:
		data.SystemTab = "users"
		data.Users, err = h.users.ListUsers(req.Context(), 100)
	}
	if err != nil {
		h.renderError(w, req, http.StatusInternalServerError, "Не удалось загрузить данные панели управления.")
		return
	}
	h.render(w, "admin_system", http.StatusOK, data)
}

func (h *Handler) adminCreateUser(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok || !h.parseForm(w, req) {
		return
	}
	_, err := h.users.AdminCreateUser(req.Context(), *actor, req.FormValue("email"), req.FormValue("nickname"), req.FormValue("password"), req.FormValue("role"), clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	h.finishAdminMutation(w, req, "users", err)
}

func (h *Handler) adminUpdateUser(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok || !h.parseForm(w, req) {
		return
	}
	id, err := pathInt64(req, "id")
	if err != nil {
		h.renderError(w, req, http.StatusBadRequest, "Некорректный ID пользователя.")
		return
	}
	verified := formBool(req.FormValue("email_verified"))
	_, err = h.users.AdminUpdateUser(req.Context(), *actor, id, req.FormValue("nickname"), req.FormValue("status"), &verified, clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	if err == nil {
		role := strings.TrimSpace(req.FormValue("role"))
		if role != "" && !containsRole(req.FormValue("current_roles"), role) {
			err = h.users.AddRole(req.Context(), *actor, id, role, clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
		}
	}
	h.finishAdminMutation(w, req, "users", err)
}

func (h *Handler) adminDeleteUser(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok {
		return
	}
	id, err := pathInt64(req, "id")
	if err == nil {
		err = h.users.AdminDeleteUser(req.Context(), *actor, id, clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	}
	h.finishAdminMutation(w, req, "users", err)
}

func (h *Handler) adminCreateApp(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok || !h.parseForm(w, req) {
		return
	}
	_, err := h.users.CreateAdminApp(req.Context(), *actor, account.AdminApp{
		Slug: req.FormValue("slug"), Name: req.FormValue("name"), BundleID: req.FormValue("bundle_id"),
		DeveloperName: req.FormValue("developer_name"), Summary: req.FormValue("summary"), Description: req.FormValue("description"),
		WebsiteURL: req.FormValue("website_url"), SourceURL: req.FormValue("source_url"),
	}, req.FormValue("category_slug"), clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	h.finishAdminMutation(w, req, "apps", err)
}

func (h *Handler) adminUpdateApp(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok || !h.parseForm(w, req) {
		return
	}
	id, err := pathInt64(req, "id")
	if err == nil {
		_, err = h.users.UpdateAdminApp(req.Context(), *actor, id, account.AdminApp{
			Slug: req.FormValue("slug"), Name: req.FormValue("name"), BundleID: req.FormValue("bundle_id"),
			DeveloperName: req.FormValue("developer_name"), Summary: req.FormValue("summary"), Description: req.FormValue("description"),
			WebsiteURL: req.FormValue("website_url"), SourceURL: req.FormValue("source_url"), ModerationStatus: req.FormValue("moderation_status"),
		}, clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	}
	h.finishAdminMutation(w, req, "apps", err)
}

func (h *Handler) adminDeleteApp(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok {
		return
	}
	id, err := pathInt64(req, "id")
	if err == nil {
		err = h.users.DeleteAdminApp(req.Context(), *actor, id, clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	}
	h.finishAdminMutation(w, req, "apps", err)
}

func (h *Handler) adminCreateArtifact(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok || !h.parseForm(w, req) {
		return
	}
	versionID, _ := strconv.ParseInt(req.FormValue("version_id"), 10, 64)
	sizeBytes, _ := strconv.ParseInt(req.FormValue("size_bytes"), 10, 64)
	_, err := h.users.CreateAdminArtifact(req.Context(), *actor, account.AdminArtifact{
		AppVersionID: versionID, FileName: req.FormValue("file_name"), PackageType: req.FormValue("package_type"),
		SourceType: req.FormValue("source_type"), StoragePath: req.FormValue("storage_path"), PrimaryDownloadURL: req.FormValue("download_url"),
		SizeBytes: sizeBytes, SHA256: req.FormValue("sha256"), MinOS: req.FormValue("min_os"),
		MaxSupportedOS: req.FormValue("max_supported_os"), MaxTestedOS: req.FormValue("max_tested_os"),
		Architectures: csvList(req.FormValue("architectures")), RequiresRosetta: formBool(req.FormValue("requires_rosetta")),
		RequiresJava: formBool(req.FormValue("requires_java")), InstallNotes: req.FormValue("install_notes"),
	}, clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	h.finishAdminMutation(w, req, "uploads", err)
}

func (h *Handler) adminUpdateArtifact(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok || !h.parseForm(w, req) {
		return
	}
	id, err := pathInt64(req, "id")
	if err == nil {
		sizeBytes := int64(-1)
		if raw := strings.TrimSpace(req.FormValue("size_bytes")); raw != "" {
			sizeBytes, _ = strconv.ParseInt(raw, 10, 64)
		}
		_, err = h.users.UpdateAdminArtifact(req.Context(), *actor, id, account.AdminArtifact{
			FileName: req.FormValue("file_name"), PackageType: req.FormValue("package_type"), SourceType: req.FormValue("source_type"),
			StoragePath: req.FormValue("storage_path"), PrimaryDownloadURL: req.FormValue("download_url"),
			SizeBytes: sizeBytes, SHA256: req.FormValue("sha256"), MinOS: req.FormValue("min_os"),
			MaxSupportedOS: req.FormValue("max_supported_os"), MaxTestedOS: req.FormValue("max_tested_os"),
			Architectures: csvList(req.FormValue("architectures")), RequiresRosetta: formBool(req.FormValue("requires_rosetta")),
			RequiresJava: formBool(req.FormValue("requires_java")), InstallNotes: req.FormValue("install_notes"),
			ModerationStatus: req.FormValue("moderation_status"),
		}, clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	}
	h.finishAdminMutation(w, req, "uploads", err)
}

func (h *Handler) adminDeleteArtifact(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok {
		return
	}
	id, err := pathInt64(req, "id")
	if err == nil {
		err = h.users.DeleteAdminArtifact(req.Context(), *actor, id, clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	}
	h.finishAdminMutation(w, req, "uploads", err)
}

func (h *Handler) adminCreateReview(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok || !h.parseForm(w, req) {
		return
	}
	userID, _ := strconv.ParseInt(req.FormValue("user_id"), 10, 64)
	rating, _ := strconv.Atoi(req.FormValue("rating"))
	_, err := h.users.AdminCreateReview(req.Context(), *actor, req.FormValue("app_slug"), userID, rating, req.FormValue("title"), req.FormValue("body"), clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	h.finishAdminMutation(w, req, "reviews", err)
}

func (h *Handler) adminUpdateReview(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok || !h.parseForm(w, req) {
		return
	}
	id, err := pathInt64(req, "id")
	rating, _ := strconv.Atoi(req.FormValue("rating"))
	if err == nil {
		err = h.users.AdminUpdateReview(req.Context(), *actor, id, rating, req.FormValue("title"), req.FormValue("body"), clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	}
	h.finishAdminMutation(w, req, "reviews", err)
}

func (h *Handler) adminDeleteReview(w http.ResponseWriter, req *http.Request) {
	actor, _, ok := h.requireRoles(w, req, "admin")
	if !ok {
		return
	}
	id, err := pathInt64(req, "id")
	if err == nil {
		err = h.users.AdminDeleteReview(req.Context(), *actor, id, clientIP(req, h.cfg.TrustProxyHeaders), req.UserAgent())
	}
	h.finishAdminMutation(w, req, "reviews", err)
}

func (h *Handler) finishAdminMutation(w http.ResponseWriter, req *http.Request, tab string, err error) {
	if err != nil {
		h.renderError(w, req, http.StatusBadRequest, "Операция не выполнена: "+err.Error())
		return
	}
	http.Redirect(w, req, "/admin/system?tab="+tab+"&notice=saved", http.StatusSeeOther)
}

func pathInt64(req *http.Request, key string) (int64, error) {
	return strconv.ParseInt(req.PathValue(key), 10, 64)
}

func csvList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func containsRole(csv, role string) bool {
	for _, value := range csvList(csv) {
		if value == role {
			return true
		}
	}
	return false
}
