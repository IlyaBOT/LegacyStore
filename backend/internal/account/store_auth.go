package account

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/lib/pq"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Register(ctx context.Context, email, nickname, password string, remember bool, ip net.IP, userAgent string) (*AuthResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	nickname = strings.TrimSpace(nickname)
	if email == "" || password == "" || len(password) < 8 {
		return nil, ErrInvalidCredential
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var user User
	err = tx.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, nickname, email_verified, status)
		VALUES ($1, $2, $3, false, 'active')
		RETURNING id, email, COALESCE(nickname, ''), COALESCE(avatar_url, ''), email_verified, two_factor_enabled, status, created_at::text, updated_at::text
	`, email, hash, nullEmpty(nickname)).Scan(
		&user.ID,
		&user.Email,
		&user.Nickname,
		&user.AvatarURL,
		&user.EmailVerified,
		&user.TwoFactorEnabled,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
		return nil, ErrEmailExists
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = 'user'
		ON CONFLICT DO NOTHING
	`, user.ID); err != nil {
		return nil, err
	}
	user.Roles = []string{"user"}

	token, expiresAt, err := createSession(ctx, tx, user.ID, remember, ip, userAgent)
	if err != nil {
		return nil, err
	}
	if err := auditTx(ctx, tx, user.ID, "auth.register", "user", intString(user.ID), ip, userAgent); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &AuthResult{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *Store) Login(ctx context.Context, email, password, totpCode string, remember bool, ip net.IP, userAgent string) (*AuthResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var user User
	var passwordHash string
	var totpSecret string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, COALESCE(nickname, ''), COALESCE(avatar_url, ''), email_verified, two_factor_enabled,
		       COALESCE(password_hash, ''), COALESCE(two_factor_secret, ''), status, created_at::text, updated_at::text
		FROM users
		WHERE lower(email) = $1
	`, email).Scan(
		&user.ID,
		&user.Email,
		&user.Nickname,
		&user.AvatarURL,
		&user.EmailVerified,
		&user.TwoFactorEnabled,
		&passwordHash,
		&totpSecret,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		_ = verifyPassword(dummyPasswordHash, password)
		return nil, ErrInvalidCredential
	}
	if err != nil {
		return nil, err
	}
	if user.Status != "active" || !verifyPassword(passwordHash, password) {
		return nil, ErrInvalidCredential
	}
	if user.TwoFactorEnabled && !verifyTOTP(totpSecret, totpCode, time.Now()) {
		return nil, ErrTwoFactorRequired
	}
	roles, err := s.roles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	token, expiresAt, err := createSession(ctx, tx, user.ID, remember, ip, userAgent)
	if err != nil {
		return nil, err
	}
	if err := auditTx(ctx, tx, user.ID, "auth.login", "user", intString(user.ID), ip, userAgent); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &AuthResult{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *Store) UserBySession(ctx context.Context, token string, ip net.IP, userAgent string) (*User, *Session, error) {
	if token == "" {
		return nil, nil, ErrUnauthorized
	}
	hash := tokenHash(token)
	var user User
	var session Session
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.email, COALESCE(u.nickname, ''), COALESCE(u.avatar_url, ''), u.email_verified,
		       u.two_factor_enabled, u.status, u.created_at::text, u.updated_at::text,
		       s.id, COALESCE(s.device_name, ''), COALESCE(s.ip_address::text, ''), COALESCE(s.user_agent, ''),
		       s.remember_me, s.auth_kind, s.scopes, COALESCE(s.legacy_password_id, 0), COALESCE(s.legacy_device_identifier, ''),
		       s.expires_at::text, s.last_seen_at::text, s.created_at::text
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.status = 'active'
	`, hash).Scan(
		&user.ID,
		&user.Email,
		&user.Nickname,
		&user.AvatarURL,
		&user.EmailVerified,
		&user.TwoFactorEnabled,
		&user.Status,
		&user.CreatedAt,
		&user.UpdatedAt,
		&session.ID,
		&session.DeviceName,
		&session.IPAddress,
		&session.UserAgent,
		&session.RememberMe,
		&session.AuthKind,
		pq.Array(&session.Scopes),
		&session.LegacyPasswordID,
		&session.LegacyDeviceIdentifier,
		&session.ExpiresAt,
		&session.LastSeenAt,
		&session.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, ErrUnauthorized
	}
	if err != nil {
		return nil, nil, err
	}
	roles, err := s.roles(ctx, user.ID)
	if err != nil {
		return nil, nil, err
	}
	user.Roles = roles
	_, _ = s.db.ExecContext(ctx, `
		UPDATE sessions
		SET last_seen_at = now(), ip_address = $2, user_agent = $3
		WHERE id = $1
	`, session.ID, nullableIP(ip), truncate(userAgent, 512))
	return &user, &session, nil
}

func (s *Store) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET revoked_at = now() WHERE token_hash = $1`, tokenHash(token))
	return err
}

func (s *Store) UpdateProfile(ctx context.Context, userID int64, nickname, avatarURL string) (*User, error) {
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET nickname = COALESCE(NULLIF($2, ''), nickname),
		    avatar_url = COALESCE(NULLIF($3, ''), avatar_url)
		WHERE id = $1
	`, userID, strings.TrimSpace(nickname), strings.TrimSpace(avatarURL))
	if err != nil {
		return nil, err
	}
	return s.UserByID(ctx, userID)
}

