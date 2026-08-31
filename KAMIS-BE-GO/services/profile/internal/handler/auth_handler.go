// Package handler is the profile service's transport layer (the Java
// restcontroller package). Every response uses the shared httpx.Envelope.
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/auth"
	"github.com/karina/kamis-be-go/pkg/httpx"
	"github.com/karina/kamis-be-go/services/profile/internal/dto"
	"github.com/karina/kamis-be-go/services/profile/internal/service"
)

type AuthHandler struct{ svc *service.UserService }

func NewAuthHandler(svc *service.UserService) *AuthHandler { return &AuthHandler{svc: svc} }

// Login handles POST /api/auth/login (public). Mirrors AuthRestController:
// 404 when the email is unknown, 401 on a bad password, 200 with {token} on success.
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	resp, err := h.svc.Login(c.Request.Context(), req)
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		httpx.Respond(c, http.StatusNotFound, "User not found", nil)
	case errors.Is(err, auth.ErrInvalidCredentials):
		httpx.Respond(c, http.StatusUnauthorized, "Login failed: Invalid username or password", nil)
	case err != nil:
		httpx.Respond(c, http.StatusUnauthorized, "Login failed: "+err.Error(), nil)
	default:
		httpx.Respond(c, http.StatusOK, "Login successful", resp)
	}
}
