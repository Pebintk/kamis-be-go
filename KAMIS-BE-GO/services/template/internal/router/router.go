// Package router assembles the HTTP routes and their auth rules. This is the Go
// analog of Spring's WebSecurityConfig: public routes, an authenticated group,
// and per-route role guards via auth.GinRequireRole (≈ hasAnyAuthority).
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/pebintk/kamis-be-go/pkg/auth"
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/pkg/metrics"
	"github.com/pebintk/kamis-be-go/services/template/internal/handler"
)

func New(v *auth.Verifier, resources *handler.ResourceHandler) *gin.Engine {
	r := gin.New()
	r.Use(httpx.RequestLogger(), gin.Recovery(), metrics.Middleware())

	// Public.
	r.GET("/health", handler.Health)

	api := r.Group("/api")

	// Everything below requires a valid token (the GinAuth middleware).
	secured := api.Group("")
	secured.Use(v.GinAuth())
	{
		secured.GET("/resource/all",
			auth.GinRequireRole("Admin", "Direksi", "Finance", "Operasional"),
			resources.List)

		secured.GET("/resource/:id",
			auth.GinRequireRole("Admin", "Direksi", "Finance", "Operasional"),
			resources.Get)

		secured.POST("/resource",
			auth.GinRequireRole("Operasional"),
			resources.Create)
	}

	metrics.Install(r)

	return r
}
