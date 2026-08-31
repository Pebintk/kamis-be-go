package httpx

import (
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

// SetupLogging installs the process-wide structured logger. Output is JSON so
// the container log collector can parse it; set LOG_LEVEL=debug to widen it.
func SetupLogging(service string) {
	level := slog.LevelInfo
	if err := level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL"))); err != nil {
		level = slog.LevelInfo
	}
	slog.SetDefault(slog.New(
		slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}),
	).With("service", service))
}

// RequestLogger logs one structured line per request. It replaces gin.Logger(),
// whose plain-text output would not match the rest of the process's logs.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		}
		if err := c.Errors.ByType(gin.ErrorTypePrivate).String(); err != "" {
			attrs = append(attrs, "error", err)
		}

		switch {
		case c.Writer.Status() >= 500:
			slog.Error("request", attrs...)
		case c.Writer.Status() >= 400:
			slog.Warn("request", attrs...)
		default:
			slog.Info("request", attrs...)
		}
	}
}
