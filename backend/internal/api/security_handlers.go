package api

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"legacystore/backend/internal/account"
)

type loginSecurityPayload struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	TOTPCode     string `json:"totp_code"`
	RecoveryCode string `json:"recovery_code"`
	RememberMe   bool   `json:"remember_me"`
}

type sensitiveAccountPayload struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	NewEmail        string `json:"new_email"`
	TOTPCode        string `json:"totp_code"`
	RecoveryCode    string `json:"recovery_code"`
}

type recoveryRequestPayload struct {
	Email string `json:"email"`
}

type recoveryResetPayload struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (r *Router) loginV2(w http.ResponseWriter, req *http.Request) {
	if !r.requireSecure(w, req) || !r.requireUsers(w) {
		return
	}
	var payload loginSecurityPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	result, err := r.users.LoginWithSecondFactor(
		req.Context(), payload.Email, payload.Password, payload.TOTPCode, payload.RecoveryCode,
		payload.RememberMe, clientIP(req), req.UserAgent(),
	)
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

func (r *Router) setup2FAV2(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireWebAuth(w, req)
	if !ok {
		return
	}
	var payload sensitiveAccountPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if err := r.users.VerifyCurrentPassword(req.Context(), user.ID, payload.CurrentPassword); err != nil {
		writeAccountError(w, err)
		return
	}
	setup, err := r.users.Setup2FA(req.Context(), *user)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, setup)
}

func (r *Router) verify2FAV2(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireWebAuth(w, req)
	if !ok {
		return
	}
	var payload sensitiveAccountPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	codes, err := r.users.Confirm2FA(req.Context(), user.ID, payload.TOTPCode)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "enabled",
		"recovery_codes": codes,
	})
}

func (r *Router) disable2FAV2(w http.ResponseWriter, req *http.Request) {
	user, session, ok := r.requireWebAuth(w, req)
	if !ok {
		return
	}
	var payload sensitiveAccountPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if err := r.users.Disable2FAWithSecondFactor(
		req.Context(), user.ID, session.ID, payload.CurrentPassword, payload.TOTPCode, payload.RecoveryCode,
	); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (r *Router) regenerate2FARecoveryCodes(w http.ResponseWriter, req *http.Request) {
	user, _, ok := r.requireWebAuth(w, req)
	if !ok {
		return
	}
	var payload sensitiveAccountPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	codes, err := r.users.Regenerate2FARecoveryCodes(
		req.Context(), user.ID, payload.CurrentPassword, payload.TOTPCode, payload.RecoveryCode,
	)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

func (r *Router) changePassword(w http.ResponseWriter, req *http.Request) {
	user, session, ok := r.requireWebAuth(w, req)
	if !ok {
		return
	}
	var payload sensitiveAccountPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if err := r.users.ChangePassword(
		req.Context(), user.ID, session.ID, payload.CurrentPassword, payload.NewPassword, payload.TOTPCode, payload.RecoveryCode,
	); err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "password_changed"})
}

func (r *Router) changeEmail(w http.ResponseWriter, req *http.Request) {
	user, session, ok := r.requireWebAuth(w, req)
	if !ok {
		return
	}
	var payload sensitiveAccountPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	updated, err := r.users.ChangeEmail(
		req.Context(), user.ID, session.ID, payload.NewEmail, payload.CurrentPassword, payload.TOTPCode, payload.RecoveryCode,
	)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "email_changed", "user": updated})
}

func (r *Router) requestPasswordRecovery(w http.ResponseWriter, req *http.Request) {
	if !r.requireSecure(w, req) || !r.requireUsers(w) {
		return
	}
	if r.cfg.SMTPHost == "" && !r.cfg.RecoveryDebugToken {
		writeError(w, http.StatusServiceUnavailable, "recovery_delivery_unavailable")
		return
	}
	var payload recoveryRequestPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	token, found, err := r.users.BeginPasswordRecovery(req.Context(), payload.Email)
	if err != nil {
		writeAccountError(w, err)
		return
	}
	response := map[string]any{"status": "accepted"}
	if found {
		if r.cfg.SMTPHost != "" {
			if err := sendPasswordRecoveryEmail(r.cfg, strings.TrimSpace(payload.Email), token); err != nil {
				// Do not disclose whether the account exists through a different response.
				log.Printf("password recovery delivery failed: %v", err)
			}
		}
		if r.cfg.RecoveryDebugToken {
			response["recovery_token"] = token
		}
	}
	writeJSON(w, http.StatusAccepted, response)
}

func (r *Router) resetPasswordRecovery(w http.ResponseWriter, req *http.Request) {
	if !r.requireSecure(w, req) || !r.requireUsers(w) {
		return
	}
	var payload recoveryResetPayload
	if !decodeJSON(w, req, &payload) {
		return
	}
	if err := r.users.CompletePasswordRecovery(req.Context(), payload.Token, payload.NewPassword); err != nil {
		if errors.Is(err, account.ErrRecoveryTokenInvalid) {
			writeError(w, http.StatusUnauthorized, "invalid_recovery_token")
			return
		}
		writeAccountError(w, err)
		return
	}
	clearSessionCookie(w, req)
	writeJSON(w, http.StatusOK, map[string]string{"status": "password_reset"})
}

func (r *Router) requireWebAuth(w http.ResponseWriter, req *http.Request) (*account.User, *account.Session, bool) {
	user, session, ok := r.requireAuth(w, req)
	if !ok {
		return nil, nil, false
	}
	if session.AuthKind != "web" {
		writeError(w, http.StatusForbidden, "web_session_required")
		return nil, nil, false
	}
	return user, session, true
}
