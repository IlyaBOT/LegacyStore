package account

import (
	"context"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"strings"
	"time"

	"github.com/lib/pq"
)

var ErrRecoveryTokenInvalid = errors.New("invalid recovery token")

const (
	passwordRecoveryTTL = 30 * time.Minute
	recoveryCodeCount   = 10
)

func (s *Store) LoginWithSecondFactor(ctx context.Context, email, password, totpCode, recoveryCode string, remember bool, ip net.IP, userAgent string) (*AuthResult, error) {
	email = normalizeEmail(email)
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if user.TwoFactorEnabled {
		if err := verifySecondFactorTx(ctx, tx, user.ID, totpSecret, totpCode, recoveryCode, true); err != nil {
			return nil, err
		}
	}

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

	roles, err := s.roles(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	user.Roles = roles
	return &AuthResult{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *Store) VerifyCurrentPassword(ctx context.Context, userID int64, password string) error {
	var passwordHash string
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(password_hash, '') FROM users WHERE id = $1`, userID).Scan(&passwordHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}
	if !verifyPassword(passwordHash, password) {
		return ErrInvalidCredential
	}
	return nil
}

func (s *Store) Confirm2FA(ctx context.Context, userID int64, code string) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var secret string
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(two_factor_secret, '') FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&secret); err != nil {
		return nil, err
	}
	if secret == "" || !verifyTOTP(secret, code, time.Now()) {
		return nil, ErrInvalidCredential
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET two_factor_enabled = true, two_factor_confirmed_at = now()
		WHERE id = $1
	`, userID); err != nil {
		return nil, err
	}
	codes, err := regenerateRecoveryCodesTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *Store) Disable2FAWithSecondFactor(ctx context.Context, userID, currentSessionID int64, password, totpCode, recoveryCode string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var passwordHash, secret string
	var enabled bool
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(password_hash, ''), COALESCE(two_factor_secret, ''), two_factor_enabled
		FROM users WHERE id = $1 FOR UPDATE
	`, userID).Scan(&passwordHash, &secret, &enabled); err != nil {
		return err
	}
	if !verifyPassword(passwordHash, password) {
		return ErrInvalidCredential
	}
	if enabled {
		if err := verifySecondFactorTx(ctx, tx, userID, secret, totpCode, recoveryCode, true); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET two_factor_secret = NULL, two_factor_enabled = false, two_factor_confirmed_at = NULL
		WHERE id = $1
	`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM two_factor_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = now()
		WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL
	`, userID, currentSessionID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Regenerate2FARecoveryCodes(ctx context.Context, userID int64, password, totpCode, recoveryCode string) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var passwordHash, secret string
	var enabled bool
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(password_hash, ''), COALESCE(two_factor_secret, ''), two_factor_enabled
		FROM users WHERE id = $1 FOR UPDATE
	`, userID).Scan(&passwordHash, &secret, &enabled); err != nil {
		return nil, err
	}
	if !verifyPassword(passwordHash, password) || !enabled {
		return nil, ErrInvalidCredential
	}
	if err := verifySecondFactorTx(ctx, tx, userID, secret, totpCode, recoveryCode, true); err != nil {
		return nil, err
	}
	codes, err := regenerateRecoveryCodesTx(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *Store) ChangePassword(ctx context.Context, userID, currentSessionID int64, currentPassword, newPassword, totpCode, recoveryCode string) error {
	if len(newPassword) < 8 || len(newPassword) > 1024 {
		return ErrInvalidCredential
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var passwordHash, secret string
	var enabled bool
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(password_hash, ''), COALESCE(two_factor_secret, ''), two_factor_enabled
		FROM users WHERE id = $1 FOR UPDATE
	`, userID).Scan(&passwordHash, &secret, &enabled); err != nil {
		return err
	}
	if !verifyPassword(passwordHash, currentPassword) {
		return ErrInvalidCredential
	}
	if enabled {
		if err := verifySecondFactorTx(ctx, tx, userID, secret, totpCode, recoveryCode, true); err != nil {
			return err
		}
	}
	newHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET password_hash = $2, password_changed_at = now() WHERE id = $1
	`, userID, newHash); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = now()
		WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL
	`, userID, currentSessionID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM password_recovery_tokens WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ChangeEmail(ctx context.Context, userID, currentSessionID int64, newEmail, currentPassword, totpCode, recoveryCode string) (*User, error) {
	newEmail = normalizeEmail(newEmail)
	if !validEmail(newEmail) {
		return nil, ErrInvalidCredential
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var passwordHash, secret string
	var enabled bool
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(password_hash, ''), COALESCE(two_factor_secret, ''), two_factor_enabled
		FROM users WHERE id = $1 FOR UPDATE
	`, userID).Scan(&passwordHash, &secret, &enabled); err != nil {
		return nil, err
	}
	if !verifyPassword(passwordHash, currentPassword) {
		return nil, ErrInvalidCredential
	}
	if enabled {
		if err := verifySecondFactorTx(ctx, tx, userID, secret, totpCode, recoveryCode, true); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET email = $2, email_verified = false, email_changed_at = now()
		WHERE id = $1
	`, userID, newEmail); err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, ErrEmailExists
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sessions SET revoked_at = now()
		WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL
	`, userID, currentSessionID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.UserByID(ctx, userID)
}

func (s *Store) BeginPasswordRecovery(ctx context.Context, email string) (string, bool, error) {
	email = normalizeEmail(email)
	var userID int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT id FROM users WHERE lower(email) = $1 AND status = 'active'
	`, email).Scan(&userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	token, err := randomToken()
	if err != nil {
		return "", false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM password_recovery_tokens WHERE user_id = $1 AND used_at IS NULL`, userID); err != nil {
		return "", false, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO password_recovery_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHash(token), time.Now().UTC().Add(passwordRecoveryTTL)); err != nil {
		return "", false, err
	}
	if err := tx.Commit(); err != nil {
		return "", false, err
	}
	return token, true, nil
}

