package account

import (
	"errors"
	"time"
)

var (
	ErrEmailExists       = errors.New("email exists")
	ErrInvalidCredential = errors.New("invalid credentials")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrForbidden         = errors.New("forbidden")
	ErrNotFound          = errors.New("not found")
	ErrTwoFactorRequired = errors.New("two factor required")
)

type User struct {
	ID               int64    `json:"id"`
	Email            string   `json:"email"`
	Nickname         string   `json:"nickname,omitempty"`
	AvatarURL        string   `json:"avatar_url,omitempty"`
	EmailVerified    bool     `json:"email_verified"`
	TwoFactorEnabled bool     `json:"two_factor_enabled"`
	Status           string   `json:"status"`
	Roles            []string `json:"roles"`
	CreatedAt        string   `json:"created_at,omitempty"`
	UpdatedAt        string   `json:"updated_at,omitempty"`
}

type Session struct {
	ID         int64  `json:"id"`
	DeviceName string `json:"device_name,omitempty"`
	IPAddress  string `json:"ip_address,omitempty"`
	UserAgent  string `json:"user_agent,omitempty"`
	RememberMe bool   `json:"remember_me"`
	ExpiresAt  string `json:"expires_at"`
	LastSeenAt string `json:"last_seen_at"`
	CreatedAt  string `json:"created_at"`
}

type LegacyPassword struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Prefix     string   `json:"prefix"`
	Scopes     []string `json:"scopes"`
	LastUsedAt string   `json:"last_used_at,omitempty"`
	CreatedAt  string   `json:"created_at"`
}

type LegacyDevice struct {
	ID               int64  `json:"id"`
	LegacyPasswordID int64  `json:"legacy_password_id,omitempty"`
	DeviceIdentifier string `json:"device_identifier,omitempty"`
	DeviceName       string `json:"device_name,omitempty"`
	LastIP           string `json:"last_ip,omitempty"`
	LastUserAgent    string `json:"last_user_agent,omitempty"`
	LastSeenAt       string `json:"last_seen_at"`
	CreatedAt        string `json:"created_at"`
}

type Review struct {
	ID        int64  `json:"id"`
	AppSlug   string `json:"app_slug,omitempty"`
	UserID    int64  `json:"user_id,omitempty"`
	Author    string `json:"author,omitempty"`
	Rating    int    `json:"rating"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
	Likes     int    `json:"likes"`
	Replies   int    `json:"replies"`
	CreatedAt string `json:"created_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type ReviewReply struct {
	ID        int64  `json:"id"`
	ReviewID  int64  `json:"review_id"`
	UserID    int64  `json:"user_id"`
	Author    string `json:"author,omitempty"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at,omitempty"`
}

type AdminApp struct {
	ID               int64  `json:"id"`
	Slug             string `json:"slug"`
	Name             string `json:"name"`
	BundleID         string `json:"bundle_id,omitempty"`
	DeveloperName    string `json:"developer_name"`
	Summary          string `json:"summary"`
	Description      string `json:"description,omitempty"`
	Category         string `json:"category,omitempty"`
	ModerationStatus string `json:"moderation_status"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type ModerationItem struct {
	ID               int64  `json:"id"`
	EntityType       string `json:"entity_type"`
	EntityID         string `json:"entity_id"`
	Status           string `json:"status"`
	SubmittedBy      int64  `json:"submitted_by,omitempty"`
	ModeratorID      int64  `json:"moderator_id,omitempty"`
	ModeratorComment string `json:"moderator_comment,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

type AuthResult struct {
	User      User      `json:"user"`
	Token     string    `json:"session_token,omitempty"`
	ExpiresAt time.Time `json:"-"`
	CSRFToken string    `json:"csrf_token,omitempty"`
}
