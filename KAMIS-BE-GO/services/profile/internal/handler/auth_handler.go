// Package handler is the profile service's transport layer (the Java
// restcontroller package). Every response uses the shared httpx.Envelope.
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pebintk/kamis-be-go/pkg/auth"
	"github.com/pebintk/kamis-be-go/pkg/httpx"
	"github.com/pebintk/kamis-be-go/services/profile/internal/dto"
	"github.com/pebintk/kamis-be-go/services/profile/internal/service"
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

// Refresh handles POST /api/auth/refresh (public). It authenticates by the
// refresh token in the body, so it needs no bearer — the access token it
// replaces has usually expired by the time this is called.
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, "Invalid request body", nil)
		return
	}

	resp, err := h.svc.Refresh(c.Request.Context(), req.RefreshToken)
	switch {
	case errors.Is(err, service.ErrInvalidRefreshToken):
		// One message for unknown, expired and already-spent, so a caller
		// cannot learn which of their guesses was once real.
		httpx.Respond(c, http.StatusUnauthorized, "Refresh token tidak valid", nil)
	case err != nil:
		httpx.RespondError(c, err)
	default:
		httpx.Respond(c, http.StatusOK, "Token refreshed", resp)
	}
}

// Logout handles POST /api/auth/logout (public). It revokes the refresh token,
// which is what stops new access tokens being minted. The access token already
// issued stays valid until it expires — see MIGRATION.md on why that window is
// bounded rather than closed.
//
// An unrecognised token still answers 200: logging out is not a place to
// confirm what exists.
func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Respond(c, http.StatusBadRequest, "Invalid request body", nil)
		return
	}
	if err := h.svc.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		httpx.RespondError(c, err)
		return
	}
	httpx.Respond(c, http.StatusOK, "Logout berhasil", nil)
}
