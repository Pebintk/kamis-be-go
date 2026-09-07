// Package config loads service configuration from the environment, optionally
// seeded from a local .env file (parallels Spring's `optional:file:.env`).
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Base holds the settings every KAMIS service needs. Service-specific configs
// can embed this struct and add their own fields.
type Base struct {
	Port string

	// DatabaseURL is a Go/pgx DSN, e.g.
	//   postgres://user:pass@host:5432/dbname?sslmode=disable
	// NOTE: this is NOT the Java JDBC URL (jdbc:postgresql://...). When porting a
	// service, translate the JDBC URL to this form.
	DatabaseURL string

	JWTPublicKey  string        // base64 X509 — required by every service (verify)
	JWTPrivateKey string        // base64 PKCS8 — set only by profile (issue)
	JWTExpiration time.Duration // access-token lifetime, from JWT_EXPIRATION_MS

	// RefreshExpiration is how long a refresh token lives, from
	// REFRESH_EXPIRATION_MS. Only profile issues or accepts one.
	RefreshExpiration time.Duration

	FrontendURL string
}

// Load reads .env (if present) then the process environment, and validates the
// settings shared by all services.
func Load() (Base, error) {
	_ = godotenv.Load() // .env is optional; ignore "not found"

	// 15 minutes. The access token cannot be revoked — every service verifies
	// it locally, with no callback to profile — so its lifetime *is* the window
	// in which a logout has not yet taken effect. It was 24 hours, which made
	// logging out meaningless for a day.
	expMs, err := strconv.Atoi(getenv("JWT_EXPIRATION_MS", "900000"))
	if err != nil {
		return Base{}, fmt.Errorf("JWT_EXPIRATION_MS must be an integer: %w", err)
	}
	// 7 days. Revoking this is what a real logout does.
	refreshMs, err := strconv.Atoi(getenv("REFRESH_EXPIRATION_MS", "604800000"))
	if err != nil {
		return Base{}, fmt.Errorf("REFRESH_EXPIRATION_MS must be an integer: %w", err)
	}

	cfg := Base{
		Port:              getenv("PORT", "8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTPublicKey:      os.Getenv("JWT_PUBLIC_KEY"),
		JWTPrivateKey:     os.Getenv("JWT_SECRET_KEY"),
		JWTExpiration:     time.Duration(expMs) * time.Millisecond,
		RefreshExpiration: time.Duration(refreshMs) * time.Millisecond,
		FrontendURL:       os.Getenv("FRONTEND_URL"),
	}

	if cfg.JWTPublicKey == "" {
		return Base{}, fmt.Errorf("JWT_PUBLIC_KEY is required")
	}
	if cfg.DatabaseURL == "" {
		return Base{}, fmt.Errorf("DATABASE_URL is required")
	}
	return cfg, nil
}

// AllowedOrigins returns the CORS origins for this service. The Java CorsConfig
// also listed the sibling services' base URLs, but those are server-to-server
// callers that never send an Origin header, so only the frontend matters.
func (b Base) AllowedOrigins() []string {
	var origins []string
	if b.FrontendURL != "" {
		origins = append(origins, b.FrontendURL)
	}
	return origins
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
