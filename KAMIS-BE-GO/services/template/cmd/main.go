// Command template is the entrypoint for the template service. Copy this whole
// services/<name> tree to start a new service, then replace the sample Resource
// domain (model/repository/service/handler) and the routes in internal/router.
package main

import (
	"log/slog"
	"os"

	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/config"
	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/template/internal/handler"
	"github.com/karina/kamis-be-go/services/template/internal/migrations"
	"github.com/karina/kamis-be-go/services/template/internal/repository"
	"github.com/karina/kamis-be-go/services/template/internal/router"
	"github.com/karina/kamis-be-go/services/template/internal/service"
)

func main() {
	httpx.SetupLogging("template")

	cfg, err := config.Load()
	if err != nil {
		fail("config", err)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		fail("database", err)
	}

	// Schema comes from the SQL files in internal/migrations, embedded in the
	// binary. Add a numbered file there for each change; database.Migrate
	// applies whatever is pending at start.
	if err := database.Migrate(db, migrations.FS); err != nil {
		fail("migrate", err)
	}

	// This service only verifies tokens. The issuer (auth.NewIssuer) is wired
	// only in the profile service's main.go.
	verifier, err := auth.NewVerifier(cfg.JWTPublicKey)
	if err != nil {
		fail("auth verifier", err)
	}

	// Compose the layers: repository -> service -> handler.
	resourceRepo := repository.NewResourceRepository(db)
	resourceSvc := service.NewResourceService(resourceRepo)
	resourceHandler := handler.NewResourceHandler(resourceSvc)

	r := router.New(verifier, resourceHandler)

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
