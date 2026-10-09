# 0003. Standard library HTTP and slog; RFC 9457 errors

**Status:** accepted

## Context

Go's `net/http.ServeMux` supports method and path-parameter patterns, and
`log/slog` provides structured logging. Third-party routers and loggers add
dependencies without solving a problem we have.

## Decision

- Routing with `net/http.ServeMux`. All routes are declared in
  `internal/httpapi/router.go` and documented in `api/openapi.yaml`, and a
  test keeps the two in sync.
- Logging with `log/slog`: JSON in deployed environments, text locally,
  timestamps in UTC, `request_id` on every request log line.
- Errors as RFC 9457 `application/problem+json` with a stable `code` field,
  including 404 and 405.
- Spec-first code generation (oapi-codegen) is deferred until there are
  enough endpoints to justify it.

## Consequences

- Very few dependencies and nothing hidden behind framework conventions.
- Middleware (request ID, access log, panic recovery) is written in-house.
  It is small and fully tested.
