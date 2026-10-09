package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mhmmdyldi/music-platform-api/internal/platform/health"
	"github.com/mhmmdyldi/music-platform-api/internal/platform/httpserver"
)

func testDeps() Dependencies {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	probes := health.New(health.Info{Service: "platform-backend", Environment: "test", Version: "dev"},
		time.Second, logger,
		health.Check{Name: "database", Fn: func(context.Context) error { return nil }})
	return Dependencies{Logger: logger, Probes: probes}
}

func serve(t *testing.T, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	NewHandler(testDeps()).ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestHealthEndpoints(t *testing.T) {
	for _, path := range []string{"/livez", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			rec := serve(t, http.MethodGet, path)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
			}
			if rec.Header().Get(httpserver.RequestIDHeader) == "" {
				t.Fatal("response has no X-Request-ID")
			}
		})
	}
}

func TestUnknownRoute_IsProblemJSON(t *testing.T) {
	rec := serve(t, http.MethodGet, "/does-not-exist")
	p := decodeProblem(t, rec, http.StatusNotFound)
	if p.Code != "route_not_found" || p.RequestID == "" {
		t.Fatalf("problem = %+v", p)
	}
}

func TestWrongMethod_IsProblemJSONWithAllowHeader(t *testing.T) {
	rec := serve(t, http.MethodPost, "/livez")
	p := decodeProblem(t, rec, http.StatusMethodNotAllowed)
	if p.Code != "method_not_allowed" {
		t.Fatalf("problem = %+v", p)
	}
	if allow := rec.Header().Get("Allow"); allow != "GET" {
		t.Fatalf("Allow = %q", allow)
	}
}

// The OpenAPI document is the contract for every client. A route the server
// serves but the document does not describe is a contract violation, and so
// is a documented path that nothing serves.
func TestEveryRouteIsDocumentedInOpenAPI(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	documented := documentedOperations(string(spec))

	served := map[string]bool{}
	for _, rt := range Routes(testDeps()) {
		op := strings.ToLower(rt.Method) + " " + rt.Path
		served[op] = true
		if !documented[op] {
			t.Errorf("%s %s is served but not documented in api/openapi.yaml", rt.Method, rt.Path)
		}
	}
	for op := range documented {
		if !served[op] {
			t.Errorf("%q is documented in api/openapi.yaml but not served", op)
		}
	}
}

var (
	specPath   = regexp.MustCompile(`^  (/\S*):\s*$`)
	specMethod = regexp.MustCompile(`^    (get|put|post|delete|patch|head|options):\s*$`)
)

// documentedOperations reads "method path" pairs from the paths section of
// the spec. It relies on the two-space indentation the file uses, which keeps
// this test free of a YAML dependency.
func documentedOperations(spec string) map[string]bool {
	ops := map[string]bool{}
	inPaths, current := false, ""
	for _, line := range strings.Split(spec, "\n") {
		switch {
		case line == "paths:":
			inPaths = true
		case inPaths && line != "" && !strings.HasPrefix(line, " "):
			inPaths = false
		case inPaths:
			if m := specPath.FindStringSubmatch(line); m != nil {
				current = m[1]
			} else if m := specMethod.FindStringSubmatch(line); m != nil && current != "" {
				ops[m[1]+" "+current] = true
			}
		}
	}
	return ops
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) httpserver.Problem {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, wantStatus, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type = %q", ct)
	}
	var p httpserver.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Status != wantStatus {
		t.Fatalf("problem status %d != HTTP status %d", p.Status, wantStatus)
	}
	return p
}
