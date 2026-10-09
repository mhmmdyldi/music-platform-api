package health

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var testInfo = Info{Service: "platform-backend", Environment: "test", Version: "v1.2.3", Commit: "abc"}

func newProbes(checks ...Check) *Probes {
	return New(testInfo, 50*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)), checks...)
}

func ok(name string) Check {
	return Check{Name: name, Fn: func(context.Context) error { return nil }}
}

func failing(name, msg string) Check {
	return Check{Name: name, Fn: func(context.Context) error { return errors.New(msg) }}
}

func get(t *testing.T, h http.HandlerFunc) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %q", rec.Body.String())
	}
	return rec.Code, body
}

func TestLiveness_ReportsBuildInfo(t *testing.T) {
	code, body := get(t, newProbes(failing("database", "down")).Liveness)

	// Liveness must not depend on the database.
	if code != http.StatusOK {
		t.Fatalf("status = %d", code)
	}
	if body["status"] != "ok" || body["version"] != "v1.2.3" || body["environment"] != "test" {
		t.Fatalf("body = %v", body)
	}
}

func TestReadiness_AllChecksPass(t *testing.T) {
	code, body := get(t, newProbes(ok("database"), ok("other")).Readiness)

	if code != http.StatusOK || body["status"] != "ready" {
		t.Fatalf("got %d %v", code, body)
	}
	checks := body["checks"].(map[string]any)
	if checks["database"].(map[string]any)["status"] != "ok" {
		t.Fatalf("checks = %v", checks)
	}
}

func TestReadiness_FailingCheckIsNamedWithItsError(t *testing.T) {
	code, body := get(t, newProbes(ok("other"), failing("database", "connection refused")).Readiness)

	if code != http.StatusServiceUnavailable || body["status"] != "not_ready" {
		t.Fatalf("got %d %v", code, body)
	}
	db := body["checks"].(map[string]any)["database"].(map[string]any)
	if db["status"] != "fail" || db["error"] != "connection refused" {
		t.Fatalf("database check = %v", db)
	}
}

func TestReadiness_CheckIsBoundedByTimeout(t *testing.T) {
	slow := Check{Name: "database", Fn: func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}}

	start := time.Now()
	code, _ := get(t, newProbes(slow).Readiness)

	if code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", code)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("readiness took %v; the check timeout was not applied", elapsed)
	}
}

func TestReadiness_FailsWhileDraining(t *testing.T) {
	p := newProbes(ok("database"))
	p.StartDraining()

	code, body := get(t, p.Readiness)
	if code != http.StatusServiceUnavailable || body["status"] != "draining" {
		t.Fatalf("got %d %v", code, body)
	}

	// Liveness is unaffected: a draining process is still alive.
	if code, _ := get(t, p.Liveness); code != http.StatusOK {
		t.Fatalf("liveness while draining = %d", code)
	}
}
