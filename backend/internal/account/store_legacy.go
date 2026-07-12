package account

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strings"

	"github.com/lib/pq"
)

var defaultLegacyScopes = []string{"catalog:read", "downloads:read", "reviews:write", "reviews:like", "profile:read_basic"}

func (s *Store) ListLegacyPasswords(ctx context.Context, userID int64) ([]LegacyPassword, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, token_prefix, scopes, COALESCE(last_used_at::text, ''), created_at::text
		FROM legacy_passwords
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []LegacyPassword
	for rows.Next() {
		var item LegacyPassword
		if err := rows.Scan(&item.ID, &item.Name, &item.Prefix, pq.Array(&item.Scopes), &item.LastUsedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateLegacyPassword(ctx context.Context, userID int64, name string, scopes []string) (*LegacyPassword, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Legacy Client"
	}
	scopes = sanitizeLegacyScopes(scopes)
	secret, err := randomToken()
	if err != nil {
		return nil, "", err
	}
	token := "ls_" + secret
	prefix := token[:10]
	var item LegacyPassword
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO legacy_passwords (user_id, name, token_hash, token_prefix, scopes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, token_prefix, scopes, COALESCE(last_used_at::text, ''), created_at::text
	`, userID, name, tokenHash(token), prefix, pq.Array(scopes)).Scan(&item.ID, &item.Name, &item.Prefix, pq.Array(&item.Scopes), &item.LastUsedAt, &item.CreatedAt)
	return &item, token, err
}

func (s *Store) ResetLegacyPassword(ctx context.Context, userID, legacyPasswordID int64) (*LegacyPassword, string, error) {
	secret, err := randomToken()
	if err != nil {
		return nil, "", err
	}
	token := "ls_" + secret
	prefix := token[:10]
	var item LegacyPassword
	err = s.db.QueryRowContext(ctx, `
		UPDATE legacy_passwords
		SET token_hash = $3, token_prefix = $4, last_used_at = NULL
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
		RETURNING id, name, token_prefix, scopes, COALESCE(last_used_at::text, ''), created_at::text
	`, legacyPasswordID, userID, tokenHash(token), prefix).Scan(&item.ID, &item.Name, &item.Prefix, pq.Array(&item.Scopes), &item.LastUsedAt, &item.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, "", ErrNotFound
	}
	return &item, token, err
}

func (s *Store) DeleteLegacyPassword(ctx context.Context, userID, legacyPasswordID int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE legacy_passwords
		SET revoked_at = now()
		WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
	`, legacyPasswordID, userID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) LegacyLogin(ctx context.Context, email, legacyPassword, deviceIdentifier, deviceName string, ip net.IP, userAgent string) (*AuthResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.email, COALESCE(u.nickname, ''), COALESCE(u.avatar_url, ''), u.email_verified,
		       u.two_factor_enabled, u.status, u.created_at::text, u.updated_at::text,
		       lp.id, lp.token_hash
		FROM users u
		JOIN legacy_passwords lp ON lp.user_id = u.id
		WHERE lower(u.email) = $1 AND lp.revoked_at IS NULL AND u.status = 'active'
		ORDER BY lp.created_at DESC
	`, email)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var user User
	var legacyID int64
	wantHash := tokenHash(legacyPassword)
	found := false
	for rows.Next() {
		var candidate User
		var candidateLegacyID int64
		var storedHash string
		if err := rows.Scan(&candidate.ID, &candidate.Email, &candidate.Nickname, &candidate.AvatarURL, &candidate.EmailVerified, &candidate.TwoFactorEnabled, &candidate.Status, &candidate.CreatedAt, &candidate.UpdatedAt, &candidateLegacyID, &storedHash); err != nil {
			return nil, err
		}
		if storedHash == wantHash {
			user = candidate
			legacyID = candidateLegacyID
			found = true
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrInvalidCredential
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
	token, expiresAt, err := createSession(ctx, tx, user.ID, true, ip, userAgent)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE legacy_passwords SET last_used_at = now() WHERE id = $1`, legacyID); err != nil {
		return nil, err
	}
	if err := upsertLegacyDevice(ctx, tx, user.ID, legacyID, deviceIdentifier, deviceName, ip, userAgent); err != nil {
		return nil, err
	}
	if err := auditTx(ctx, tx, user.ID, "auth.legacy_login", "legacy_password", intString(legacyID), ip, userAgent); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &AuthResult{User: user, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *Store) ListLegacyDevices(ctx context.Context, userID int64) ([]LegacyDevice, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(legacy_password_id, 0), COALESCE(device_identifier, ''), COALESCE(device_name, ''),
		       COALESCE(last_ip::text, ''), COALESCE(last_user_agent, ''), last_seen_at::text, created_at::text
		FROM legacy_devices
		WHERE user_id = $1 AND revoked_at IS NULL
		ORDER BY last_seen_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var devices []LegacyDevice
	for rows.Next() {
		var device LegacyDevice
		if err := rows.Scan(&device.ID, &device.LegacyPasswordID, &device.DeviceIdentifier, &device.DeviceName, &device.LastIP, &device.LastUserAgent, &device.LastSeenAt, &device.CreatedAt); err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

func (s *Store) RevokeLegacyDevice(ctx context.Context, userID, deviceID int64) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE legacy_devices
		SET revoked_at = now()
		WHERE id = $1 AND user_id = $2
	`, deviceID, userID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func upsertLegacyDevice(ctx context.Context, tx *sql.Tx, userID, legacyID int64, identifier, name string, ip net.IP, userAgent string) error {
	identifier = strings.TrimSpace(identifier)
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Legacy Mac"
	}
	if identifier == "" {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO legacy_devices (user_id, legacy_password_id, device_name, last_ip, last_user_agent)
			VALUES ($1, $2, $3, $4, $5)
		`, userID, legacyID, name, nullableIP(ip), truncate(userAgent, 512))
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO legacy_devices (user_id, legacy_password_id, device_identifier, device_name, last_ip, last_user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, device_identifier) WHERE device_identifier IS NOT NULL
		DO UPDATE SET legacy_password_id = EXCLUDED.legacy_password_id,
		              device_name = EXCLUDED.device_name,
		              last_ip = EXCLUDED.last_ip,
		              last_user_agent = EXCLUDED.last_user_agent,
		              last_seen_at = now(),
		              revoked_at = NULL
	`, userID, legacyID, identifier, name, nullableIP(ip), truncate(userAgent, 512))
	return err
}

func sanitizeLegacyScopes(scopes []string) []string {
	allowed := map[string]bool{}
	for _, scope := range defaultLegacyScopes {
		allowed[scope] = true
	}
	var clean []string
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if allowed[scope] {
			clean = append(clean, scope)
		}
	}
	if len(clean) == 0 {
		return append([]string(nil), defaultLegacyScopes...)
	}
	return clean
}
