// Package httpapi assembles the public HTTP surface: the route table and the
// middleware every request passes through.
//
// The route table below is the single place where endpoints are declared.
// Every route must also be described in api/openapi.yaml; a test enforces it.
package httpapi

import (
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/mhmmdyldi/music-platform-api/internal/platform/health"
	"github.com/mhmmdyldi/music-platform-api/internal/platform/httpserver"
)

// Route is one endpoint.
type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

// Dependencies are everything the handlers need. Business modules will add
// their services here as they are introduced.
type Dependencies struct {
	Logger *slog.Logger
	Probes *health.Probes
}

// Routes returns the full route table.
func Routes(deps Dependencies) []Route {
	return []Route{
		{Method: http.MethodGet, Path: "/livez", Handler: deps.Probes.Liveness},
		{Method: http.MethodGet, Path: "/readyz", Handler: deps.Probes.Readiness},
	}
}

// NewHandler builds the root handler: routes plus middleware.
//
// Unknown paths and unsupported methods answer with problem+json like every
// other error, rather than net/http's plain-text defaults, so clients and
// tests only ever parse one error format.
func NewHandler(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	allowed := map[string][]string{}
	for _, rt := range Routes(deps) {
		mux.HandleFunc(rt.Method+" "+rt.Path, rt.Handler)
		allowed[rt.Path] = append(allowed[rt.Path], rt.Method)
	}

	// A method-less pattern is less specific than "GET /path", so it only
	// receives requests whose method no route accepts.
	for path, methods := range allowed {
		sort.Strings(methods)
		allow := strings.Join(methods, ", ")
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Allow", allow)
			httpserver.WriteProblem(w, r, http.StatusMethodNotAllowed, "method_not_allowed",
				r.Method+" is not supported on "+r.URL.Path+"; allowed: "+allow)
		})
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpserver.WriteProblem(w, r, http.StatusNotFound, "route_not_found",
			"No endpoint exists at "+r.URL.Path)
	})

	return httpserver.Chain(mux,
		httpserver.RequestID(),
		httpserver.AccessLog(deps.Logger, "/livez", "/readyz"),
		httpserver.Recover(deps.Logger),
	)
}
