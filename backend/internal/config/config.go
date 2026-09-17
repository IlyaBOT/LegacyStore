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
	TrustProxyHeaders     bool
	StorageBackend        string
	LocalStoragePath      string
	MaxUploadBytes        int64
}

func Load() Config {
	return Config{
		Host:                  env("BACKEND_HOST", "0.0.0.0"),
		Port:                  env("BACKEND_PORT", "8080"),
		PublicBaseURL:         env("PUBLIC_BASE_URL", "http://localhost:8080"),
		DatabaseURL:           env("DATABASE_URL", ""),
		CatalogSigningEnabled: env("CATALOG_SIGNING_ENABLED", "false") == "true",
		TrustProxyHeaders:     env("TRUST_PROXY_HEADERS", "false") == "true",
		StorageBackend:        env("STORAGE_BACKEND", "local"),
		LocalStoragePath:      env("LOCAL_STORAGE_PATH", "/data/storage"),
		MaxUploadBytes:        int64Env("MAX_UPLOAD_BYTES", 8<<30),
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
