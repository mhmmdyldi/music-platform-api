# Working on platform-backend

Rules for AI agents and humans changing this repository. Read this first,
then [docs/architecture.md](docs/architecture.md).

## Verify every change

```bash
make check     # gofmt, vet, golangci-lint, unit and integration tests
make up && make verify   # when touching Docker, Compose, config or startup
```

A change is not done until `make check` passes. CI runs the same checks plus
the Docker stack, and blocks the merge on failure.

## Ground rules

1. **International English** for all code, identifiers, API fields, errors, logs,
   comments and docs.
2. **Every feature ships with tests.** Logic gets unit tests. Anything touching
   SQL gets integration tests using `testdb.NewPool(t)`. Every new endpoint gets
   an HTTP test.
3. **The OpenAPI document is the contract.** Add or change a route in
   `internal/httpapi/router.go` *and* in `api/openapi.yaml`. A test fails if
   they disagree.
4. **Errors are problem+json** via `httpserver.WriteProblem`, with a stable
   snake_case `code` such as `room_not_found`. Clients branch on `code`.
5. **Schema changes are new migration files** (`make migration-new NAME=...`).
   Never edit a migration that has been merged. Every migration must have a
   working `-- +goose Down`. Keep changes backward compatible with the previous
   release (expand, then contract in a later release).
6. **Configuration is environment variables only**, declared and validated in
   `internal/config`. New settings get a default or are required, are
   documented in `docs/configuration.md`, and are never secrets in `config/*.env`.
7. **Wiring happens only in `internal/app`.** Packages receive their
   dependencies as constructor arguments; no globals, no `init()` side effects.
8. **The core is source-agnostic.** Nothing in this repository may branch on
   whether a Music Source is a human DJ or the AI DJ. See
   [docs/architecture.md](docs/architecture.md#music-sources).
9. **Logs are structured** (`log/slog`) and include `request_id` when inside a
   request. Never log secrets: use `config.Database.RedactedURL()` and friends.
10. **No new dependency without a reason** written in the PR description.
    Prefer the standard library.

## Where things go

| Adding... | Put it in |
|---|---|
| A business module (rooms, sources, ...) | `internal/modules/<name>/` (domain types, service, store interface, Postgres store, HTTP handlers, tests) and register its routes in `internal/httpapi` |
| Cross-cutting infrastructure | `internal/platform/<name>/` (no business logic) |
| A new binary | `cmd/<name>/`, thin `main` that calls into `internal/` |
| An architectural decision | `docs/adr/NNNN-title.md` |

## Determinism

Tests must not depend on wall-clock timing, ordering of map iteration, network
access beyond the test database, or the machine's environment. Pass time and
randomness in as dependencies when logic depends on them.
