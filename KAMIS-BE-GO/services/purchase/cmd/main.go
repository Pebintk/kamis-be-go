// Command purchase is the procurement service: purchase requests, their status
// workflow, and the staged assets and resource line items behind them. It is
// verify-only — it checks tokens issued by profile and issues none.
package main

import (
	"log/slog"
	"os"

	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/config"
	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/purchase/internal/handler"
	"github.com/karina/kamis-be-go/services/purchase/internal/model"
	"github.com/karina/kamis-be-go/services/purchase/internal/repository"
	"github.com/karina/kamis-be-go/services/purchase/internal/router"
	"github.com/karina/kamis-be-go/services/purchase/internal/service"
)

func main() {
	httpx.SetupLogging("purchase")

	cfg, err := config.Load()
	if err != nil {
		fail("config", err)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		fail("database", err)
	}
	if err := db.AutoMigrate(
		&model.Purchase{},
		&model.ResourceTemp{},
		&model.AssetTemp{},
		&model.LogPurchase{},
	); err != nil {
		fail("migrate", err)
	}

	verifier, err := auth.NewVerifier(cfg.JWTPublicKey)
	if err != nil {
		fail("auth verifier", err)
	}

	// Clients for the services purchase reads from and writes to. An unset URL
	// disables that dependency: the call fails and is logged, the same way the
	// legacy WebClient errors were handled.
	profile := httpx.NewClient(os.Getenv("PROFILE_URL"), httpx.DefaultTimeout)
	resource := httpx.NewClient(os.Getenv("RESOURCE_URL"), httpx.DefaultTimeout)

	repo := repository.NewPurchaseRepository(db)
	handlers := handler.NewPurchaseHandler(service.NewPurchaseService(repo, profile, resource))

	if err := httpx.Serve(router.New(verifier, cfg.AllowedOrigins(), handlers), cfg.Port); err != nil {
		fail("server", err)
	}
}

// fail logs a fatal startup error and exits. slog has no Fatal, so this keeps
// the exit path in one place.
func fail(what string, err error) {
	slog.Error("startup failed", "at", what, "error", err)
	os.Exit(1)
}
