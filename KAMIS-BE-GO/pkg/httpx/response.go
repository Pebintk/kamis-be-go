// Package httpx holds small HTTP helpers shared across services so error and
// success responses have a consistent JSON shape.
package httpx

import "github.com/gin-gonic/gin"

// Error aborts the request with a uniform {"error": "..."} body.
func Error(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}
