// Package config loads the profile service's configuration. Profile is the only
// service that issues tokens, so it additionally requires the JWT private key
// and the seed-admin credentials.
package config

import (
	"fmt"
	"os"

	pkgconfig "github.com/pebintk/kamis-be-go/pkg/config"
)

type Config struct {
	pkgconfig.Base
	AdminEmail    string
	AdminUsername string
	AdminPassword string

	// Base URLs of the services the client and supplier flows call, e.g.
	// http://project-service:8083/api. Empty means that dependency is not
	// configured; calls to it fail and are treated as "no data", the same way
	// the legacy service swallowed WebClient errors.
	ProjectURL  string
	ResourceURL string
	AssetURL    string
	PurchaseURL string
}

func Load() (Config, error) {
	base, err := pkgconfig.Load()
	if err != nil {
		return Config{}, err
	}
	if base.JWTPrivateKey == "" {
		return Config{}, fmt.Errorf("JWT_SECRET_KEY is required for the profile service (it issues tokens)")
	}
	return Config{
		Base:          base,
		AdminEmail:    os.Getenv("ADMIN_EMAIL"),
		AdminUsername: os.Getenv("ADMIN_USERNAME"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		ProjectURL:    os.Getenv("PROJECT_URL"),
		ResourceURL:   os.Getenv("RESOURCE_URL"),
		AssetURL:      os.Getenv("ASSET_URL"),
		PurchaseURL:   os.Getenv("PURCHASE_URL"),
	}, nil
}
