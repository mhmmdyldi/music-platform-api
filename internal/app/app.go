// Package app is the composition root: the one place that turns a Config into
// running components and wires them together. Nothing else in the codebase
// constructs infrastructure, so how the service is assembled can be read
// top to bottom in this file.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mhmmdyldi/music-platform-api/db/migrations"
	"github.com/mhmmdyldi/music-platform-api/internal/buildinfo"
	"github.com/mhmmdyldi/music-platform-api/internal/config"
	"github.com/mhmmdyldi/music-platform-api/internal/httpapi"
	"github.com/mhmmdyldi/music-platform-api/internal/platform/database"
	"github.com/mhmmdyldi/music-platform-api/internal/platform/health"
	"github.com/mhmmdyldi/music-platform-api/internal/platform/httpserver"
)

// App is the assembled API service.
type App struct {
	cfg     config.Config
	logger  *slog.Logger
	pool    *pgxpool.Pool
	probes  *health.Probes
	handler http.Handler
}

// New connects to dependencies and builds the HTTP handler. It fails fast
// when the database is unreachable.
func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	pool, err := database.Open(ctx, database.PoolOptions{
		URL:            cfg.Database.URL,
		MaxConns:       cfg.Database.MaxConns,
		ConnectTimeout: cfg.Database.ConnectTimeout,
	})
	if err != nil {
		return nil, err
	}

	probes := health.New(
		health.Info{
			Service:     cfg.ServiceName,
			Environment: string(cfg.Env),
			Version:     buildinfo.Version,
			Commit:      buildinfo.Commit,
		},
		cfg.Database.ConnectTimeout,
		logger,
		health.Check{Name: "database", Fn: database.Ping(pool)},
		health.Check{Name: "schema", Fn: schemaUpToDate(pool)},
	)

	return &App{
		cfg:     cfg,
		logger:  logger,
		pool:    pool,
		probes:  probes,
		handler: httpapi.NewHandler(httpapi.Dependencies{Logger: logger, Probes: probes}),
	}, nil
}

// Handler returns the root HTTP handler. Tests serve it with httptest.
func (a *App) Handler() http.Handler {
	return a.handler
}

// Run serves HTTP on the configured address until ctx is cancelled, then
// stops reporting ready and drains in-flight requests.
func (a *App) Run(ctx context.Context) error {
	ln, err := net.Listen("tcp", a.cfg.HTTP.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", a.cfg.HTTP.Addr, err)
	}

	serveCtx, stopServing := context.WithCancel(context.Background())
	defer stopServing()
	go func() {
		<-ctx.Done()
		a.probes.StartDraining()
		stopServing()
	}()

	return httpserver.Serve(serveCtx, ln, a.handler, httpserver.Timeouts{
		ReadHeader: a.cfg.HTTP.ReadHeaderTimeout,
		Read:       a.cfg.HTTP.ReadTimeout,
		Write:      a.cfg.HTTP.WriteTimeout,
		Idle:       a.cfg.HTTP.IdleTimeout,
		Shutdown:   a.cfg.HTTP.ShutdownTimeout,
	}, a.logger)
}

// Close releases the database pool.
func (a *App) Close() {
	a.pool.Close()
}

// schemaUpToDate fails readiness while migrations shipped in this binary are
// not applied, which means the migrate step of the deployment did not run.
// A database that is ahead of the binary is fine: that is the normal state
// during a rolling deploy, while old replicas are still serving.
func schemaUpToDate(pool *pgxpool.Pool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		m, err := database.NewMigrator(pool, migrations.FS)
		if err != nil {
			return err
		}
		defer m.Close()

		pending, err := m.Pending(ctx)
		if err != nil {
			return err
		}
		if pending > 0 {
			return fmt.Errorf("%d migration(s) not applied; run the migrate command", pending)
		}
		return nil
	}
}
