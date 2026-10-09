# Architecture decision records

Short records of decisions that shape the codebase: the context, the decision,
and what it costs. Add one when a choice would surprise a newcomer or would be
expensive to reverse. Number them sequentially, and never rewrite an accepted
record: supersede it with a new one.

| # | Decision |
|---|---|
| [0001](0001-modular-monolith.md) | Modular monolith in Go with a single composition root |
| [0002](0002-postgresql-pgx-goose.md) | PostgreSQL with pgx and goose SQL migrations |
| [0003](0003-standard-library-http-and-logging.md) | Standard library HTTP and slog; RFC 9457 errors |
| [0004](0004-environment-configuration.md) | Environment-variable configuration with enforced environment isolation |
| [0005](0005-live-audio-pipeline.md) | MediaMTX ingest and HLS delivery for live audio |
| [0006](0006-temporary-codename.md) | Temporary codename confined to the module path |
| [0007](0007-test-database-strategy.md) | A fresh database per integration test |
| [0008](0008-container-images.md) | Scratch runtime image; Docker Hub-only builds |
