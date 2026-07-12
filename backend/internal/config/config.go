package config

import "os"

type Config struct {
	Host                  string
	Port                  string
	PublicBaseURL         string
	DatabaseURL           string
	CatalogSigningEnabled bool
}

func Load() Config {
	return Config{
		Host:                  env("BACKEND_HOST", "0.0.0.0"),
		Port:                  env("BACKEND_PORT", "8080"),
		PublicBaseURL:         env("PUBLIC_BASE_URL", "http://localhost:8080"),
		DatabaseURL:           env("DATABASE_URL", ""),
		CatalogSigningEnabled: env("CATALOG_SIGNING_ENABLED", "false") == "true",
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
