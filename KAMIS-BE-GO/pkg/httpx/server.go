package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// Server timeouts. A bare http.Server has none, which lets a single slow or
// idle client hold a connection open indefinitely.
//
// WriteTimeout has to clear the slowest handler: the supplier detail endpoint
// fans out to three services that each get DefaultTimeout, so it is set well
// above that ceiling rather than at it.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second

	// shutdownGrace is how long in-flight requests get to finish after a
	// SIGTERM before the process exits anyway.
	shutdownGrace = 20 * time.Second
)

// Serve runs handler on the given port until the process receives SIGINT or
// SIGTERM, then drains in-flight requests before returning.
//
// Use this instead of gin's Engine.Run, which sets no timeouts and gives
// `docker stop` no chance to drain: it kills in-flight requests outright.
func Serve(handler http.Handler, port string) error {
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
			return
		}
		errc <- nil
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	slog.Info("shutting down", "grace", shutdownGrace)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	slog.Info("stopped")
	return nil
}
