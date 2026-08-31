// Package router wires the profile routes and their auth rules — the Go analog
// of WebSecurityConfig.
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/profile/internal/handler"
)

// Handlers bundles the service's HTTP handlers so the router signature does not
// grow a parameter per domain.
type Handlers struct {
	Auth     *handler.AuthHandler
	Profile  *handler.ProfileHandler
	Client   *handler.ClientHandler
	Supplier *handler.SupplierHandler
}

// allRoles is Spring's hasAnyAuthority("Admin","Direksi","Finance","Operasional").
var allRoles = []string{"Admin", "Direksi", "Finance", "Operasional"}

func New(v *auth.Verifier, allowedOrigins []string, h Handlers) *gin.Engine {
	r := gin.New()
	// ForwardToken runs on every route, public ones included: the legacy
	// services read the Authorization header straight off the request and
	// forward it downstream regardless of whether the route required it.
	r.Use(gin.Logger(), gin.Recovery(), httpx.CORS(allowedOrigins), auth.ForwardToken())

	r.GET("/health", handler.Health)

	api := r.Group("/api")

	// ---- permitAll ----
	api.POST("/auth/login", h.Auth.Login)
	api.POST("/profile/add", h.Profile.Add)

	// WARNING — these three are unauthenticated in the legacy WebSecurityConfig
	// too, and that is not an oversight in this port. Its rule list covers
	// /api/client/all and /api/client/add but has no /api/client/** catch-all,
	// so everything else under /api/client falls through to
	// `.anyRequest().permitAll()`. The frontend depends on it: its
	// getClientDetail and updateClient calls send no Authorization header at
	// all, so requiring a token here would break the client detail and edit
	// pages. Tighten this and the frontend together, not separately.
	api.GET("/client/all/paginated", h.Client.Paginated)
	api.GET("/client/:id", h.Client.Detail)
	api.PUT("/client/update/:id", h.Client.Update)

	// ---- authenticated ----
	secured := api.Group("")
	secured.Use(v.GinAuth())
	{
		// /api/profile/**
		secured.GET("/profile/all", auth.GinRequireRole("Admin"), h.Profile.All)
		secured.GET("/profile/all/paginated", auth.GinRequireRole(allRoles...), h.Profile.Paginated)
		secured.PUT("/profile/:id", auth.GinRequireRole(allRoles...), h.Profile.Update)

		// /api/client/**
		secured.GET("/client/all", auth.GinRequireRole("Operasional", "Direksi", "Admin", "Finance"), h.Client.All)
		secured.POST("/client/add", auth.GinRequireRole("Operasional"), h.Client.Add)

		// /api/supplier/** — the specific rules come first, exactly as the
		// ordered matcher list in WebSecurityConfig does.
		secured.POST("/supplier/add", auth.GinRequireRole("Operasional"), h.Supplier.Add)
		secured.PUT("/supplier/update", auth.GinRequireRole("Operasional", "Admin"), h.Supplier.Update)
		secured.PUT("/supplier/add-purchase", auth.GinRequireRole("Operasional", "Admin"), h.Supplier.AddPurchase)

		supplier := auth.GinRequireRole(allRoles...)
		secured.GET("/supplier/all", supplier, h.Supplier.All)
		secured.GET("/supplier/all/paginated", supplier, h.Supplier.Paginated)
		secured.GET("/supplier/getall", supplier, h.Supplier.GetAll)
		secured.GET("/supplier/name/:supplierId", supplier, h.Supplier.Name)
		secured.GET("/supplier/detail/:supplierId", supplier, h.Supplier.Detail)
	}

	return r
}
