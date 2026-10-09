// Command api serves the platform's HTTP API.
//
// Configuration is read from environment variables; see docs/configuration.md.
// The process stops gracefully on SIGINT or SIGTERM.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mhmmdyldi/music-platform-api/internal/app"
	"github.com/mhmmdyldi/music-platform-api/internal/buildinfo"
	"github.com/mhmmdyldi/music-platform-api/internal/config"
	"github.com/mhmmdyldi/music-platform-api/internal/platform/logging"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}

	logger := logging.New(os.Stdout, cfg.Log.Level, cfg.Log.Format, cfg.ServiceName, string(cfg.Env))
	logger.Info("starting",
		slog.String("version", buildinfo.Version),
		slog.String("commit", buildinfo.Commit),
		slog.Any("config", cfg))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg, logger)
	if err != nil {
		logger.Error("startup failed", slog.String("error", err.Error()))
		return err
	}
	defer a.Close()

	if err := a.Run(ctx); err != nil {
		logger.Error("stopped with error", slog.String("error", err.Error()))
		return err
	}
	return nil
}
