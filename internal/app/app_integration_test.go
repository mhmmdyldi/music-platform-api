//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mhmmdyldi/music-platform-api/db/migrations"
	"github.com/mhmmdyldi/music-platform-api/internal/app"
	"github.com/mhmmdyldi/music-platform-api/internal/config"
	"github.com/mhmmdyldi/music-platform-api/internal/platform/database"
	"github.com/mhmmdyldi/music-platform-api/internal/testutil/testdb"
)

func newConfig(t *testing.T, dbURL, addr string) config.Config {
	t.Helper()
	cfg, err := config.Load(config.FromMap(map[string]string{
		"APP_ENV":      "test",
		"DATABASE_URL": dbURL,
		"HTTP_ADDR":    addr,
	}))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func migrate(t *testing.T, dbURL string) {
	t.Helper()
	pool, err := database.Open(context.Background(), database.PoolOptions{URL: dbURL, MaxConns: 1, ConnectTimeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	m, err := database.NewMigrator(pool, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	if _, err := m.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type readiness struct {
	Status string `json:"status"`
	Checks map[string]struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	} `json:"checks"`
}

func getReadiness(t *testing.T, baseURL string) (int, readiness) {
	t.Helper()
	resp, err := http.Get(baseURL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body readiness
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, body
}

// The full service against a real database: not ready until the schema is
// migrated, ready afterwards, and alive throughout.
func TestApp_ReadinessFollowsSchema(t *testing.T) {
	t.Parallel()
	dbURL := testdb.NewURL(t)

	a, err := app.New(context.Background(), newConfig(t, dbURL, "127.0.0.1:0"), quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)

	srv := httptest.NewServer(a.Handler())
	t.Cleanup(srv.Close)

	code, body := getReadiness(t, srv.URL)
	if code != http.StatusServiceUnavailable || body.Checks["schema"].Status != "fail" {
		t.Fatalf("before migrating: %d %+v", code, body)
	}
	if body.Checks["database"].Status != "ok" {
		t.Fatalf("database check failed: %+v", body.Checks["database"])
	}

	migrate(t, dbURL)

	code, body = getReadiness(t, srv.URL)
	if code != http.StatusOK || body.Status != "ready" {
		t.Fatalf("after migrating: %d %+v", code, body)
	}

	resp, err := http.Get(srv.URL + "/livez")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("livez = %d", resp.StatusCode)
	}
}

func TestApp_RunStopsCleanlyOnCancel(t *testing.T) {
	t.Parallel()
	dbURL := testdb.NewURL(t)
	migrate(t, dbURL)

	addr := freeAddr(t)
	a, err := app.New(context.Background(), newConfig(t, dbURL, addr), quietLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	waitUntilServing(t, "http://"+addr+"/livez")
	if code, _ := getReadiness(t, "http://"+addr); code != http.StatusOK {
		t.Fatalf("readyz = %d", code)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestApp_NewFailsWhenDatabaseIsUnreachable(t *testing.T) {
	t.Parallel()
	cfg := newConfig(t, "postgres://u:p@127.0.0.1:1/platform_test?sslmode=disable", "127.0.0.1:0")
	cfg.Database.ConnectTimeout = 2 * time.Second

	if _, err := app.New(context.Background(), cfg, quietLogger()); err == nil {
		t.Fatal("expected startup to fail")
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().String()
}

func waitUntilServing(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("%s never answered", url)
}
