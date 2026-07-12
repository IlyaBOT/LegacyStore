package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"legacystore/backend/internal/account"
	"legacystore/backend/internal/catalog"
	"legacystore/backend/internal/compatibility"
	"legacystore/backend/internal/config"
)

type Router struct {
	mux   *http.ServeMux
	cfg   config.Config
	store *catalog.Store
	users *account.Store
}

func NewRouter(cfg config.Config, store *catalog.Store, users *account.Store) http.Handler {
	r := &Router{
		mux:   http.NewServeMux(),
		cfg:   cfg,
		store: store,
		users: users,
	}

	r.mux.HandleFunc("GET /healthz", r.health)
	r.mux.HandleFunc("GET /api/v1/bootstrap", r.bootstrap)
	r.mux.HandleFunc("GET /api/v1/categories", r.categories)
	r.mux.HandleFunc("GET /api/v1/apps", r.apps)
	r.mux.HandleFunc("GET /api/v1/apps/{slug}", r.appDetail)
	r.mux.HandleFunc("GET /api/v1/apps/{slug}/versions", r.appVersions)
	r.mux.HandleFunc("GET /api/v1/apps/{slug}/reviews", r.appReviews)
	r.mux.HandleFunc("GET /api/v1/search", r.search)
	r.mux.HandleFunc("GET /api/v1/download/{artifact_id}", r.download)
	r.mux.HandleFunc("POST /api/v1/auth/login", r.login)
	r.mux.HandleFunc("POST /api/v1/auth/register", r.register)
	r.mux.HandleFunc("POST /api/v1/auth/logout", r.logout)
	r.mux.HandleFunc("POST /api/v1/auth/refresh", r.refresh)
	r.mux.HandleFunc("POST /api/v1/auth/2fa/setup", r.setup2FA)
	r.mux.HandleFunc("POST /api/v1/auth/2fa/verify", r.verify2FA)
	r.mux.HandleFunc("POST /api/v1/auth/2fa/disable", r.disable2FA)
	r.mux.HandleFunc("POST /api/v1/auth/oauth/google", r.authNotAvailable)
	r.mux.HandleFunc("POST /api/v1/auth/oauth/apple", r.authNotAvailable)
	r.mux.HandleFunc("POST /api/v1/auth/legacy/login", r.legacyLogin)
	r.mux.HandleFunc("GET /api/v1/me", r.me)
	r.mux.HandleFunc("PATCH /api/v1/me", r.updateMe)
	r.mux.HandleFunc("POST /api/v1/me/avatar", r.updateMe)
	r.mux.HandleFunc("GET /api/v1/me/sessions", r.sessions)
	r.mux.HandleFunc("DELETE /api/v1/me/sessions/{id}", r.revokeSession)
	r.mux.HandleFunc("GET /api/v1/me/legacy-passwords", r.legacyPasswords)
	r.mux.HandleFunc("POST /api/v1/me/legacy-passwords", r.createLegacyPassword)
	r.mux.HandleFunc("POST /api/v1/me/legacy-passwords/{id}/reset", r.resetLegacyPassword)
	r.mux.HandleFunc("DELETE /api/v1/me/legacy-passwords/{id}", r.deleteLegacyPassword)
	r.mux.HandleFunc("GET /api/v1/me/devices", r.devices)
	r.mux.HandleFunc("DELETE /api/v1/me/devices/{id}", r.revokeDevice)
	r.mux.HandleFunc("POST /api/v1/apps/{slug}/reviews", r.createReview)
	r.mux.HandleFunc("PATCH /api/v1/reviews/{id}", r.updateReview)
	r.mux.HandleFunc("DELETE /api/v1/reviews/{id}", r.deleteReview)
	r.mux.HandleFunc("POST /api/v1/reviews/{id}/like", r.likeReview)
	r.mux.HandleFunc("DELETE /api/v1/reviews/{id}/like", r.unlikeReview)
	r.mux.HandleFunc("POST /api/v1/reviews/{id}/replies", r.createReviewReply)
	r.mux.HandleFunc("GET /api/v1/admin/dashboard", r.adminDashboard)
	r.mux.HandleFunc("GET /api/v1/admin/users", r.adminUsers)
	r.mux.HandleFunc("PATCH /api/v1/admin/users/{id}", r.adminUpdateUser)
	r.mux.HandleFunc("POST /api/v1/admin/users/{id}/roles", r.adminAddRole)
	r.mux.HandleFunc("DELETE /api/v1/admin/users/{id}/roles/{role}", r.adminRemoveRole)
	r.mux.HandleFunc("GET /api/v1/admin/apps", r.adminApps)
	r.mux.HandleFunc("POST /api/v1/admin/apps", r.adminCreateApp)
	r.mux.HandleFunc("PATCH /api/v1/admin/apps/{id}", r.adminUpdateApp)
	r.mux.HandleFunc("DELETE /api/v1/admin/apps/{id}", r.adminDeleteApp)
	r.mux.HandleFunc("GET /api/v1/admin/moderation", r.adminModeration)
	r.mux.HandleFunc("POST /api/v1/admin/moderation/{id}/approve", r.adminApprove)
	r.mux.HandleFunc("POST /api/v1/admin/moderation/{id}/reject", r.adminReject)
	r.mux.HandleFunc("GET /api/v1/admin/audit-log", r.adminAuditLog)
	r.mux.HandleFunc("/", r.notFound)

	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func (r *Router) health(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) notFound(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
}

func (r *Router) categories(w http.ResponseWriter, req *http.Request) {
	if !r.requireStore(w) {
		return
	}
	categories, err := r.store.Categories(req.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "categories_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": categories})
}

