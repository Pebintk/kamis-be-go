// Command profile is the auth/account service: it both issues tokens (login)
// and verifies them (to guard /api/profile/**). It is the only service wired
// with an auth.Issuer.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/profile/internal/config"
	"github.com/karina/kamis-be-go/services/profile/internal/handler"
	"github.com/karina/kamis-be-go/services/profile/internal/migrations"
	"github.com/karina/kamis-be-go/services/profile/internal/repository"
	"github.com/karina/kamis-be-go/services/profile/internal/router"
	"github.com/karina/kamis-be-go/services/profile/internal/service"
)

func main() {
	httpx.SetupLogging("profile")

	cfg, err := config.Load()
	if err != nil {
		fail("config", err)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		fail("database", err)
	}
	if err := database.Migrate(db, migrations.FS); err != nil {
		fail("migrate", err)
	}

	verifier, err := auth.NewVerifierFromKeys(cfg.JWTPublicKeys...)
	if err != nil {
		fail("auth verifier", err)
	}
	issuer, err := auth.NewIssuer(cfg.JWTPrivateKey, cfg.JWTExpiration)
	if err != nil {
		fail("auth issuer", err)
	}

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewRefreshTokenRepository(db)
	svc := service.NewUserService(userRepo, tokenRepo, issuer, cfg.RefreshExpiration)

	// Clients for the services the client/supplier flows call out to, standing
	// in for the Java WebClient instances.
	projectClient := httpx.NewClient(cfg.ProjectURL, httpx.DefaultTimeout)
	resourceClient := httpx.NewClient(cfg.ResourceURL, httpx.DefaultTimeout)
	assetClient := httpx.NewClient(cfg.AssetURL, httpx.DefaultTimeout)
	purchaseClient := httpx.NewClient(cfg.PurchaseURL, httpx.DefaultTimeout)

	clientSvc := service.NewClientService(repository.NewClientRepository(db), projectClient)
	supplierSvc := service.NewSupplierService(
		repository.NewSupplierRepository(db), resourceClient, assetClient, purchaseClient)

	// Seed the default admin (legacy AdminInitializer). Non-fatal on failure.
	if err := svc.EnsureAdmin(context.Background(), cfg.AdminEmail, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		slog.Warn("could not ensure admin account", "error", err)
	}

	r := router.New(verifier, cfg.AllowedOrigins(), router.Handlers{
		Auth:     handler.NewAuthHandler(svc),
		Profile:  handler.NewProfileHandler(svc),
		Client:   handler.NewClientHandler(clientSvc),
		Supplier: handler.NewSupplierHandler(supplierSvc),
	})

	if err := httpx.Serve(r, cfg.Port); err != nil {
		fail("server", err)
	}
}

// fail logs a fatal startup error and exits. slog has no Fatal, so this keeps
// the exit path in one place.
func fail(what string, err error) {
	slog.Error("startup failed", "at", what, "error", err)
	os.Exit(1)
}
