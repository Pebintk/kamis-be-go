// Package handler is the transport layer (Spring @RestController).
//
// Errors go through httpx.RespondError, so every endpoint answers 400 for a
// caller mistake, 404 for a missing purchase and 500 for anything else. The
// legacy PurchaseController answered 400 for nearly everything, including
// database failures.
package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/purchase/internal/dto"
	"github.com/karina/kamis-be-go/services/purchase/internal/repository"
	"github.com/karina/kamis-be-go/services/purchase/internal/service"
)

// filterDateLayout is the format the legacy @DateTimeFormat declared for the
// startDate and endDate query parameters.
const filterDateLayout = "02-01-2006"

type PurchaseHandler struct{ svc *service.PurchaseService }

func NewPurchaseHandler(svc *service.PurchaseService) *PurchaseHandler {
	return &PurchaseHandler{svc: svc}
}

// Add handles POST /api/purchase/add.
func (h *PurchaseHandler) Add(c *gin.Context) {
	var req dto.AddPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	purchase, err := h.svc.AddPurchase(c.Request.Context(), req)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Success", purchase)
}

// All handles GET /api/purchase/viewall.
func (h *PurchaseHandler) All(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	purchases, err := h.svc.ListPurchases(c.Request.Context(), filter)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "OK", purchases)
}

// Paginated handles GET /api/purchase/viewall/paginated. It accepts the same
// filters as /viewall plus a status.
func (h *PurchaseHandler) Paginated(c *gin.Context) {
	filter, err := parseFilter(c)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	number, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	result, err := h.svc.ListPurchasesPaginated(c.Request.Context(), filter, number, size)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Success", result)
}

// Detail handles GET /api/purchase/detail/{purchaseId}.
func (h *PurchaseHandler) Detail(c *gin.Context) {
	purchase, err := h.svc.GetPurchase(c.Request.Context(), c.Param("purchaseId"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "OK", purchase)
}

// Update handles PUT /api/purchase/update/{purchaseId}.
func (h *PurchaseHandler) Update(c *gin.Context) {
	var req dto.UpdatePurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	purchase, err := h.svc.UpdatePurchase(c.Request.Context(), c.Param("purchaseId"), req)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Pembelian berhasil diperbarui", purchase)
}

// BySupplier handles GET /api/purchase/supplier/{supplierId}.
func (h *PurchaseHandler) BySupplier(c *gin.Context) {
	purchases, err := h.svc.ListBySupplier(c.Request.Context(), c.Param("supplierId"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "OK", purchases)
}

// parseFilter reads the shared list query parameters. An unparseable value is a
// caller mistake, which is what Spring's binder answered too — unlike the rest
// of this controller, which turned everything into a 400 regardless.
func parseFilter(c *gin.Context) (repository.Filter, error) {
	f := repository.Filter{
		Type:     c.Query("type"),
		IDSearch: c.Query("idSearch"),
		Status:   c.Query("status"),
	}
	if f.Type == "all" {
		f.Type = ""
	}

	var err error
	if f.StartNominal, err = queryInt(c, "startNominal"); err != nil {
		return f, err
	}
	if f.EndNominal, err = queryInt(c, "endNominal"); err != nil {
		return f, err
	}
	if f.StartDate, err = queryDate(c, "startDate"); err != nil {
		return f, err
	}
	if f.EndDate, err = queryDate(c, "endDate"); err != nil {
		return f, err
	}
	if f.HighNominal, err = queryBool(c, "highNominal"); err != nil {
		return f, err
	}
	if f.NewDate, err = queryBool(c, "newDate"); err != nil {
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
		return nil, badQuery(name, raw, "angka")
	}
	return &v, nil
}

func queryBool(c *gin.Context, name string) (*bool, error) {
	raw, ok := c.GetQuery(name)
	if !ok || raw == "" {
		return nil, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, badQuery(name, raw, "true atau false")
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
		return nil, badQuery(name, raw, "tanggal dd-MM-yyyy")
	}
	return &v, nil
}

// AdvanceStatus handles PUT /api/purchase/updatestatus/next/{idPurchase}.
func (h *PurchaseHandler) AdvanceStatus(c *gin.Context) {
	h.changeStatus(c, h.svc.AdvanceStatus, "Status pembelian berhasil diperbarui")
}

// CancelStatus handles PUT /api/purchase/updatestatus/cancel/{idPurchase}.
func (h *PurchaseHandler) CancelStatus(c *gin.Context) {
	h.changeStatus(c, h.svc.CancelStatus, "Status pembelian berhasil dibatalkan")
}

// ConfirmPayment handles PUT /api/purchase/updatestatus/pembayaran/{idPurchase}.
func (h *PurchaseHandler) ConfirmPayment(c *gin.Context) {
	h.changeStatus(c, h.svc.ConfirmPayment, "Status pembayaran berhasil diperbarui")
}

// changeStatus is the shared body of the three status endpoints, which differ
// only in the service call and the success message. Which roles may take a
// given transition depends on the purchase's current status, so that check lives
// in the service rather than on the route.
func (h *PurchaseHandler) changeStatus(
	c *gin.Context,
	apply func(ctx context.Context, purchaseID string, req dto.UpdateStatusRequest) (*dto.PurchaseResponse, error),
	message string,
) {
	var req dto.UpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	purchase, err := apply(c.Request.Context(), c.Param("idPurchase"), req)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, message, purchase)
}
