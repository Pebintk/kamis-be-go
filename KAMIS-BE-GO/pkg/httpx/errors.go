package httpx

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/karina/kamis-be-go/pkg/apierr"
)

// serverErrorMessage is what a 500 tells the client. The underlying error is
// logged instead of returned: the Java controllers put e.getMessage() straight
// into the response body, so a failed connection handed the browser the database
// DSN with its password in it.
const serverErrorMessage = "Terjadi kesalahan pada server"

// RespondError writes err with the status its type calls for — 400 for a caller
// mistake, 404 for a missing record, 500 for anything else.
func RespondError(c *gin.Context, err error) {
	status := apierr.Status(err)
	message := err.Error()
	if status == http.StatusInternalServerError {
		slog.ErrorContext(c.Request.Context(), "request failed",
			"method", c.Request.Method, "path", c.FullPath(), "error", err)
		message = serverErrorMessage
	}
	Respond(c, status, message, nil)
}