func (r *Router) apps(w http.ResponseWriter, req *http.Request) {
	if !r.requireStore(w) {
		return
	}
	apps, err := r.store.Apps(req.Context(), r.filters(req, false))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "apps_failed")
		return
	}
	filters := r.filters(req, false)
	writeJSON(w, http.StatusOK, map[string]any{
		"page":  filters.Page,
		"limit": filters.Limit,
		"apps":  apps,
	})
}

func (r *Router) appDetail(w http.ResponseWriter, req *http.Request) {
	if !r.requireStore(w) {
		return
	}
	app, err := r.store.AppBySlug(req.Context(), req.PathValue("slug"), targetFromRequest(req))
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "app_detail_failed")
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (r *Router) appVersions(w http.ResponseWriter, req *http.Request) {
	if !r.requireStore(w) {
		return
	}
	versions, err := r.store.Versions(req.Context(), req.PathValue("slug"), targetFromRequest(req))
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "versions_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (r *Router) appReviews(w http.ResponseWriter, req *http.Request) {
	if !r.requireStore(w) {
		return
	}
	reviews, err := r.store.Reviews(req.Context(), req.PathValue("slug"))
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "app_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reviews_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": reviews})
}

func (r *Router) search(w http.ResponseWriter, req *http.Request) {
	if !r.requireStore(w) {
		return
	}
	filters := r.filters(req, true)
	results, err := r.store.Apps(req.Context(), filters)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search_failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"page":    filters.Page,
		"limit":   filters.Limit,
		"query":   filters.Query,
		"results": results,
	})
}

func (r *Router) download(w http.ResponseWriter, req *http.Request) {
	if !r.requireStore(w) {
		return
	}
	artifactID, err := strconv.ParseInt(req.PathValue("artifact_id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_artifact_id")
		return
	}
	metadata, err := r.store.Download(req.Context(), artifactID, targetFromRequest(req))
	if errors.Is(err, catalog.ErrNotFound) {
		writeError(w, http.StatusNotFound, "artifact_not_found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "download_metadata_failed")
		return
	}
	writeJSON(w, http.StatusOK, metadata)
}

func (r *Router) invalidLogin(w http.ResponseWriter, req *http.Request) {
	if !isSecureRequest(req) {
		writeError(w, http.StatusForbidden, "https_required")
		return
	}
	writeError(w, http.StatusUnauthorized, "invalid_credentials")
}

func (r *Router) authNotAvailable(w http.ResponseWriter, req *http.Request) {
	if !isSecureRequest(req) {
		writeError(w, http.StatusForbidden, "https_required")
		return
	}
	writeError(w, http.StatusNotImplemented, "auth_not_implemented")
}

func (r *Router) unauthorized(w http.ResponseWriter, req *http.Request) {
	writeError(w, http.StatusUnauthorized, "unauthorized")
}

func (r *Router) requireStore(w http.ResponseWriter) bool {
	if r.store == nil {
		writeError(w, http.StatusServiceUnavailable, "database_unavailable")
		return false
	}
	return true
}

func (r *Router) filters(req *http.Request, useSearchQuery bool) catalog.Filters {
	values := req.URL.Query()
	filters := catalog.Filters{
		Page:           intParam(values.Get("page"), 1),
		Limit:          intParam(values.Get("limit"), 24),
		Category:       values.Get("category"),
		Target:         targetFromRequest(req),
		CompatibleOnly: boolParam(values.Get("compatible")),
	}
	if useSearchQuery {
		filters.Query = values.Get("q")
	}
	return filters
}

func targetFromRequest(req *http.Request) compatibility.Target {
	values := req.URL.Query()
	return compatibility.Target{
		OSVersion: values.Get("os"),
		Arch:      values.Get("arch"),
	}
}

func intParam(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func boolParam(raw string) bool {
	return raw == "1" || raw == "true" || raw == "yes"
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}

func isSecureRequest(req *http.Request) bool {
	return req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https"
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	encoder := json.NewEncoder(w)
	if err := encoder.Encode(payload); err != nil {
		http.Error(w, `{"error":"json_encode_failed"}`, http.StatusInternalServerError)
	}
}
