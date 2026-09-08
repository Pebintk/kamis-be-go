package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/services/asset/internal/dto"
	"github.com/pebintk/kamis-be-go/services/asset/internal/service"
)

type MaintenanceHandler struct{ svc *service.MaintenanceService }

func NewMaintenanceHandler(svc *service.MaintenanceService) *MaintenanceHandler {
	return &MaintenanceHandler{svc: svc}
}

// Create handles POST /api/asset/maintenance.
func (h *MaintenanceHandler) Create(c *gin.Context) {
	var req dto.MaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	maintenance, err := h.svc.Create(c.Request.Context(), req)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusCreated, "Maintenance berhasil dicatat", maintenance)
}

// All handles GET /api/asset/maintenance/all.
func (h *MaintenanceHandler) All(c *gin.Context) {
	records, err := h.svc.ListAll(c.Request.Context())
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Daftar maintenance berhasil diambil", records)
}

// InProgress handles GET /api/asset/maintenance/in-progress.
//
// An empty list comes back as 200 with []. Java threw when nothing was open, so
// the controller answered 404 with a null body — but no vehicle being in the
// workshop is a normal state, and the frontend's handler already falls back to
// an empty array on a 200.
func (h *MaintenanceHandler) InProgress(c *gin.Context) {
	records, err := h.svc.ListInProgress(c.Request.Context())
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Daftar aset dalam maintenance berhasil diambil", records)
}

// Complete handles PATCH /api/asset/maintenance/{id}/complete.
func (h *MaintenanceHandler) Complete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Respond(c, http.StatusBadRequest, "id harus berupa angka: "+c.Param("id"), nil)
		return
	}
	maintenance, completeErr := h.svc.Complete(c.Request.Context(), id)
	if completeErr != nil {
		httpx.RespondError(c, completeErr)
		return
	}
	httpx.Respond(c, http.StatusOK, "Maintenance berhasil diselesaikan", maintenance)
}
