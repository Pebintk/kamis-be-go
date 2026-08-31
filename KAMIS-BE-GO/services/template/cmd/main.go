// Command template is the entrypoint for the template service. Copy this whole
// services/<name> tree to start a new service, then replace the sample Resource
// domain (model/repository/service/handler) and the routes in internal/router.
package main

import (
	"log"

	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/config"
	"github.com/karina/kamis-be-go/pkg/database"
	"github.com/karina/kamis-be-go/services/template/internal/handler"
	"github.com/karina/kamis-be-go/services/template/internal/model"
	"github.com/karina/kamis-be-go/services/template/internal/repository"
	"github.com/karina/kamis-be-go/services/template/internal/router"
	"github.com/karina/kamis-be-go/services/template/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	// AutoMigrate parallels the legacy Hibernate `ddl-auto: update`. Swap for
	// goose/golang-migrate once you want explicit, reviewed schema changes.
	if err := db.AutoMigrate(&model.Resource{}); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	// This service only verifies tokens. The issuer (auth.NewIssuer) is wired
	// only in the profile service's main.go.
	verifier, err := auth.NewVerifier(cfg.JWTPublicKey)
	if err != nil {
		log.Fatalf("auth verifier: %v", err)
	}

	// Compose the layers: repository -> service -> handler.
	resourceRepo := repository.NewResourceRepository(db)
	resourceSvc := service.NewResourceService(resourceRepo)
	resourceHandler := handler.NewResourceHandler(resourceSvc)

	r := router.New(verifier, resourceHandler)

	log.Printf("template service listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
