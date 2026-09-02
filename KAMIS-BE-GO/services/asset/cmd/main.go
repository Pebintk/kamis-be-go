// Command asset is the vehicle registry: assets, their photos, their
// maintenance history and their project reservations. It is verify-only — it
// checks tokens issued by profile and issues none.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/blob"
	"github.com/karina/kamis-be-go/pkg/config"
	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/asset/internal/handler"
	"github.com/karina/kamis-be-go/services/asset/internal/model"
	"github.com/karina/kamis-be-go/services/asset/internal/repository"
	"github.com/karina/kamis-be-go/services/asset/internal/router"
	"github.com/karina/kamis-be-go/services/asset/internal/service"
)

func main() {
	httpx.SetupLogging("asset")

	cfg, err := config.Load()
	if err != nil {
		fail("config", err)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		fail("database", err)
	}
	if err := db.AutoMigrate(&model.Asset{}, &model.Maintenance{}); err != nil {
		fail("migrate", err)
	}

	verifier, err := auth.NewVerifier(cfg.JWTPublicKey)
	if err != nil {
		fail("auth verifier", err)
	}

	// GCS when GCS_BUCKET is set, a local directory otherwise — see pkg/blob.
	photos, err := blob.FromEnv(context.Background())
	if err != nil {
		fail("photo storage", err)
	}

	repo := repository.NewAssetRepository(db)
	handlers := handler.NewAssetHandler(service.NewAssetService(repo, photos))

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
