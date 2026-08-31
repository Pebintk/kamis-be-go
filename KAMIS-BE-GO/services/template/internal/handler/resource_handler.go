// Package handler is the transport layer (Spring @RestController).
package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/template/internal/service"
)

type ResourceHandler struct{ svc *service.ResourceService }

func NewResourceHandler(svc *service.ResourceService) *ResourceHandler {
	return &ResourceHandler{svc: svc}
}

func (h *ResourceHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to list resources")
		return
	}
	c.JSON(http.StatusOK, items)
}

func (h *ResourceHandler) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid id")
		return
	}
	res, err := h.svc.Get(c.Request.Context(), uint(id))
	if errors.Is(err, service.ErrNotFound) {
		httpx.Error(c, http.StatusNotFound, "resource not found")
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to get resource")
		return
	}
	c.JSON(http.StatusOK, res)
}

// createResourceRequest demonstrates Gin's declarative validation, the analog of
// Spring's @Valid @RequestBody DTOs.
type createResourceRequest struct {
	Name     string `json:"name" binding:"required"`
	Quantity int    `json:"quantity" binding:"gte=0"`
}

func (h *ResourceHandler) Create(c *gin.Context) {
	var req createResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	res, err := h.svc.Create(c.Request.Context(), req.Name, req.Quantity)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "failed to create resource")
		return
	}
	c.JSON(http.StatusCreated, res)
}
