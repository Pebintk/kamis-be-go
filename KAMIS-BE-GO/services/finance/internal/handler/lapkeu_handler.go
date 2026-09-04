// Package handler is the transport layer (Spring @RestController).
//
// Errors go through httpx.RespondError: 400 for a caller mistake, 404 for a
// missing ledger line, 500 for anything else.
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/apierr"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/finance/internal/dto"
	"github.com/karina/kamis-be-go/services/finance/internal/repository"
	"github.com/karina/kamis-be-go/services/finance/internal/service"
)

// filterDateLayout is the format the legacy @DateTimeFormat declared for the
// startDate and endDate query parameters.
const filterDateLayout = "2006-01-02"

type LapkeuHandler struct{ svc *service.LapkeuService }

func NewLapkeuHandler(svc *service.LapkeuService) *LapkeuHandler { return &LapkeuHandler{svc: svc} }

// All handles GET /api/lapkeu/all.
func (h *LapkeuHandler) All(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	rows, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Data laporan keuangan berhasil diambil", rows)
}

// Summary handles GET /api/lapkeu/summary.
func (h *LapkeuHandler) Summary(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	summary, err := h.svc.Summarise(c.Request.Context(), filter)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Ringkasan laporan keuangan berhasil diambil", summary)
}

// Page handles GET /api/lapkeu/page, which answers the ledger screen's totals
// and rows in one request.
func (h *LapkeuHandler) Page(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	page, err := h.svc.Page(c.Request.Context(), filter)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Data laporan keuangan berhasil diambil", page)
}

// Add handles POST /api/lapkeu/add. The project, purchase and asset services
// call it when money moves, forwarding the token of whoever triggered the flow.
func (h *LapkeuHandler) Add(c *gin.Context) {
	var req dto.LapkeuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	entry, err := h.svc.Record(c.Request.Context(), req)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Data laporan keuangan berhasil ditambahkan", entry)
}

// Delete handles DELETE /api/lapkeu/{id}, which undoes a refunded project's
// entry.
func (h *LapkeuHandler) Delete(c *gin.Context) {
	if err := h.svc.Remove(c.Request.Context(), c.Param("id")); err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Data laporan keuangan berhasil dihapus", nil)
}

// parseFilter reads the shared ledger query parameters.
func parseFilter(c *gin.Context) (repository.Filter, error) {
	var f repository.Filter

	var err error
	if f.StartDate, err = queryDate(c, "startDate"); err != nil {
		return f, err
	}
	if f.EndDate, err = queryDate(c, "endDate"); err != nil {
		return f, err
	}
	if f.ActivityType, err = queryInt(c, "activityType"); err != nil {
		return f, err
	}
	return f, nil
}

func queryInt(c *gin.Context, name string) (*int, error) {
	raw, ok := c.GetQuery(name)
	if !ok || raw == "" {
		return nil, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return nil, apierr.Invalidf("Parameter %s tidak valid (%q): harus berupa angka", name, raw)
	}
	return &v, nil
}

func queryDate(c *gin.Context, name string) (*time.Time, error) {
	raw, ok := c.GetQuery(name)
	if !ok || raw == "" {
		return nil, nil
	}
	v, err := time.Parse(filterDateLayout, raw)
	if err != nil {
		return nil, apierr.Invalidf("Parameter %s tidak valid (%q): harus berupa tanggal yyyy-MM-dd", name, raw)
	}
	return &v, nil
}

// ExpenseChart handles GET /api/lapkeu/chart-pengeluaran.
func (h *LapkeuHandler) ExpenseChart(c *gin.Context) {
	rows, err := h.svc.ExpenseChart(c.Request.Context(), c.DefaultQuery("range", "THIS_YEAR"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Data chart pengeluaran berhasil diambil", rows)
}

// IncomeExpenseChart handles GET /api/lapkeu/chart-pemasukan-pengeluaran.
func (h *LapkeuHandler) IncomeExpenseChart(c *gin.Context) {
	rows, err := h.svc.IncomeExpenseChart(c.Request.Context(),
		c.Query("periodType"), c.DefaultQuery("range", "THIS_YEAR"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Data chart pemasukan dan pengeluaran berhasil diambil", rows)
}

// IncomeExpenseTotals handles GET /api/lapkeu/chart-total-pemasukan-pengeluaran,
// which is the same series with a trailing "Total" bar.
func (h *LapkeuHandler) IncomeExpenseTotals(c *gin.Context) {
	rows, err := h.svc.IncomeExpenseTotals(c.Request.Context(),
		c.Query("periodType"), c.DefaultQuery("range", "THIS_YEAR"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Data total pemasukan dan pengeluaran berhasil diambil", rows)
}

// FinancialSummary handles GET /api/finance-report/summary.
func (h *LapkeuHandler) FinancialSummary(c *gin.Context) {
	summary, err := h.svc.FinancialSummary(c.Request.Context(), c.DefaultQuery("range", "THIS_YEAR"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Ringkasan keuangan berhasil diambil", summary)
}

// ActivityChart handles GET /api/operational-report/activity-chart, combining
// the purchase and project activity streams.
func (h *LapkeuHandler) ActivityChart(c *gin.Context) {
	rows, err := h.svc.CombinedActivityChart(c.Request.Context(),
		c.DefaultQuery("range", "THIS_YEAR"),
		c.Query("periodType"),
		c.DefaultQuery("status", "ALL"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Data aktivitas operasional berhasil diambil", rows)
}
