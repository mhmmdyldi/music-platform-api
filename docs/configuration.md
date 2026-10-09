# Configuration

The service reads **environment variables only**. Parsing and validation live in
[`internal/config/config.go`](../internal/config/config.go). An invalid
configuration stops the process at startup with a list of every problem.

## Settings

| Variable | Required | Default | Notes |
|---|---|---|---|
| `APP_ENV` | yes | | `local`, `test`, `stage` or `production` |
| `APP_SERVICE_NAME` | no | `platform-backend` | Added to every log line |
| `HTTP_ADDR` | no | `:8080` | Listen address |
| `HTTP_READ_HEADER_TIMEOUT` | no | `5s` | Go duration syntax |
| `HTTP_READ_TIMEOUT` | no | `15s` | |
| `HTTP_WRITE_TIMEOUT` | no | `30s` | |
| `HTTP_IDLE_TIMEOUT` | no | `60s` | |
| `HTTP_SHUTDOWN_TIMEOUT` | no | `20s` | Time allowed for in-flight requests on shutdown |
| `LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, `error` |
| `LOG_FORMAT` | no | `json` | `json` or `text` |
| `DATABASE_URL` | yes | | `postgres://user:password@host:port/name?sslmode=...` **Secret** |
| `DATABASE_MAX_CONNS` | no | `10` | Pool size per process |
| `DATABASE_CONNECT_TIMEOUT` | no | `5s` | Initial connect and each readiness ping |

## Safety rules (enforced at startup)

1. **Each environment owns its database.** The database name must end with
   `_<APP_ENV>`: `platform_local`, `platform_stage`, `platform_production`. A stage
   process given the production URL refuses to start.
2. **Stage and production require TLS to the database:** `sslmode=require`,
   `verify-ca` or `verify-full` (use `verify-full`).
3. **Stage and production require JSON logs** and forbid `LOG_LEVEL=debug`.
4. The database password never appears in logs. The config is logged at
   startup with the URL redacted.

## Committed env files

`config/` holds one file per environment, in plain `KEY=VALUE` format (no quotes,
no `$`, no `export`), readable the same way by Docker Compose, the shell and
the tests:

| File | Used by | Contains |
|---|---|---|
| `config/local.env` | `make run`, `docker-compose.yml` | Complete local config. Its database password is a throwaway local value, not a secret. |
| `config/stage.env` | stage deployment | Non-secret values only |
| `config/production.env` | production deployment | Non-secret values only |

`internal/config/config_test.go` loads every file and fails if one is invalid,
names the wrong `APP_ENV`, or (for stage and production) contains anything that
looks like a secret.

## Secrets

Secrets are never committed. In stage and production they live in **AWS Secrets
Manager** (region `eu-central-1`) and are injected into the container
environment by the deployment, one secret per environment:

| Variable | Secret name |
|---|---|
| `DATABASE_URL` | `stage/platform-backend/database-url`, `production/platform-backend/database-url` |

For personal local overrides, create `config/<anything>.local.env` (gitignored)
and load it after `config/local.env`.

## Adding a setting

1. Read it in `Load` with a default, or as required.
2. Validate it in `validate` if it has rules, especially stricter rules for
   `stage` and `production`.
3. Add tests in `config_test.go`.
4. Add it to the table above and, if non-secret, to the env files.
