//go:build integration

package database_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mhmmdyldi/music-platform-api/db/migrations"
	"github.com/mhmmdyldi/music-platform-api/internal/platform/database"
	"github.com/mhmmdyldi/music-platform-api/internal/testutil/testdb"
)

func TestOpen_ConnectsAndUsesUTC(t *testing.T) {
	t.Parallel()
	pool := testdb.NewPool(t)

	if err := database.Ping(pool)(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}

	var tz string
	if err := pool.QueryRow(context.Background(), "SHOW timezone").Scan(&tz); err != nil {
		t.Fatal(err)
	}
	if tz != "UTC" {
		t.Fatalf("session time zone = %q, want UTC", tz)
	}
}

func TestOpen_UnreachableDatabaseFailsFast(t *testing.T) {
	t.Parallel()
	start := time.Now()
	_, err := database.Open(context.Background(), database.PoolOptions{
		// Port 1 on localhost: nothing listens there.
		URL:            "postgres://u:secret-password@127.0.0.1:1/platform_test?sslmode=disable",
		MaxConns:       1,
		ConnectTimeout: 2 * time.Second,
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if time.Since(start) > 10*time.Second {
		t.Fatalf("took %v to fail", time.Since(start))
	}
	if strings.Contains(err.Error(), "secret-password") {
		t.Fatalf("error leaks the password: %v", err)
	}
}

// Every migration must apply on an empty database, roll back completely, and
// apply again. This is what makes a bad release recoverable.
func TestMigrations_UpDownUp(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool := testdb.NewPool(t)

	m, err := database.NewMigrator(pool, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })

	pending, err := m.Pending(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if pending == 0 {
		t.Fatal("a fresh database reports no pending migrations")
	}

	applied, err := m.Up(ctx)
	if err != nil {
		t.Fatalf("up: %v", err)
	}
	if len(applied) != pending {
		t.Fatalf("applied %d migrations, %d were pending", len(applied), pending)
	}
	assertPending(t, m, 0)

	if _, err := m.DownAll(ctx); err != nil {
		t.Fatalf("down: %v", err)
	}
	assertPending(t, m, pending)

	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("second up: %v", err)
	}
	assertPending(t, m, 0)

	// A second run is a no-op.
	again, err := m.Up(ctx)
	if err != nil || len(again) != 0 {
		t.Fatalf("re-running up applied %v, err %v", again, err)
	}
}

func assertPending(t *testing.T, m *database.Migrator, want int) {
	t.Helper()
	got, err := m.Pending(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("pending migrations = %d, want %d", got, want)
	}
}
