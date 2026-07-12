package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"legacystore/backend/internal/account"
)

type authPayload struct {
	Email            string   `json:"email"`
	Nickname         string   `json:"nickname"`
	Password         string   `json:"password"`
	TOTPCode         string   `json:"totp_code"`
	RememberMe       bool     `json:"remember_me"`
	DeviceName       string   `json:"device_name"`
	DeviceIdentifier string   `json:"device_identifier"`
	Scopes           []string `json:"scopes"`
	Name             string   `json:"name"`
}

type profilePayload struct {
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
}

type reviewPayload struct {
	Rating int    `json:"rating"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type adminUserPayload struct {
	Nickname      string `json:"nickname"`
	Status        string `json:"status"`
	EmailVerified *bool  `json:"email_verified"`
	Role          string `json:"role"`
}

type adminAppPayload struct {
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	BundleID         string `json:"bundle_id"`
	DeveloperName    string `json:"developer_name"`
	Summary          string `json:"summary"`
	Description      string `json:"description"`
	CategorySlug     string `json:"category_slug"`
	ModerationStatus string `json:"moderation_status"`
}

type moderationPayload struct {
	Comment string `json:"comment"`
}

func (r *Router) register(w http.ResponseWriter, req *http.Request) {
	if !r.requireSecure(w, req) || !r.requireUsers(w) {
		return
	}
	var payload authPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	result, err := r.users.Register(req.Context(), payload.Email, payload.Nickname, payload.Password, payload.RememberMe, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	setSessionCookie(w, req, result.Token, result.ExpiresAt, payload.RememberMe)
	writeJSON(w, http.StatusOK, result)
}

func (r *Router) login(w http.ResponseWriter, req *http.Request) {
	if !r.requireSecure(w, req) || !r.requireUsers(w) {
		return
	}
	var payload authPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	result, err := r.users.Login(req.Context(), payload.Email, payload.Password, payload.TOTPCode, payload.RememberMe, clientIP(req), req.UserAgent())
	if err != nil {
		if errors.Is(err, account.ErrTwoFactorRequired) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "two_factor_required"})
			return
		}
		writeAccountError(w, err)
		return
	}
	setSessionCookie(w, req, result.Token, result.ExpiresAt, payload.RememberMe)
	writeJSON(w, http.StatusOK, result)
}

func (r *Router) legacyLogin(w http.ResponseWriter, req *http.Request) {
	if !r.requireSecure(w, req) || !r.requireUsers(w) {
		return
	}
	var payload authPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	result, err := r.users.LegacyLogin(req.Context(), payload.Email, payload.Password, payload.DeviceIdentifier, payload.DeviceName, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	setSessionCookie(w, req, result.Token, result.ExpiresAt, true)
	writeJSON(w, http.StatusOK, result)
}

func (r *Router) logout(w http.ResponseWriter, req *http.Request) {
	token := sessionToken(req)
	if r.users != nil {
		_ = r.users.Logout(req.Context(), token)
	}
	clearSessionCookie(w, req)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) refresh(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user})
}

func (r *Router) me(w http.ResponseWriter, req *http.Request) {
	user, session, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "session": session})
}

func (r *Router) updateMe(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	var payload profilePayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	updated, err := r.users.UpdateProfile(req.Context(), user.ID, payload.Nickname, payload.AvatarURL)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": updated})
}

func (r *Router) setup2FA(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	setup, err := r.users.Setup2FA(req.Context(), *user)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, setup)
}

func (r *Router) verify2FA(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	var payload authPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if err := r.users.Verify2FA(req.Context(), user.ID, payload.TOTPCode); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

func (r *Router) disable2FA(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	var payload authPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if err := r.users.Disable2FA(req.Context(), user.ID, payload.TOTPCode); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (r *Router) sessions(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	sessions, err := r.users.ListSessions(req.Context(), user.ID)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

func (r *Router) revokeSession(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.RevokeSession(req.Context(), user.ID, id); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (r *Router) legacyPasswords(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	items, err := r.users.ListLegacyPasswords(req.Context(), user.ID)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"legacy_passwords": items})
}

func (r *Router) createLegacyPassword(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	var payload authPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	item, token, err := r.users.CreateLegacyPassword(req.Context(), user.ID, payload.Name, payload.Scopes)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"legacy_password": item, "token": token})
}

func (r *Router) resetLegacyPassword(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	item, token, err := r.users.ResetLegacyPassword(req.Context(), user.ID, id)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"legacy_password": item, "token": token})
}

func (r *Router) deleteLegacyPassword(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.DeleteLegacyPassword(req.Context(), user.ID, id); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (r *Router) devices(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	devices, err := r.users.ListLegacyDevices(req.Context(), user.ID)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": devices})
}

func (r *Router) revokeDevice(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.RevokeLegacyDevice(req.Context(), user.ID, id); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (r *Router) createReview(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	var payload reviewPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	review, err := r.users.CreateReview(req.Context(), *user, req.PathValue("slug"), payload.Rating, payload.Title, payload.Body, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": review})
}

func (r *Router) updateReview(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload reviewPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	review, err := r.users.UpdateReview(req.Context(), *user, id, payload.Rating, payload.Title, payload.Body, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"review": review})
}

func (r *Router) deleteReview(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.DeleteReview(req.Context(), *user, id, clientIP(req), req.UserAgent()); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (r *Router) likeReview(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.LikeReview(req.Context(), *user, id); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "liked"})
}

func (r *Router) unlikeReview(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	if err := r.users.UnlikeReview(req.Context(), user.ID, id); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unliked"})
}

func (r *Router) createReviewReply(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return
	}
	id, ok := pathID(w, req, "id")
	if !ok {
		return
	}
	var payload reviewPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	reply, err := r.users.CreateReviewReply(req.Context(), *user, id, payload.Body, clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reply": reply})
}

func (r *Router) requireAuth(w http.ResponseWriter, req *http.Request) (*account.User, *account.Session, bool) {
	if !r.requireSecure(w, req) || !r.requireUsers(w) {
		return nil, nil, false
	}
	user, session, err := r.users.UserBySession(req.Context(), sessionToken(req), clientIP(req), req.UserAgent())
	if err != nil {
		writeAccountError(w, err)
		return nil, nil, false
	}
	return user, session, true
}

func (r *Router) requireAdmin(w http.ResponseWriter, req *http.Request, roles ...string) (*account.User, bool) {
	user, _, ok := r.requireAuth(w, req)
	if !ok {
		return nil, false
	}
	if len(roles) == 0 {
		roles = []string{"admin", "moder"}
	}
	if !account.HasRole(*user, roles...) {
		writeError(w, http.StatusForbidden, "forbidden")
		return nil, false
	}
	return user, true
}

func (r *Router) requireSecure(w http.ResponseWriter, req *http.Request) bool {
	if !isSecureRequest(req) {
		writeError(w, http.StatusForbidden, "https_required")
		return false
	}
	return true
}

func (r *Router) requireUsers(w http.ResponseWriter) bool {
	if r.users == nil {
		writeError(w, http.StatusServiceUnavailable, "accounts_unavailable")
		return false
	}
	return true
}

func decodeJSON(w http.ResponseWriter, req *http.Request, dest any) bool {
	req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
	defer req.Body.Close()
	if err := json.NewDecoder(req.Body).Decode(dest); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json")
		return false
	}
	return true
}

func writeAccountError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, account.ErrInvalidCredential):
		writeError(w, http.StatusUnauthorized, "invalid_credentials")
	case errors.Is(err, account.ErrTwoFactorRequired):
		writeError(w, http.StatusUnauthorized, "two_factor_required")
	case errors.Is(err, account.ErrEmailExists):
		writeError(w, http.StatusConflict, "email_exists")
	case errors.Is(err, account.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, account.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, account.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found")
	default:
		writeError(w, http.StatusInternalServerError, "account_failed")
	}
}

func sessionToken(req *http.Request) string {
	if cookie, err := req.Cookie("legacystore_session"); err == nil {
		return cookie.Value
	}
	header := req.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	}
	return ""
}

func setSessionCookie(w http.ResponseWriter, req *http.Request, token string, expiresAt time.Time, remember bool) {
	cookie := &http.Cookie{
		Name:     "legacystore_session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(req),
	}
	if remember {
		cookie.Expires = expiresAt
		cookie.MaxAge = int(time.Until(expiresAt).Seconds())
	}
	http.SetCookie(w, cookie)
}

func clearSessionCookie(w http.ResponseWriter, req *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "legacystore_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(req),
	})
}

func pathID(w http.ResponseWriter, req *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(req.PathValue(name), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id")
		return 0, false
	}
	return id, true
}

func clientIP(req *http.Request) net.IP {
	if forwarded := req.Header.Get("X-Forwarded-For"); forwarded != "" {
		first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
		if ip := net.ParseIP(first); ip != nil {
			return ip
		}
	}
	if real := req.Header.Get("X-Real-IP"); real != "" {
		if ip := net.ParseIP(real); ip != nil {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}
