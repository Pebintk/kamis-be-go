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
	if err := db.AutoMigrate(&model.Asset{}, &model.Maintenance{}, &model.AssetReservation{}); err != nil {
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

	// The ledger is written to when a maintenance job is booked. An unset
	// FINANCE_URL disables it: the call fails and is logged, which is how the
	// legacy service treated a finance outage too.
	finance := httpx.NewClient(os.Getenv("FINANCE_URL"), httpx.DefaultTimeout)

	repo := repository.NewAssetRepository(db)
	assetSvc := service.NewAssetService(repo, photos)
	maintenanceSvc := service.NewMaintenanceService(repo, finance)
	reservationSvc := service.NewReservationService(repo)

	engine := router.New(verifier, cfg.AllowedOrigins(),
		handler.NewAssetHandler(assetSvc),
		handler.NewMaintenanceHandler(maintenanceSvc, assetSvc),
		handler.NewReservationHandler(reservationSvc))

	if err := httpx.Serve(engine, cfg.Port); err != nil {
		fail("server", err)
	}
}

// fail logs a fatal startup error and exits. slog has no Fatal, so this keeps
// the exit path in one place.
func fail(what string, err error) {
	slog.Error("startup failed", "at", what, "error", err)
	os.Exit(1)
}
