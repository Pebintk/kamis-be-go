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

type ProfileHandler struct{ svc *service.UserService }

func NewProfileHandler(svc *service.UserService) *ProfileHandler { return &ProfileHandler{svc: svc} }

// Add handles POST /api/profile/add (public).
func (h *ProfileHandler) Add(c *gin.Context) {
	var req dto.AddUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	user, err := h.svc.AddUser(c.Request.Context(), req)
	if err != nil {
		httpx.Respond(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "User added successfully", user)
}

// All handles GET /api/profile/all (Admin only).
func (h *ProfileHandler) All(c *gin.Context) {
	users, err := h.svc.GetAllUsers(c.Request.Context())
	if err != nil {
		httpx.Respond(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}
	httpx.Respond(c, http.StatusOK, "Success", users)
}

// Paginated handles GET /api/profile/all/paginated with optional filters.
func (h *ProfileHandler) Paginated(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "10"))
	username := c.Query("username")
	email := c.Query("email")
	userType := c.Query("userType")

	result, err := h.svc.GetAllUsersPaginated(c.Request.Context(), page, size, email, username, userType)
	if err != nil {
		httpx.Respond(c, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	message := "List Users berhasil ditemukan"
	if username != "" || email != "" || userType != "" {
		message = "List Users berhasil difilter"
	}
	httpx.Respond(c, http.StatusOK, message, result)
}

// Update handles PUT /api/profile/{id}. Per the legacy service, the path value
// is the user's email, not the UUID.
func (h *ProfileHandler) Update(c *gin.Context) {
	email := c.Param("id")
	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, err.Error(), nil)
		return
	}
	user, err := h.svc.UpdateUser(c.Request.Context(), email, req)
	switch {
	case errors.Is(err, service.ErrUserExists):
		httpx.Respond(c, http.StatusConflict, "Email or username already exists", nil)
	case err != nil:
		httpx.Respond(c, http.StatusInternalServerError, err.Error(), nil)
	default:
		httpx.Respond(c, http.StatusOK, "User updated successfully", user)
	}
}
