package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health is a public liveness probe.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
