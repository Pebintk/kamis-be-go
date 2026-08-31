package httpx

import (
	"time"

	"github.com/gin-gonic/gin"
)

// Envelope is the response wrapper every KAMIS service returns — the Go analog
// of the Java BaseResponseDTO<T>. The frontend reads `response.data.data`.
type Envelope struct {
	Status    int       `json:"status"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// Respond writes an Envelope with the given status, message and payload.
func Respond(c *gin.Context, status int, message string, data any) {
	c.JSON(status, Envelope{
		Status:    status,
		Message:   message,
		Timestamp: time.Now(),
		Data:      data,
	})
}
