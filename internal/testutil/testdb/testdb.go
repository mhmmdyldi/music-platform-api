// Package testdb gives each integration test its own empty PostgreSQL
// database, so tests never share state, can run in parallel and leave nothing
// behind.
//
// It needs TEST_DATABASE_URL: a connection URL for a role allowed to CREATE
// DATABASE, pointing at an existing maintenance database (usually "postgres").
// `make test-integration` sets it for the local Compose Postgres, and CI sets
// it for its Postgres service. When it is missing the test FAILS rather than
// skips, so a misconfigured run can never look green.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mhmmdyldi/music-platform-api/internal/platform/database"
)

// EnvVar names the variable holding the admin connection URL.
const EnvVar = "TEST_DATABASE_URL"

// NewURL creates a fresh, empty database and returns its connection URL. The
// database is dropped when the test ends. Its name ends in "_test", which is
// what config validation requires for APP_ENV=test.
func NewURL(t testing.TB) string {
	t.Helper()

	adminURL := os.Getenv(EnvVar)
	if adminURL == "" {
		t.Fatalf("%s is not set. Start Postgres with `make up` and run `make test-integration`, "+
			"or export %s=postgres://platform:local-only-password@localhost:5433/postgres?sslmode=disable", EnvVar, EnvVar)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect to %s: %v", EnvVar, err)
	}
	defer admin.Close(ctx)

	name := "platform_it_" + randomSuffix() + "_test"
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatalf("create test database: %v", err)
	}

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, adminURL)
		if err != nil {
			t.Errorf("drop test database %s: %v", name, err)
			return
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Errorf("drop test database %s: %v", name, err)
		}
	})

	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatalf("parse %s: %v", EnvVar, err)
	}
	u.Path = "/" + name
	return u.String()
}

// NewPool creates a fresh database and returns a pool connected to it. The
// pool is closed before the database is dropped.
func NewPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	dbURL := NewURL(t)

	pool, err := database.Open(context.Background(), database.PoolOptions{
		URL: dbURL, MaxConns: 4, ConnectTimeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(pool.Close) // Cleanups run last-in-first-out: closed before the drop.
	return pool
}

func randomSuffix() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
