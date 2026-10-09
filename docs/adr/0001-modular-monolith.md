# 0001. Modular monolith in Go with a single composition root

**Status:** accepted

## Context

The platform will grow many business areas (identity, clubs, events, rooms,
Music Sources, access control, gamification, premium). The team is small,
development is heavily AI-assisted, and the product must be able to scale
later. An earlier project that served as a reference showed what to avoid:
services depending on concrete repositories, dependency cycles resolved by
setter injection, very large service files, and database models returned
directly as API payloads.

## Decision

- One deployable Go service, organised into modules under `internal/modules/`
  with explicit boundaries.
- `internal/app` is the only place that constructs and wires components.
  No globals, no `init()` side effects.
- Modules depend on small interfaces they define; implementations are injected.
- Separate types for database rows, domain objects and API payloads.
- Infrastructure without business logic lives in `internal/platform/`.

## Consequences

- One build, one deploy, one database: simple to operate for the MVP.
- Clear seams allow extracting a module into its own service later.
- Some boilerplate (mapping between types) is accepted in exchange for a
  stable public API.