func (s *Store) CompletePasswordRecovery(ctx context.Context, token, newPassword string) error {
	if strings.TrimSpace(token) == "" || len(newPassword) < 8 || len(newPassword) > 1024 {
		return ErrRecoveryTokenInvalid
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var tokenID, userID int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id, user_id
		FROM password_recovery_tokens
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > now()
		FOR UPDATE
	`, tokenHash(token)).Scan(&tokenID, &userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRecoveryTokenInvalid
		}
		return err
	}
	newHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET password_hash = $2, password_changed_at = now() WHERE id = $1
	`, userID, newHash); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE password_recovery_tokens SET used_at = now() WHERE id = $1`, tokenID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) EnsureAdmin(ctx context.Context, email, password, nickname string) error {
	email = normalizeEmail(email)
	nickname = strings.TrimSpace(nickname)
	if !validEmail(email) || len(password) < 12 {
		return fmt.Errorf("admin provisioning requires a valid email and a password of at least 12 characters")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('legacystore.ensure_admin'))`); err != nil {
		return err
	}

	var existingID int64
	var existingEmail string
	err = tx.QueryRowContext(ctx, `
		SELECT u.id, u.email
		FROM user_roles ur
		JOIN users u ON u.id = ur.user_id
		JOIN roles r ON r.id = ur.role_id
		WHERE r.name = 'admin'
		LIMIT 1
	`).Scan(&existingID, &existingEmail)
	if err == nil {
		if normalizeEmail(existingEmail) != email {
			return fmt.Errorf("admin role is already provisioned to %s", existingEmail)
		}
		return tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	var userID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE lower(email) = $1 FOR UPDATE`, email).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO users (email, password_hash, nickname, email_verified, status)
			VALUES ($1, $2, NULLIF($3, ''), true, 'active')
			RETURNING id
		`, email, hash, nickname).Scan(&userID)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE users
			SET password_hash = $2, nickname = COALESCE(NULLIF($3, ''), nickname), email_verified = true, status = 'active'
			WHERE id = $1
		`, userID, hash, nickname); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = 'user'
		ON CONFLICT DO NOTHING
	`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = 'admin'
	`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) HasAdmin(ctx context.Context) (bool, error) {
	var exists bool
	err := s.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE r.name = 'admin'
		)
	`).Scan(&exists)
	return exists, err
}

func verifySecondFactorTx(ctx context.Context, tx *sql.Tx, userID int64, secret, totpCode, recoveryCode string, consumeRecovery bool) error {
	if secret != "" && verifyTOTP(secret, strings.TrimSpace(totpCode), time.Now()) {
		return nil
	}
	normalized := normalizeRecoveryCode(recoveryCode)
	if normalized == "" {
		return ErrTwoFactorRequired
	}
	if consumeRecovery {
		result, err := tx.ExecContext(ctx, `
			UPDATE two_factor_recovery_codes
			SET used_at = now()
			WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL
		`, userID, tokenHash(normalized))
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n == 1 {
			return nil
		}
	} else {
		var exists bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM two_factor_recovery_codes
				WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL
			)
		`, userID, tokenHash(normalized)).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}
	}
	return ErrTwoFactorRequired
}

func regenerateRecoveryCodesTx(ctx context.Context, tx *sql.Tx, userID int64) ([]string, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM two_factor_recovery_codes WHERE user_id = $1`, userID); err != nil {
		return nil, err
	}
	codes := make([]string, 0, recoveryCodeCount)
	for i := 0; i < recoveryCodeCount; i++ {
		code, err := generateRecoveryCode()
		if err != nil {
			return nil, err
		}
		normalized := normalizeRecoveryCode(code)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO two_factor_recovery_codes (user_id, code_hash) VALUES ($1, $2)
		`, userID, tokenHash(normalized)); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, nil
}

func generateRecoveryCode() (string, error) {
	raw, err := randomBytes(10)
	if err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	encoded = strings.ToUpper(encoded)
	return encoded[0:4] + "-" + encoded[4:8] + "-" + encoded[8:12] + "-" + encoded[12:16], nil
}

func normalizeRecoveryCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func validEmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email && strings.Contains(email, "@")
}
