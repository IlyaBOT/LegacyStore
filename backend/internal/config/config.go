package config

import (
	"os"
	"strconv"
)

type Config struct {
	Host                  string
	Port                  string
	PublicBaseURL         string
	DatabaseURL           string
	CatalogSigningEnabled bool
	CatalogPrivateKeyPath string
	CatalogPublicKeyPath  string
	TrustProxyHeaders     bool
	StorageBackend        string
	LocalStoragePath      string
	MaxUploadBytes        int64
	AdminEmail            string
	AdminPassword         string
	AdminNickname         string
	SMTPHost              string
	SMTPPort              string
	SMTPUsername          string
	SMTPPassword          string
	SMTPFrom              string
	RecoveryBaseURL       string
	RecoveryDebugToken    bool
}

func Load() Config {
	return Config{
		Host:                  env("BACKEND_HOST", "0.0.0.0"),
		Port:                  env("BACKEND_PORT", "8080"),
		PublicBaseURL:         env("PUBLIC_BASE_URL", "http://localhost:8080"),
		DatabaseURL:           env("DATABASE_URL", ""),
		CatalogSigningEnabled: env("CATALOG_SIGNING_ENABLED", "false") == "true",
		CatalogPrivateKeyPath: env("CATALOG_PRIVATE_KEY_PATH", ""),
		CatalogPublicKeyPath:  env("CATALOG_PUBLIC_KEY_PATH", ""),
		TrustProxyHeaders:     env("TRUST_PROXY_HEADERS", "false") == "true",
		StorageBackend:        env("STORAGE_BACKEND", "local"),
		LocalStoragePath:      env("LOCAL_STORAGE_PATH", "/data/storage"),
		MaxUploadBytes:        int64Env("MAX_UPLOAD_BYTES", 8<<30),
		AdminEmail:            env("ADMIN_EMAIL", ""),
		AdminPassword:         env("ADMIN_PASSWORD", ""),
		AdminNickname:         env("ADMIN_NICKNAME", "Administrator"),
		SMTPHost:              env("SMTP_HOST", ""),
		SMTPPort:              env("SMTP_PORT", "587"),
		SMTPUsername:          env("SMTP_USERNAME", ""),
		SMTPPassword:          env("SMTP_PASSWORD", ""),
		SMTPFrom:              env("SMTP_FROM", ""),
		RecoveryBaseURL:       env("RECOVERY_BASE_URL", ""),
		RecoveryDebugToken:    env("RECOVERY_DEBUG_RETURN_TOKEN", "false") == "true",
	}
}

func (c Config) Addr() string {
	return c.Host + ":" + c.Port
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func int64Env(key string, fallback int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
