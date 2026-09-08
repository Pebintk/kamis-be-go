package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/services/profile/internal/dto"
	"github.com/pebintk/kamis-be-go/services/profile/internal/service"
)

type SupplierHandler struct{ svc *service.SupplierService }

func NewSupplierHandler(svc *service.SupplierService) *SupplierHandler {
	return &SupplierHandler{svc: svc}
}

// Add handles POST /api/supplier/add.
func (h *SupplierHandler) Add(c *gin.Context) {
	var req dto.AddSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	supplier, err := h.svc.AddSupplier(c.Request.Context(), req)
	if err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "Success", supplier)
}

// All handles GET /api/supplier/all with optional name/company filters.
func (h *SupplierHandler) All(c *gin.Context) {
	nameSupplier := c.Query("nameSupplier")
	companySupplier := c.Query("companySupplier")

	suppliers, err := h.svc.FilterSuppliers(c.Request.Context(), nameSupplier, companySupplier)
	if err != nil {
		httpx.Respond(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	message := "List supplier berhasil ditemukan"
	if nameSupplier != "" || companySupplier != "" {
		message = "List supplier berhasil difilter"
	}
	httpx.Respond(c, http.StatusOK, message, suppliers)
}

// Paginated handles GET /api/supplier/all/paginated. Note the legacy default
// page size here is 5, not the 10 used elsewhere.
func (h *SupplierHandler) Paginated(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "5"))
	nameSupplier := c.Query("nameSupplier")
	companySupplier := c.Query("companySupplier")

	result, err := h.svc.GetAllSupplierPaginated(c.Request.Context(), nameSupplier, companySupplier, page, size)
	if err != nil {
		httpx.Respond(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "Success", result)
}

// GetAll handles GET /api/supplier/getall, the unpaginated full representation.
func (h *SupplierHandler) GetAll(c *gin.Context) {
	suppliers, err := h.svc.GetAllSuppliers(c.Request.Context())
	if err != nil {
		httpx.Respond(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "Seluruh supplier berhasil ditemukan", suppliers)
}

// Update handles PUT /api/supplier/update. The id travels in the body.
func (h *SupplierHandler) Update(c *gin.Context) {
	var req dto.UpdateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	supplier, err := h.svc.UpdateSupplier(c.Request.Context(), req)
	if err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "Data supplier berhasil diperbarui", supplier)
}

// Name handles GET /api/supplier/name/{supplierId}, used by other services.
func (h *SupplierHandler) Name(c *gin.Context) {
	name, err := h.svc.GetSupplierName(c.Request.Context(), c.Param("supplierId"))
	if err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "OK", name)
}

// AddPurchase handles PUT /api/supplier/add-purchase, called by the purchase
// service to link a purchase to its supplier.
func (h *SupplierHandler) AddPurchase(c *gin.Context) {
	var req dto.AddPurchaseIDRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	if err := h.svc.AddPurchaseID(c.Request.Context(), req.SupplierID, req.PurchaseID); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "Data supplier berhasil diperbarui", nil)
}

// Detail handles GET /api/supplier/detail/{supplierId}.
func (h *SupplierHandler) Detail(c *gin.Context) {
	detail, err := h.svc.GetSupplierDetail(c.Request.Context(), c.Param("supplierId"))
	if err != nil {
		message := err.Error()
		if errors.Is(err, service.ErrSupplierNotFound) {
			message = "Supplier not found"
		}
		httpx.Respond(c, http.StatusBadRequest, message, nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "Supplier detail berhasil ditemukan", detail)
}
