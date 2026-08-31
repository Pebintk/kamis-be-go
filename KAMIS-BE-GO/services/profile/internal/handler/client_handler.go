package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/profile/internal/dto"
	"github.com/karina/kamis-be-go/services/profile/internal/service"
)

type ClientHandler struct{ svc *service.ClientService }

func NewClientHandler(svc *service.ClientService) *ClientHandler { return &ClientHandler{svc: svc} }

// Add handles POST /api/client/add.
func (h *ClientHandler) Add(c *gin.Context) {
	var req dto.AddClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	client, err := h.svc.AddClient(c.Request.Context(), req)
	if err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "Success", client)
}

// All handles GET /api/client/all with the optional name/type/profit filters.
func (h *ClientHandler) All(c *gin.Context) {
	nameClient := c.Query("nameClient")
	typeClient := queryBool(c, "typeClient")
	minProfit := queryInt64(c, "minProfit")
	maxProfit := queryInt64(c, "maxProfit")

	clients, err := h.svc.FilterClients(c.Request.Context(), nameClient, typeClient, minProfit, maxProfit)
	if err != nil {
		httpx.Respond(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	message := "List Client berhasil ditemukan"
	if nameClient != "" || typeClient != nil {
		message = "List Client berhasil difilter"
	}
	httpx.Respond(c, http.StatusOK, message, clients)
}

// Paginated handles GET /api/client/all/paginated.
func (h *ClientHandler) Paginated(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	nameClient := c.Query("nameClient")
	typeClient := queryBool(c, "typeClient")
	minProfit := queryInt64(c, "minProfit")
	maxProfit := queryInt64(c, "maxProfit")

	result, err := h.svc.FilterClientsPaginated(c.Request.Context(), nameClient, typeClient, minProfit, maxProfit, page, size)
	if err != nil {
		httpx.Respond(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	message := "List Client berhasil ditemukan"
	if nameClient != "" || typeClient != nil || minProfit != nil || maxProfit != nil {
		message = "List Client berhasil difilter"
	}
	httpx.Respond(c, http.StatusOK, message, result)
}

// Detail handles GET /api/client/{id}.
func (h *ClientHandler) Detail(c *gin.Context) {
	id := c.Param("id")
	client, err := h.svc.GetClientByID(c.Request.Context(), id)
	if err != nil {
		httpx.Respond(c, http.StatusNotFound, "Client dengan ID "+id+" tidak ditemukan", nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "Detail Client berhasil ditemukan", client)
}

// Update handles PUT /api/client/update/{id}.
func (h *ClientHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req dto.UpdateClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	client, err := h.svc.UpdateClient(c.Request.Context(), id, req)
	if err != nil {
		// The legacy controller answers 404 for every failure here, not just a
		// missing client.
		httpx.Respond(c, http.StatusNotFound, "Client dengan ID "+id+" tidak ditemukan", nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "Client berhasil diperbarui", client)
}

// queryBool returns nil when the parameter is absent, matching an omitted
// Spring `Boolean` request param. It also returns nil for an unparseable value,
// where Spring would answer 400 — a deliberate simplification, since the
// frontend only ever sends real booleans here.
func queryBool(c *gin.Context, name string) *bool {
	raw, ok := c.GetQuery(name)
	if !ok || raw == "" {
		return nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return nil
	}
	return &v
}

func queryInt64(c *gin.Context, name string) *int64 {
	raw, ok := c.GetQuery(name)
	if !ok || raw == "" {
		return nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}
