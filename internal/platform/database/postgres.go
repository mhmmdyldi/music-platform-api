// Package database owns the PostgreSQL connection pool and schema migrations.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PoolOptions configures the connection pool.
type PoolOptions struct {
	URL            string
	MaxConns       int32
	ConnectTimeout time.Duration
}

// Open creates a connection pool and verifies the database is reachable.
// Failing here, at startup, gives a clear error instead of a service that
// starts and then fails every request.
func Open(ctx context.Context, opts PoolOptions) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(opts.URL)
	if err != nil {
		// The parse error may echo the URL, which contains the password.
		return nil, fmt.Errorf("parse database URL: invalid connection string")
	}
	cfg.MaxConns = opts.MaxConns
	cfg.ConnConfig.ConnectTimeout = opts.ConnectTimeout
	// Every timestamp is stored and returned in UTC, whatever the server's
	// default time zone is.
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, opts.ConnectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database %q on %s:%d: %w",
			cfg.ConnConfig.Database, cfg.ConnConfig.Host, cfg.ConnConfig.Port, err)
	}
	return pool, nil
}

// Ping is the readiness check for the database.
func Ping(pool *pgxpool.Pool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return pool.Ping(ctx)
	}
}
