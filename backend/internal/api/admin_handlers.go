package api

import (
	"net/http"

	"legacystore/backend/internal/account"
)

func (r *Router) adminDashboard(w http.ResponseWriter, req *http.Request) {
	if _, ok := r.requireAdmin(w, req); !ok {
		return
	}
	dashboard, err := r.users.Dashboard(req.Context())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"dashboard": dashboard})
}

func (r *Router) adminUsers(w http.ResponseWriter, req *http.Request) {
	if _, ok := r.requireAdmin(w, req, "admin"); !ok {
		return
	}
	users, err := r.users.ListUsers(req.Context(), intParam(req.URL.Query().Get("limit"), 50))
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (r *Router) adminUpdateUser(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "admin")
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminUserPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	user, err := r.users.AdminUpdateUser(req.Context(), *actor, id, payload.Nickname, payload.Status, payload.EmailVerified, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (r *Router) adminAddRole(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "admin")
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminUserPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if err := r.users.AddRole(req.Context(), *actor, id, payload.Role, clientIP(req), req.UserAgent()); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) adminRemoveRole(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "admin")
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.RemoveRole(req.Context(), *actor, id, req.PathValue("role"), clientIP(req), req.UserAgent()); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) adminApps(w http.ResponseWriter, req *http.Request) {
	if _, ok := r.requireAdmin(w, req); !ok {
		return
	}
	apps, err := r.users.ListAdminApps(req.Context(), req.URL.Query().Get("status"), intParam(req.URL.Query().Get("limit"), 50))
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": apps})
}

func (r *Router) adminCreateApp(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "uploader", "trusted", "moder", "admin")
	if !ok {
		return
	}
	var payload adminAppPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	app, err := r.users.CreateAdminApp(req.Context(), *actor, account.AdminApp{
		Slug:             payload.Slug,
		Name:             payload.Name,
		BundleID:         payload.BundleID,
		DeveloperName:    payload.DeveloperName,
		Summary:          payload.Summary,
		Description:      payload.Description,
		WebsiteURL:       payload.WebsiteURL,
		SourceURL:        payload.SourceURL,
		ModerationStatus: payload.ModerationStatus,
	}, payload.CategorySlug, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"app": app})
}

func (r *Router) adminUpdateApp(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req)
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload adminAppPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	app, err := r.users.UpdateAdminApp(req.Context(), *actor, id, account.AdminApp{
		Slug:             payload.Slug,
		Name:             payload.Name,
		BundleID:         payload.BundleID,
		DeveloperName:    payload.DeveloperName,
		Summary:          payload.Summary,
		Description:      payload.Description,
		ModerationStatus: payload.ModerationStatus,
	}, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"app": app})
}

func (r *Router) adminDeleteApp(w http.ResponseWriter, req *http.Request) {
	actor, ok := r.requireAdmin(w, req, "admin")
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.DeleteAdminApp(req.Context(), *actor, id, clientIP(req), req.UserAgent()); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) adminModeration(w http.ResponseWriter, req *http.Request) {
	if _, ok := r.requireAdmin(w, req); !ok {
		return
	}
	items, err := r.users.ListModeration(req.Context(), req.URL.Query().Get("status"))
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (r *Router) adminApprove(w http.ResponseWriter, req *http.Request) {
	r.adminModerate(w, req, true)
}

func (r *Router) adminReject(w http.ResponseWriter, req *http.Request) {
	r.adminModerate(w, req, false)
}

func (r *Router) adminModerate(w http.ResponseWriter, req *http.Request, approve bool) {
	actor, ok := r.requireAdmin(w, req)
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload moderationPayload
	if req.ContentLength > 0 {
		if !decodeJSON(w, req, &payload) {
			return
		}
	}
	if err := r.users.Moderate(req.Context(), *actor, id, approve, payload.Comment, clientIP(req), req.UserAgent()); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) adminAuditLog(w http.ResponseWriter, req *http.Request) {
	if _, ok := r.requireAdmin(w, req, "admin"); !ok {
		return
	}
	items, err := r.users.AuditLog(req.Context())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
