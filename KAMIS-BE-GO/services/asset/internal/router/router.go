// Package router wires the asset routes and their auth rules — the Go analog of
// the service's WebSecurityConfig.
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/asset/internal/handler"
)

// allRoles is the full set. The legacy config named these four explicitly on the
// reads and let everything else fall through to .anyRequest().authenticated(),
// which is the same thing here: there are no other roles.
var allRoles = []string{"Admin", "Direksi", "Finance", "Operasional"}

// writeRoles guards every mutation, matching the DELETE/PUT/POST/PATCH rules on
// /api/asset/**.
var writeRoles = []string{"Operasional", "Admin"}

func New(v *auth.Verifier, allowedOrigins []string, assets *handler.AssetHandler, maintenance *handler.MaintenanceHandler,
	reservations *handler.ReservationHandler) *gin.Engine {
	r := gin.New()
	r.Use(httpx.RequestLogger(), gin.Recovery(), httpx.CORS(allowedOrigins), auth.ForwardToken())

	r.GET("/health", handler.Health)

	// Nothing under /api is public: this service verifies tokens and issues none.
	secured := r.Group("/api")
	secured.Use(v.GinAuth())
	{
		read := auth.GinRequireRole(allRoles...)
		write := auth.GinRequireRole(writeRoles...)

		// The reservation routes come first: their static "reservations" segment
		// sits beside the :platNomor wildcard below.
		secured.POST("/asset/reservations/check-availability", write, reservations.CheckAvailability)
		secured.POST("/asset/reservations/reserve", write, reservations.Reserve)
		secured.PUT("/asset/reservations/project/:projectId/status", write, reservations.UpdateProjectStatus)
		secured.PUT("/asset/reservations/:reservationId/status", write, reservations.UpdateStatus)
		secured.GET("/asset/reservations/asset/:platNomor", read, reservations.ByAsset)
		secured.GET("/asset/reservations/project/:projectId", read, reservations.ByProject)

		secured.GET("/asset/all", read, assets.All)
		secured.GET("/asset/viewall/paginated", read, assets.Paginated)
		secured.GET("/asset/by-supplier/:supplierId", read, assets.BySupplier)
		secured.GET("/asset/:platNomor", read, assets.Detail)
		secured.GET("/asset/:platNomor/foto", read, assets.Photo)
		secured.GET("/asset/:platNomor/maintenance", read, assets.Maintenance)

		// addAsset is called by the purchase service, not the browser.
		secured.POST("/asset/addAsset", write, assets.Add)
		secured.PUT("/asset/:platNomor", write, assets.Update)
		secured.PUT("/asset/:platNomor/supplier", write, assets.SetSupplier)
		secured.DELETE("/asset/:platNomor", write, assets.Delete)

		// /api/maintenance/** sits outside /api/asset/**, so the legacy config's
		// method rules never applied to it and every route fell through to
		// .anyRequest().authenticated() — including the writes. Kept as-is;
		// see MIGRATION.md on the inconsistency.
		//
		// The trailing slash on the create route is load-bearing: the legacy
		// controller is mapped at "/api/maintenance/" and the frontend posts
		// there.
		secured.POST("/maintenance/", read, maintenance.Create)
		secured.GET("/maintenance/all", read, maintenance.All)
		secured.GET("/maintenance/maintenance-in-progress", read, maintenance.InProgress)
		secured.GET("/maintenance/:platNomor", read, maintenance.ByAsset)
		secured.PATCH("/maintenance/:id/complete", read, maintenance.Complete)
	}

	return r
}
