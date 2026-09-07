// Command project is the jobs service: sales drawing on the resource catalogue
// and distributions drawing on vehicles. It is verify-only — it checks tokens
// issued by profile and issues none.
package main

import (
	"log/slog"
	"os"

	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/config"
	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/project/internal/handler"
	"github.com/karina/kamis-be-go/services/project/internal/migrations"
	"github.com/karina/kamis-be-go/services/project/internal/repository"
	"github.com/karina/kamis-be-go/services/project/internal/router"
	"github.com/karina/kamis-be-go/services/project/internal/service"
)

func main() {
	httpx.SetupLogging("project")

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

	// An unset URL disables that dependency: the call fails and the request that
	// needed it fails with it, rather than silently recording a project against
	// a client or vehicle nobody verified.
	deps := service.Deps{
		Profile:  httpx.NewClient(os.Getenv("PROFILE_URL"), httpx.DefaultTimeout),
		Resource: httpx.NewClient(os.Getenv("RESOURCE_URL"), httpx.DefaultTimeout),
		Asset:    httpx.NewClient(os.Getenv("ASSET_URL"), httpx.DefaultTimeout),
		Finance:  httpx.NewClient(os.Getenv("FINANCE_URL"), httpx.DefaultTimeout),
	}

	repo := repository.NewProjectRepository(db)
	handlers := handler.NewProjectHandler(service.NewProjectService(repo, deps))

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
