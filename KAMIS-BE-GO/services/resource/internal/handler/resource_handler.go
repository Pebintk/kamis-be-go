// Package handler is the transport layer (Spring @RestController).
//
// Every endpoint maps errors the same way — 400 for a caller mistake, 404 for a
// resource that does not exist, 500 for anything unexpected. The legacy
// ResourceController wrapped each method in its own catch blocks and they did
// not agree with each other (a missing resource was 404 on find/{id} but 400 on
// update/{id}; an unexpected failure was 500 on viewall/paginated but 400 on
// viewall). See statusFor.
package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/resource/internal/dto"
	"github.com/karina/kamis-be-go/services/resource/internal/service"
)

type ResourceHandler struct{ svc *service.ResourceService }

func NewResourceHandler(svc *service.ResourceService) *ResourceHandler {
	return &ResourceHandler{svc: svc}
}

// Add handles POST /api/resource/add.
func (h *ResourceHandler) Add(c *gin.Context) {
	var req dto.AddResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	resource, err := h.svc.AddResource(c.Request.Context(), req)
	if err != nil {
		respondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Success", resource)
}

// All handles GET /api/resource/viewall.
func (h *ResourceHandler) All(c *gin.Context) {
	resources, err := h.svc.ListResources(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "OK", resources)
}

// Paginated handles GET /api/resource/viewall/paginated.
func (h *ResourceHandler) Paginated(c *gin.Context) {
	number, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))

	result, err := h.svc.ListResourcesPaginated(c.Request.Context(), c.Query("resourceName"), number, size)
	if err != nil {
		respondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Success", result)
}

// Detail handles GET /api/resource/find/{idResource}.
func (h *ResourceHandler) Detail(c *gin.Context) {
	id, ok := pathInt64(c, "idResource")
	if !ok {
		return
	}
	resource, err := h.svc.GetResource(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "OK", resource)
}

// Update handles PUT /api/resource/update/{idResource}.
func (h *ResourceHandler) Update(c *gin.Context) {
	id, ok := pathInt64(c, "idResource")
	if !ok {
		return
	}
	var req dto.UpdateResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	resource, err := h.svc.UpdateResource(c.Request.Context(), id, req)
	if err != nil {
		respondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "OK", resource)
}

// AddStock handles PUT /api/resource/{idResource}/add-stock.
func (h *ResourceHandler) AddStock(c *gin.Context) {
	h.changeStock(c, h.svc.AddStock, "Stock berhasil ditambahkan")
}

// DeductStock handles PUT /api/resource/{idResource}/deduct-stock.
func (h *ResourceHandler) DeductStock(c *gin.Context) {
	h.changeStock(c, h.svc.DeductStock, "Stock berhasil dikurangi")
}

// changeStock is the shared body of the add-stock and deduct-stock endpoints,
// which differ only in the service call and the success message.
func (h *ResourceHandler) changeStock(
	c *gin.Context,
	apply func(ctx context.Context, id int64, quantity int) (*dto.ResourceResponse, error),
	message string,
) {
	id, ok := pathInt64(c, "idResource")
	if !ok {
		return
	}
	var req dto.UpdateResourceStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	resource, err := apply(c.Request.Context(), id, *req.Quantity)
	if err != nil {
		respondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, message, resource)
}

// AddToDB handles PUT /api/resource/addToDb/{idResource}/{stockUpdate}, the
// endpoint the purchase service calls to book a confirmed purchase into stock.
func (h *ResourceHandler) AddToDB(c *gin.Context) {
	id, ok := pathInt64(c, "idResource")
	if !ok {
		return
	}
	stock, ok := pathInt64(c, "stockUpdate")
	if !ok {
		return
	}
	resource, err := h.svc.AddStockToDB(c.Request.Context(), id, int(stock))
	if err != nil {
		respondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "OK", resource)
}

// BySupplier handles GET /api/resource/find-by-supplier/{idSupplier}.
func (h *ResourceHandler) BySupplier(c *gin.Context) {
	resources, err := h.svc.ListBySupplier(c.Request.Context(), c.Param("idSupplier"))
	if err != nil {
		respondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "OK", resources)
}

// ByStock handles GET /api/resource/find-by-stock/{stock}, the low-stock report.
func (h *ResourceHandler) ByStock(c *gin.Context) {
	stock, ok := pathInt64(c, "stock")
	if !ok {
		return
	}
	resources, err := h.svc.ListByStockAtMost(c.Request.Context(), int(stock))
	if err != nil {
		respondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "OK", resources)
}

// AddSupplier handles PUT /api/resource/add-supplier.
func (h *ResourceHandler) AddSupplier(c *gin.Context) {
	h.linkSupplier(c, h.svc.AddSupplier, "Data supplier berhasil diperbarui")
}

// UpdateSupplier handles PUT /api/resource/update-supplier.
func (h *ResourceHandler) UpdateSupplier(c *gin.Context) {
	h.linkSupplier(c, h.svc.UpdateSupplier, "Data supplier berhasil diupdate")
}

// linkSupplier is the shared body of the two supplier-link endpoints. Both
// return a null data field, as the legacy BaseResponseDTO<Void> did.
func (h *ResourceHandler) linkSupplier(
	c *gin.Context,
	apply func(ctx context.Context, supplierID string, resourceIDs []int64) error,
	message string,
) {
	var req dto.AddSupplierIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := apply(c.Request.Context(), req.SupplierID, req.ResourceID); err != nil {
		respondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, message, nil)
}

// respondError writes the error with the status its type calls for. Anything
// that is neither a caller mistake nor a missing resource is a fault on our
// side, so it answers 500 with a generic message rather than leaking the
// underlying failure (a DSN in a connection error, say) to the browser; the
// detail goes to the request log instead.
func respondError(c *gin.Context, err error) {
	status := statusFor(err)
	message := err.Error()
	if status == http.StatusInternalServerError {
		slog.ErrorContext(c.Request.Context(), "resource request failed",
			"method", c.Request.Method, "path", c.FullPath(), "error", err)
		message = "Terjadi kesalahan pada server"
	}
	httpx.Respond(c, status, message, nil)
}

// statusFor maps a service error onto its HTTP status: 400 for a caller
// mistake, 404 for a resource that does not exist, 500 for anything else.
func statusFor(err error) int {
	var invalid *service.InvalidError
	if errors.As(err, &invalid) {
		return http.StatusBadRequest
	}
	var missing *service.NotFoundError
	if errors.As(err, &missing) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

// pathInt64 parses a numeric path segment, answering 400 on a bad value the way
// Spring did before the controller method ran. It reports whether the request
// should continue.
func pathInt64(c *gin.Context, name string) (int64, bool) {
	raw := c.Param(name)
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		httpx.Respond(c, http.StatusBadRequest, name+" harus berupa angka: "+raw, nil)
		return 0, false
	}
	return value, true
}
