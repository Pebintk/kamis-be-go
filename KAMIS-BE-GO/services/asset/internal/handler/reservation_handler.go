package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/asset/internal/dto"
	"github.com/karina/kamis-be-go/services/asset/internal/service"
)

type ReservationHandler struct{ svc *service.ReservationService }

func NewReservationHandler(svc *service.ReservationService) *ReservationHandler {
	return &ReservationHandler{svc: svc}
}

// CheckAvailability handles POST /api/asset/reservations/check-availability.
func (h *ReservationHandler) CheckAvailability(c *gin.Context) {
	var req dto.AssetAvailabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	available, err := h.svc.CheckAvailability(c.Request.Context(), req)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Asset availability checked successfully", available)
}

// Reserve handles POST /api/asset/reservations/reserve.
func (h *ReservationHandler) Reserve(c *gin.Context) {
	var req dto.AssetReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	reservations, err := h.svc.Reserve(c.Request.Context(), req)
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusCreated, "Assets reserved successfully", reservations)
}

// UpdateProjectStatus handles PUT /api/asset/reservations/project/{projectId}/status.
// The status arrives as a query parameter, as it did in Java.
func (h *ReservationHandler) UpdateProjectStatus(c *gin.Context) {
	reservations, err := h.svc.UpdateProjectStatus(c.Request.Context(),
		c.Param("projectId"), c.Query("status"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Reservations status updated successfully", reservations)
}

// UpdateStatus handles PUT /api/asset/reservations/{reservationId}/status.
func (h *ReservationHandler) UpdateStatus(c *gin.Context) {
	reservation, err := h.svc.UpdateStatus(c.Request.Context(),
		c.Param("reservationId"), c.Query("status"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Reservation status updated successfully", reservation)
}

// ByAsset handles GET /api/asset/reservations/asset/{platNomor}.
func (h *ReservationHandler) ByAsset(c *gin.Context) {
	reservations, err := h.svc.ByAsset(c.Request.Context(), c.Param("platNomor"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Asset reservations retrieved successfully", reservations)
}

// ByProject handles GET /api/asset/reservations/project/{projectId}.
func (h *ReservationHandler) ByProject(c *gin.Context) {
	reservations, err := h.svc.ByProject(c.Request.Context(), c.Param("projectId"))
	if err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Project reservations retrieved successfully", reservations)
}
