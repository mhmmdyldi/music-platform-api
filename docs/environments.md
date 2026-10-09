# Environments

| Environment | Where | Database | Purpose |
|---|---|---|---|
| `local` | a developer machine, Docker Compose | `platform_local` in the Compose Postgres | development |
| `test` | `go test` (locally and in CI) | a fresh `platform_it_<random>_test` per test | automated tests |
| `stage` | AWS `eu-central-1` | `platform_stage` on its own RDS instance | pre-production verification |
| `production` | AWS `eu-central-1` | `platform_production` on its own RDS instance | real users |

## Isolation rules

Stage and production are separate in every dimension. Nothing in one may
reach the other:

- **Separate AWS accounts** (or at minimum separate VPCs) for stage and production.
- **Separate databases** on separate instances. The backend enforces the naming
  rule (`_<APP_ENV>` suffix) at startup, so a swapped `DATABASE_URL` fails
  loudly instead of using the wrong data.
- **Separate secrets** in AWS Secrets Manager, namespaced `stage/...` and
  `production/...`, readable only by that environment's workload.
- **Separate media servers, buckets and CDN distributions** when those are added.
- **No production data in stage.** Stage data is synthetic or seeded.
- **Stricter config in both:** TLS to the database, JSON logs, no debug logging.

The project is hosted entirely on international infrastructure (AWS, GitHub).
It has no connection to, or dependency on, any other organisation's
infrastructure.

## Release flow (planned)

CI currently builds and tests only. Deployment will be added in the
infrastructure and config repositories, following this shape:

1. A pull request runs CI: static checks, unit tests, integration tests, Docker stack.
2. Merging to `main` builds the image once, tagged with the commit SHA, and pushes it to the registry.
3. **Stage** deploys that image automatically: run `/app/migrate up` as a one-off
   task, then roll out `/app/api`. A smoke test checks `/readyz`.
4. **Production** promotes the *same image digest* after an explicit approval.
   It is never rebuilt.

Migrations must stay backward compatible with the previously deployed version
(expand, migrate, then contract in a later release), because old and new
replicas run side by side during a rollout.

## Running a binary against an environment

Every binary reads the same variables, so pointing one at an environment is a matter of
its environment variables. For example, to inspect stage migrations from an
authorised machine:

```bash
set -a && . config/stage.env && set +a
export DATABASE_URL="$(aws secretsmanager get-secret-value --region eu-central-1 \
  --secret-id stage/platform-backend/database-url --query SecretString --output text)"
go run ./cmd/migrate status
```

`migrate down` refuses to run in production without `-allow-production-down`.
