package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/httpx"
)

// GinAuth validates the Bearer token and stores the claims in the request
// context. It replaces the legacy per-service JwtTokenFilter.
func (v *Verifier) GinAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := bearer(c.GetHeader("Authorization"))
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		claims, err := v.Parse(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		ctx := context.WithValue(c.Request.Context(), ctxKey{}, claims)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// GinRequireRole enforces that the token holds at least one of the allowed
// roles. It mirrors Spring's hasAnyAuthority(...) and must run after GinAuth.
func GinRequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := FromContext(c.Request.Context())
		if !ok || !claims.HasAnyRole(roles...) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

func bearer(header string) string {
	if rest, found := strings.CutPrefix(header, "Bearer "); found {
		return strings.TrimSpace(rest)
	}
	return ""
}

// ---- raw token forwarding ----

// ForwardToken stashes the raw bearer token (when present) on the request
// context so the service layer can forward it on inter-service calls. It is the
// Go stand-in for the Java services injecting HttpServletRequest and reading the
// Authorization header themselves.
//
// Unlike GinAuth it never rejects a request — apply it globally, including on
// public routes, which in the legacy app also forward whatever token they were
// given. The token itself lives in pkg/httpx, which is what consumes it.
func ForwardToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if raw := bearer(c.GetHeader("Authorization")); raw != "" {
			c.Request = c.Request.WithContext(httpx.ContextWithToken(c.Request.Context(), raw))
		}
		c.Next()
	}
}
