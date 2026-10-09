package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestRequestID_GeneratesWhenMissing(t *testing.T) {
	var seen string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if len(seen) != 32 {
		t.Fatalf("generated request id %q, want 32 hex chars", seen)
	}
	if got := rec.Header().Get(RequestIDHeader); got != seen {
		t.Fatalf("response header %q != context id %q", got, seen)
	}
}

func TestRequestID_KeepsWellFormedCallerID(t *testing.T) {
	h := RequestID()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "client-abc.123")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get(RequestIDHeader); got != "client-abc.123" {
		t.Fatalf("request id = %q, want the caller's", got)
	}
}

func TestRequestID_ReplacesMalformedCallerID(t *testing.T) {
	h := RequestID()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "bad id\nwith newline")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get(RequestIDHeader); got == "" || strings.Contains(got, " ") {
		t.Fatalf("malformed id was not replaced: %q", got)
	}
}

func TestRecover_ReturnsProblemWithRequestID(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	h := Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }),
		RequestID(), Recover(logger))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(RequestIDHeader, "req-1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content type = %q", ct)
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.Code != "internal_error" || p.RequestID != "req-1" || p.Status != 500 {
		t.Fatalf("problem = %+v", p)
	}
	if !strings.Contains(logs.String(), `"panic":"boom"`) || !strings.Contains(logs.String(), `"request_id":"req-1"`) {
		t.Fatalf("panic not logged with request id: %s", logs.String())
	}
}

func TestAccessLog_RecordsStatusAndRequestID(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hi"))
	}), RequestID(), AccessLog(logger))

	req := httptest.NewRequest(http.MethodPost, "/brew", nil)
	req.Header.Set(RequestIDHeader, "req-2")
	h.ServeHTTP(httptest.NewRecorder(), req)

	var line map[string]any
	if err := json.Unmarshal(logs.Bytes(), &line); err != nil {
		t.Fatalf("access log is not one JSON line: %q", logs.String())
	}
	if line["status"] != float64(418) || line["request_id"] != "req-2" ||
		line["method"] != "POST" || line["path"] != "/brew" || line["bytes"] != float64(2) {
		t.Fatalf("access log = %v", line)
	}
}

func TestAccessLog_QuietPathsLogAtDebugUnlessFailing(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	status := http.StatusOK
	h := AccessLog(logger, "/livez")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/livez", nil))
	if logs.Len() != 0 {
		t.Fatalf("successful probe logged at info: %s", logs.String())
	}

	status = http.StatusServiceUnavailable
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/livez", nil))
	if !strings.Contains(logs.String(), `"status":503`) {
		t.Fatalf("failing probe was not logged: %q", logs.String())
	}
}

func TestChain_FirstMiddlewareIsOutermost(t *testing.T) {
	var order []string
	mw := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}
	Chain(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), mw("a"), mw("b")).
		ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if strings.Join(order, ",") != "a,b" {
		t.Fatalf("order = %v", order)
	}
}

// Cancelling the context must let a request that is already running finish
// before Serve returns: that is what makes rolling deploys lossless.
func TestServe_DrainsInFlightRequestsOnShutdown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		_, _ = w.Write([]byte("done"))
	})

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() {
		served <- Serve(ctx, ln, handler, Timeouts{
			ReadHeader: time.Second, Read: time.Second, Write: 5 * time.Second,
			Idle: time.Second, Shutdown: 5 * time.Second,
		}, discardLogger())
	}()

	body := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			body <- "error: " + err.Error()
			return
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		body <- string(b)
	}()

	<-started
	cancel()
	close(release)

	if got := <-body; got != "done" {
		t.Fatalf("in-flight request got %q", got)
	}
	if err := <-served; err != nil {
		t.Fatalf("Serve returned %v, want nil after clean shutdown", err)
	}
}
