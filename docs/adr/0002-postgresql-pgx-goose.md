# 0002. PostgreSQL with pgx and goose SQL migrations

**Status:** accepted

## Context

The domain is relational: users, clubs, memberships, events, rooms, schedules,
permissions. We need transactions, mature managed hosting on AWS, and a schema
history that is reviewable and reversible. An earlier project used as a reference used GORM
with `AutoMigrate`, which gave no migration history, no rollback and hidden
queries.

## Decision

- **PostgreSQL 17** (Amazon RDS in stage and production).
- **pgx/v5** connection pool. No ORM.
- **goose** for versioned SQL migrations in `db/migrations`, embedded in the
  binaries and applied by `cmd/migrate` once per release, before rollout.
- Every migration has a working `Down`; integration tests run up → down → up.
- When business tables arrive, queries will be written in SQL and generated
  into type-safe Go with **sqlc**, keeping SQL explicit and reviewable.
- Timestamps are `timestamptz` in UTC; new primary keys will be UUIDv7.

## Consequences

- SQL is visible in code review and easy for AI agents to reason about.
- Migrations must be written by hand and kept backward compatible with the
  previous release (expand/contract).
- High-volume event or analytics data will need a different store later.
