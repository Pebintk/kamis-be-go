// Command finance is the reporting service: the financial ledger every other
// service pushes to, and the dashboards built on top of it. It is verify-only —
// it checks tokens issued by profile and issues none.
//
// The Java service is named finance.report; the dot does not belong in a Go
// import path, so the directory is services/finance.
package main

import (
	"log/slog"
	"os"

	"github.com/pebintk/kamis-be-go/pkg/auth"
	"github.com/pebintk/kamis-be-go/pkg/config"
	"github.com/pebintk/kamis-be-go/pkg/database"
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/services/finance/internal/handler"
	"github.com/pebintk/kamis-be-go/services/finance/internal/migrations"
	"github.com/pebintk/kamis-be-go/services/finance/internal/repository"
	"github.com/pebintk/kamis-be-go/services/finance/internal/router"
	"github.com/pebintk/kamis-be-go/services/finance/internal/service"
)

func main() {
	httpx.SetupLogging("finance")

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

	// The operational report combines the project and purchase activity charts.
	// The ledger itself needs nothing external — every other service pushes to it.
	deps := service.Deps{
		Project:  httpx.NewClient(os.Getenv("PROJECT_URL"), httpx.DefaultTimeout),
		Purchase: httpx.NewClient(os.Getenv("PURCHASE_URL"), httpx.DefaultTimeout),
	}

	repo := repository.NewLapkeuRepository(db)
	handlers := handler.NewLapkeuHandler(service.NewLapkeuService(repo, deps))

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
