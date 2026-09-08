// Package router wires the purchase routes and their auth rules — the Go analog
// of the service's WebSecurityConfig.
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/pkg/metrics"
	"github.com/karina/kamis-be-go/services/purchase/internal/handler"
)

// allRoles is the full set — the legacy config's
// hasAnyAuthority("Direksi","Finance","Operasional","Admin"), which is also what
// its .anyRequest().authenticated() fallback amounted to.
var allRoles = []string{"Admin", "Direksi", "Finance", "Operasional"}

// writeRoles guards creating and editing a purchase.
var writeRoles = []string{"Operasional", "Admin"}

func New(v *auth.Verifier, allowedOrigins []string,
	purchases *handler.PurchaseHandler, staged *handler.AssetTempHandler) *gin.Engine {
	r := gin.New()
	r.Use(httpx.RequestLogger(), gin.Recovery(), metrics.Middleware(), httpx.CORS(allowedOrigins), auth.ForwardToken())

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

		// Staged assets: created here while a purchase is in flight, handed to
		// the asset service once it completes.
		secured.POST("/purchase/addAsset", write, staged.Add)
		secured.GET("/purchase/asset/:idAsset", read, staged.Detail)
		secured.GET("/purchase/asset/:idAsset/foto", read, staged.Photo)

		// The status endpoints are open to all four roles at the route level
		// because which role may act depends on the purchase's current status —
		// Direksi and Finance approve a submission, Operasional processes and
		// completes it, Finance alone confirms payment. Those rules are enforced
		// in the service, where the status is known.
		secured.PUT("/purchase/updatestatus/next/:idPurchase", read, purchases.AdvanceStatus)
		secured.PUT("/purchase/updatestatus/cancel/:idPurchase", read, purchases.CancelStatus)
		secured.PUT("/purchase/updatestatus/pembayaran/:idPurchase", read, purchases.ConfirmPayment)

		// Reporting. The legacy config gave /chart/** to Operasional and Admin
		// only, while /range and /summary fell through to every authenticated
		// role — kept as-is.
		secured.GET("/purchase/chart/purchase-activity", auth.GinRequireRole(writeRoles...), purchases.ActivityLine)
		secured.GET("/purchase/range", read, purchases.ByRange)
		secured.GET("/purchase/summary", read, purchases.Summary)
	}

	metrics.Install(r)

	return r
}
