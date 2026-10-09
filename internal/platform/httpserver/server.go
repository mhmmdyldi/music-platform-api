// Package httpserver holds the HTTP plumbing shared by every API: the server
// lifecycle, middleware and the JSON / problem+json response helpers. It
// contains no routes and no business logic.
package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Timeouts configures the server. net/http's zero values mean "no timeout",
// which lets a slow client hold a connection open forever, so all are required.
type Timeouts struct {
	ReadHeader time.Duration
	Read       time.Duration
	Write      time.Duration
	Idle       time.Duration
	Shutdown   time.Duration
}

// Serve accepts connections on ln until ctx is cancelled, then stops accepting
// new connections and waits up to t.Shutdown for in-flight requests to finish.
// It returns nil after a clean shutdown.
func Serve(ctx context.Context, ln net.Listener, handler http.Handler, t Timeouts, logger *slog.Logger) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: t.ReadHeader,
		ReadTimeout:       t.Read,
		WriteTimeout:      t.Write,
		IdleTimeout:       t.Idle,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	logger.Info("http server listening", slog.String("addr", ln.Addr().String()))

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server stopped unexpectedly: %w", err)
	case <-ctx.Done():
	}

	logger.Info("http server shutting down", slog.Duration("timeout", t.Shutdown))
	shutdownCtx, cancel := context.WithTimeout(context.Background(), t.Shutdown)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http server shutdown: %w", err)
	}
	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Info("http server stopped")
	return nil
}
