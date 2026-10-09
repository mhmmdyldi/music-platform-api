// Package health serves the liveness and readiness probes.
//
//   - Liveness (/livez) answers "is the process able to serve at all?". It
//     never touches dependencies: a database outage must not make the
//     orchestrator restart every replica.
//   - Readiness (/readyz) answers "should this replica receive traffic?". It
//     runs every registered check and fails while the service is draining
//     during shutdown.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/mhmmdyldi/music-platform-api/internal/platform/httpserver"
)

// Check is one dependency probe. Fn must return promptly once ctx is done.
type Check struct {
	Name string
	Fn   func(ctx context.Context) error
}

// Info identifies the running build in the liveness response, so an operator
// or test can confirm which version is actually deployed.
type Info struct {
	Service     string `json:"service"`
	Environment string `json:"environment"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
}

// Probes holds the readiness checks and the draining flag.
type Probes struct {
	info     Info
	checks   []Check
	timeout  time.Duration
	logger   *slog.Logger
	draining atomic.Bool
}

// New returns probes that run each check with the given timeout.
func New(info Info, timeout time.Duration, logger *slog.Logger, checks ...Check) *Probes {
	return &Probes{info: info, checks: checks, timeout: timeout, logger: logger}
}

// StartDraining makes readiness fail from now on, so the load balancer stops
// sending new traffic before the server shuts down.
func (p *Probes) StartDraining() {
	p.draining.Store(true)
}

type livenessResponse struct {
	Status string `json:"status"`
	Info
}

// Liveness handles GET /livez.
func (p *Probes) Liveness(w http.ResponseWriter, _ *http.Request) {
	httpserver.WriteJSON(w, http.StatusOK, livenessResponse{Status: "ok", Info: p.info})
}

type checkResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type readinessResponse struct {
	Status string                 `json:"status"`
	Checks map[string]checkResult `json:"checks"`
}

// Readiness handles GET /readyz. It answers 200 when every check passes and
// 503 otherwise, naming the failing check and its error so a failure can be
// diagnosed from the response alone.
func (p *Probes) Readiness(w http.ResponseWriter, r *http.Request) {
	resp := readinessResponse{Status: "ready", Checks: map[string]checkResult{}}

	if p.draining.Load() {
		resp.Status = "draining"
		httpserver.WriteJSON(w, http.StatusServiceUnavailable, resp)
		return
	}

	for _, c := range p.checks {
		ctx, cancel := context.WithTimeout(r.Context(), p.timeout)
		err := c.Fn(ctx)
		cancel()

		if err != nil {
			resp.Status = "not_ready"
			resp.Checks[c.Name] = checkResult{Status: "fail", Error: err.Error()}
			p.logger.WarnContext(r.Context(), "readiness check failed",
				slog.String("check", c.Name),
				slog.String("request_id", httpserver.RequestIDFrom(r.Context())),
				slog.String("error", err.Error()))
			continue
		}
		resp.Checks[c.Name] = checkResult{Status: "ok"}
	}

	status := http.StatusOK
	if resp.Status != "ready" {
		status = http.StatusServiceUnavailable
	}
	httpserver.WriteJSON(w, status, resp)
}
