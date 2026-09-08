// Package router wires the finance routes and their auth rules — the Go analog
// of the service's WebSecurityConfig.
package router

import (
	"github.com/gin-gonic/gin"
	"github.com/pebintk/kamis-be-go/pkg/auth"
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/pkg/metrics"
	"github.com/pebintk/kamis-be-go/services/finance/internal/handler"
)

// ledgerRoles may read the financial ledger. It matches the legacy
// /api/finance-report/** rule and the two dashboards that consume this data:
// DashboardFinance (Finance) and DashboardDireksi (Direksi). Operasional has no
// business reading the books.
var ledgerRoles = []string{"Admin", "Finance", "Direksi"}

// writeRoles may record a ledger entry. Every write arrives from another service
// forwarding the token of whoever triggered the flow — Operasional completing a
// purchase or a project, Finance confirming a payment — so all four appear here.
var writeRoles = []string{"Admin", "Finance", "Direksi", "Operasional"}

func New(v *auth.Verifier, allowedOrigins []string, lapkeu *handler.LapkeuHandler) *gin.Engine {
	r := gin.New()
	r.Use(httpx.RequestLogger(), gin.Recovery(), metrics.Middleware(), httpx.CORS(allowedOrigins), auth.ForwardToken())

	r.GET("/health", handler.Health)

	// The legacy config declared `/api/lapkeu/** permitAll()`, leaving the whole
	// financial ledger readable, writable and deletable without a token. See
	// MIGRATION.md; every route here requires one.
	secured := r.Group("/api")
	secured.Use(v.GinAuth())
	{
		read := auth.GinRequireRole(ledgerRoles...)

		secured.GET("/lapkeu/all", read, lapkeu.All)
		secured.GET("/lapkeu/summary", read, lapkeu.Summary)
		secured.GET("/lapkeu/page", read, lapkeu.Page)

		secured.POST("/lapkeu/add", auth.GinRequireRole(writeRoles...), lapkeu.Add)
		// Only Finance refunds a project, which is the one flow that deletes an
		// entry.
		secured.DELETE("/lapkeu/:id", auth.GinRequireRole("Finance", "Admin"), lapkeu.Delete)

		secured.GET("/lapkeu/chart-pengeluaran", read, lapkeu.ExpenseChart)
		secured.GET("/lapkeu/chart-pemasukan-pengeluaran", read, lapkeu.IncomeExpenseChart)
		secured.GET("/lapkeu/chart-total-pemasukan-pengeluaran", read, lapkeu.IncomeExpenseTotals)

		// The legacy config named /api/finance-report/** explicitly; the
		// operational report fell through to .anyRequest().authenticated(),
		// which its own dashboard needs — Operasional reads it.
		secured.GET("/finance-report/summary", read, lapkeu.FinancialSummary)
		secured.GET("/operational-report/activity-chart",
			auth.GinRequireRole("Admin", "Finance", "Direksi", "Operasional"), lapkeu.ActivityChart)
	}

	metrics.Install(r)

	return r
}
