// Package router wires the purchase routes and their auth rules — the Go analog
// of the service's WebSecurityConfig.
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/purchase/internal/handler"
)

// allRoles is the full set — the legacy config's
// hasAnyAuthority("Direksi","Finance","Operasional","Admin"), which is also what
// its .anyRequest().authenticated() fallback amounted to.
var allRoles = []string{"Admin", "Direksi", "Finance", "Operasional"}

// writeRoles guards creating and editing a purchase.
var writeRoles = []string{"Operasional", "Admin"}

func New(v *auth.Verifier, allowedOrigins []string, purchases *handler.PurchaseHandler) *gin.Engine {
	r := gin.New()
	r.Use(httpx.RequestLogger(), gin.Recovery(), httpx.CORS(allowedOrigins), auth.ForwardToken())

	r.GET("/health", handler.Health)

	// Nothing under /api is public: this service verifies tokens and issues none.
	secured := r.Group("/api")
	secured.Use(v.GinAuth())
	{
		read := auth.GinRequireRole(allRoles...)
		write := auth.GinRequireRole(writeRoles...)

		secured.POST("/purchase/add", write, purchases.Add)
		secured.PUT("/purchase/update/:purchaseId", write, purchases.Update)

		secured.GET("/purchase/viewall", read, purchases.All)
		secured.GET("/purchase/viewall/paginated", read, purchases.Paginated)
		secured.GET("/purchase/detail/:purchaseId", read, purchases.Detail)
		secured.GET("/purchase/supplier/:supplierId", read, purchases.BySupplier)
	}

	return r
}
