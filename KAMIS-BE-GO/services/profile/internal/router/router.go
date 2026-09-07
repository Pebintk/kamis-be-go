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
	r.Use(httpx.RequestLogger(), gin.Recovery(), httpx.CORS(allowedOrigins), auth.ForwardToken())

	r.GET("/health", handler.Health)

	api := r.Group("/api")

	// ---- permitAll ----
	// Login is the only public route. The legacy config also left
	// POST /profile/add open, which let anyone mint an account with any role,
	// Admin included — see MIGRATION.md. It is Admin-only below.
	api.POST("/auth/login", h.Auth.Login)
	// Refresh and logout authenticate by the refresh token in their body, not by
	// a bearer: the access token they concern has usually expired already.
	api.POST("/auth/refresh", h.Auth.Refresh)
	api.POST("/auth/logout", h.Auth.Logout)

	// ---- authenticated ----
	secured := api.Group("")
	secured.Use(v.GinAuth())
	{
		// /api/profile/**
		secured.POST("/profile/add", auth.GinRequireRole("Admin"), h.Profile.Add)
		secured.GET("/profile/all", auth.GinRequireRole("Admin"), h.Profile.All)
		secured.GET("/profile/all/paginated", auth.GinRequireRole(allRoles...), h.Profile.Paginated)
		secured.PUT("/profile/:id", auth.GinRequireRole(allRoles...), h.Profile.Update)

		// /api/client/** — the legacy WebSecurityConfig had no catch-all here,
		// so everything except /all and /add fell through to permitAll. That
		// left GET /client/{id} and PUT /client/update/{id} world-accessible.
		// Nothing depends on that any more, so all of them are guarded, with
		// writes restricted the way the supplier writes are.
		client := auth.GinRequireRole(allRoles...)
		secured.GET("/client/all", client, h.Client.All)
		secured.GET("/client/all/paginated", client, h.Client.Paginated)
		secured.GET("/client/:id", client, h.Client.Detail)
		secured.POST("/client/add", auth.GinRequireRole("Operasional"), h.Client.Add)
		secured.PUT("/client/update/:id", auth.GinRequireRole("Operasional", "Admin"), h.Client.Update)

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
