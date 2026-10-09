# platform-backend

The core backend of a global live music platform. Listeners join **rooms** where a
**Music Source** (a human DJ, the AI DJ service, or any future source) plays live
audio. This service is the control plane: it will decide who may broadcast or
listen, and track what is live where. The audio itself flows through a media
server, never through this API.

> **Status: foundation only.** This repository currently contains the service
> skeleton: configuration, logging, HTTP server, health checks, database and
> migrations, tests, Docker and CI. No business features (users, clubs, events,
> rooms, sources) exist yet.
>
> The Go module path is `github.com/mhmmdyldi/music-platform-api`. The product
> has no final name yet; see
> [docs/adr/0006-temporary-codename.md](docs/adr/0006-temporary-codename.md).

## Quick start

Requirements: **Docker** (with Compose v2) and **Go 1.26+**. `make`, `curl` and `bash` are
assumed.

```bash
make up        # build the image and start the local stack
make verify    # check every service end to end (prints PASS/FAIL per check)
```

Then:

| What | Where |
|---|---|
| API liveness | <http://localhost:8080/livez> |
| API readiness | <http://localhost:8080/readyz> |
| Test audio stream (HLS) | <http://localhost:8888/live/test-source/index.m3u8> (open in Safari, VLC or `ffplay`) |
| MinIO console | <http://localhost:9101> (user `platform-local`, password `local-only-password`) |
| MediaMTX API | <http://localhost:9997/v3/paths/list> |

Stop with `make down` (keeps data) or `make reset` (deletes data).

## Common commands

```bash
make help              # every target with a one-line description
make test              # unit tests, no Docker needed
make test-integration  # integration tests (starts the Compose Postgres)
make lint              # gofmt, go vet, golangci-lint
make check             # lint + unit + integration: what CI runs, minus Docker
make run               # run the API natively (go run) against the Compose Postgres
make logs SERVICE=api  # follow one service's logs
```

## Repository layout

```
cmd/
  api/            HTTP API entrypoint
  migrate/        schema migration command (up | down | status)
  healthcheck/    container health probe (the image has no shell or curl)
internal/
  app/            composition root: builds and wires every component
  config/         environment-variable configuration and validation
  httpapi/        route table and middleware stack
  platform/       infrastructure with no business logic
    database/     PostgreSQL pool and migration runner
    health/       liveness and readiness probes
    httpserver/   server lifecycle, middleware, JSON / problem+json responses
    logging/      structured logging (log/slog)
  buildinfo/      version stamped at build time
  testutil/       test helpers (testdb: a fresh database per test)
api/openapi.yaml  the HTTP API contract
db/migrations/    versioned SQL migrations (embedded into the binaries)
config/           committed env files: local.env, stage.env, production.env
deploy/local/     configuration for local-only services (MediaMTX)
scripts/          verify-local-stack.sh, rename-module.sh
docs/             architecture, configuration, environments, testing, ADRs
```

## Documentation

- [docs/architecture.md](docs/architecture.md): how the service is structured, and the Music Source model
- [docs/local-development.md](docs/local-development.md): the local stack in detail, ports, troubleshooting
- [docs/configuration.md](docs/configuration.md): every setting and its rules
- [docs/environments.md](docs/environments.md): local, test, stage, production and how they are kept apart
- [docs/testing.md](docs/testing.md): test layers and how to write each kind
- [docs/adr/](docs/adr/): architecture decision records
- [AGENTS.md](AGENTS.md): working rules for AI agents (and humans) changing this code

## Workspace

This repository is one of several independent repositories checked out side by
side under a local `music-platform/` directory (backend, web, mobile, config,
infra, AI DJ). Each builds, tests and deploys on its own; the parent directory
is not a monorepo.
