// Package router wires the asset routes and their auth rules — the Go analog of
// the service's WebSecurityConfig.
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/pkg/metrics"
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
	r.Use(httpx.RequestLogger(), gin.Recovery(), metrics.Middleware(), httpx.CORS(allowedOrigins), auth.ForwardToken())

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

		// Maintenance moved under /api/asset/ from the legacy /api/maintenance/.
		// Sitting outside /api/asset/** is what let it fall through the
		// WebSecurityConfig method rules to .anyRequest().authenticated(), so
		// booking and completing a job were open to every role while the asset
		// writes beside them were not. One prefix now means one rule, and the
		// writes carry the same guard as the rest of the service.
		secured.POST("/asset/maintenance", write, maintenance.Create)
		secured.GET("/asset/maintenance/all", read, maintenance.All)
		secured.GET("/asset/maintenance/in-progress", read, maintenance.InProgress)
		secured.PATCH("/asset/maintenance/:id/complete", write, maintenance.Complete)

		// The legacy GET /api/maintenance/{platNomor} is gone: it returned the
		// same list as GET /api/asset/{platNomor}/maintenance above, through the
		// same service call. The surviving one reads as what it is.
	}

	metrics.Install(r)

	return r
}