func (s *Store) UserByID(ctx context.Context, userID int64) (*User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, COALESCE(nickname, ''), COALESCE(avatar_url, ''), email_verified, two_factor_enabled, status, created_at::text, updated_at::text
		FROM users
		WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.Nickname, &user.AvatarURL, &user.EmailVerified, &user.TwoFactorEnabled, &user.Status, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	roles, err := s.roles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles
	return &user, nil
}

func (s *Store) Setup2FA(ctx context.Context, user User) (map[string]string, error) {
	secret, err := generateTOTPSecret()
	if err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET two_factor_secret = $2, two_factor_enabled = false, two_factor_confirmed_at = NULL
		WHERE id = $1
	`, user.ID, secret); err != nil {
		return nil, err
	}
	return map[string]string{
		"secret":      secret,
		"otpauth_url": totpURL(user.Email, secret),
	}, nil
}

func (s *Store) Verify2FA(ctx context.Context, userID int64, code string) error {
	var secret string
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(two_factor_secret, '') FROM users WHERE id = $1`, userID).Scan(&secret); err != nil {
		return err
	}
	if secret == "" || !verifyTOTP(secret, code, time.Now()) {
		return ErrInvalidCredential
	}
	_, err := s.db.ExecContext(ctx, `UPDATE users SET two_factor_enabled = true, two_factor_confirmed_at = now() WHERE id = $1`, userID)
	return err
}

func (s *Store) Disable2FA(ctx context.Context, userID int64, code string) error {
	var secret string
	var enabled bool
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(two_factor_secret, ''), two_factor_enabled FROM users WHERE id = $1`, userID).Scan(&secret, &enabled); err != nil {
		return err
	}
	if enabled && (secret == "" || !verifyTOTP(secret, code, time.Now())) {
		return ErrInvalidCredential
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE users
		SET two_factor_secret = NULL, two_factor_enabled = false, two_factor_confirmed_at = NULL
		WHERE id = $1
	`, userID)
	return err
}

func (s *Store) ListSessions(ctx context.Context, userID int64) ([]Session, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(device_name, ''), COALESCE(ip_address::text, ''), COALESCE(user_agent, ''), remember_me,
		       auth_kind, scopes, COALESCE(legacy_password_id, 0), COALESCE(legacy_device_identifier, ''),
		       expires_at::text, last_seen_at::text, created_at::text
		FROM sessions
		WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
		ORDER BY last_seen_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var session Session
		if err := rows.Scan(
			&session.ID,
			&session.DeviceName,
			&session.IPAddress,
			&session.UserAgent,
			&session.RememberMe,
			&session.AuthKind,
			pq.Array(&session.Scopes),
			&session.LegacyPasswordID,
			&session.LegacyDeviceIdentifier,
			&session.ExpiresAt,
			&session.LastSeenAt,
			&session.CreatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (s *Store) RevokeSession(ctx context.Context, userID, sessionID int64) error {
	result, err := s.db.ExecContext(ctx, `UPDATE sessions SET revoked_at = now() WHERE id = $1 AND user_id = $2`, sessionID, userID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func createSession(ctx context.Context, tx *sql.Tx, userID int64, remember bool, ip net.IP, userAgent string) (string, time.Time, error) {
	return createScopedSession(ctx, tx, userID, remember, ip, userAgent, "web", []string{}, 0, "")
}

func createScopedSession(ctx context.Context, tx *sql.Tx, userID int64, remember bool, ip net.IP, userAgent, authKind string, scopes []string, legacyPasswordID int64, legacyDeviceIdentifier string) (string, time.Time, error) {
	token, err := randomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	duration := 12 * time.Hour
	if remember {
		duration = 30 * 24 * time.Hour
	}
	expiresAt := time.Now().UTC().Add(duration)
	var legacyID any
	if legacyPasswordID > 0 {
		legacyID = legacyPasswordID
	}
	if scopes == nil {
		scopes = []string{}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO sessions (
			user_id, token_hash, remember_me, device_name, ip_address, user_agent, expires_at,
			auth_kind, scopes, legacy_password_id, legacy_device_identifier
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, ''))
	`, userID, tokenHash(token), remember, deviceName(userAgent), nullableIP(ip), truncate(userAgent, 512), expiresAt,
		authKind, pq.Array(scopes), legacyID, strings.TrimSpace(legacyDeviceIdentifier))
	return token, expiresAt, err
}

func (s *Store) roles(ctx context.Context, userID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.name
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func HasRole(user User, roles ...string) bool {
	for _, have := range user.Roles {
		for _, want := range roles {
			if have == want {
				return true
			}
		}
	}
	return false
}

func SessionHasScope(session Session, scope string) bool {
	if session.AuthKind == "web" {
		return true
	}
	for _, have := range session.Scopes {
		if have == scope {
			return true
		}
	}
	return false
}

func nullEmpty(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableIP(ip net.IP) any {
	if ip == nil {
		return nil
	}
	return ip.String()
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func deviceName(userAgent string) string {
	if strings.TrimSpace(userAgent) == "" {
		return "Browser"
	}
	return truncate(userAgent, 80)
}
