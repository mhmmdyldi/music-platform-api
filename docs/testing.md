# Testing

Automated tests are mandatory for every change. CI blocks merges when any
test fails.

## Layers

| Layer | Command | Needs | What it covers |
|---|---|---|---|
| Unit | `make test` | Go only | Pure logic, HTTP handlers through `httptest`, config validation, middleware |
| Integration | `make test-integration` | Docker (starts Postgres) | Real PostgreSQL: connections, migrations up/down/up, the fully wired app |
| Local stack | `make up && make verify` | Docker | The built image, Compose wiring, MinIO, MediaMTX, test source publishing, HLS output |
| Static | `make lint` | Go (+ golangci-lint) | gofmt, go vet, golangci-lint |

`make check` runs static, unit and integration. CI runs all four layers as
separate jobs (see `.github/workflows/ci.yml`), plus `go mod tidy -diff`
and `govulncheck`.

## Conventions

- **Unit tests** sit next to the code (`foo_test.go`) and must not need Docker,
  the network or environment variables. Configuration is passed in with
  `config.FromMap(...)`, never read from the real environment.
- **Integration tests** carry the build tag `//go:build integration` and are
  named `*_integration_test.go`. They are compiled only with `-tags=integration`.
- **A fresh database per test.** Call `testdb.NewPool(t)` or `testdb.NewURL(t)`
  ([internal/testutil/testdb](../internal/testutil/testdb/testdb.go)). Each
  call creates an empty database named `platform_it_<random>_test` and drops it
  when the test ends. Tests can therefore run in parallel (`t.Parallel()`) and
  never depend on each other's data. Apply migrations explicitly when the test
  needs the schema.
- **Missing database = failure, not skip.** Without `TEST_DATABASE_URL`, an
  integration test fails with instructions, so a misconfigured run cannot pass
  silently.
- **Deterministic:** no sleeps to wait for work. Use channels, or poll with a
  deadline when an external process is involved. Inject time and randomness when
  logic depends on them.
- **Name tests after behaviour**, for example `TestReadiness_FailsWhileDraining`.
  A failing test name should explain what broke.
- **Contract tests:** `TestEveryRouteIsDocumentedInOpenAPI` fails when a route
  exists in code but not in `api/openapi.yaml`, or the reverse.

## Running a subset

```bash
go test ./internal/config/ -run TestLoad_Rejects -v
TEST_DATABASE_URL=postgres://platform:local-only-password@localhost:5433/postgres?sslmode=disable \
  go test -tags=integration ./internal/platform/database/ -run TestMigrations -v
make cover   # unit + integration with coverage.html
```

## Diagnosing failures

- `make verify` prints PASS/FAIL per check with the HTTP status and body it saw.
- `make ps` shows every container, including exited jobs (`migrate`, `minio-init`)
  and their exit codes. `make logs SERVICE=<name>` shows one service's logs.
- `GET /readyz` names the failing dependency and its error.
- Every API response carries `X-Request-ID`. Search the API logs for it.
