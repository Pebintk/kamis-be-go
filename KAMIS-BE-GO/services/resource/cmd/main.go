// Command resource is the inventory catalogue service. It is verify-only: it
// checks tokens issued by profile and issues none of its own.
package main

import (
	"log/slog"
	"os"

	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/config"
	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/resource/internal/handler"
	"github.com/karina/kamis-be-go/services/resource/internal/migrations"
	"github.com/karina/kamis-be-go/services/resource/internal/repository"
	"github.com/karina/kamis-be-go/services/resource/internal/router"
	"github.com/karina/kamis-be-go/services/resource/internal/service"
)

func main() {
	httpx.SetupLogging("resource")

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

	verifier, err := auth.NewVerifier(cfg.JWTPublicKey)
	if err != nil {
		fail("auth verifier", err)
	}

	repo := repository.NewResourceRepository(db)
	handlers := handler.NewResourceHandler(service.NewResourceService(repo))

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
