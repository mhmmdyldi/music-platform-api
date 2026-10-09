# 0007. A fresh database per integration test

**Status:** accepted

## Context

An earlier project used as a reference ran repository tests against one long-lived database
configured through a committed `.env`. Tests shared state, depended on
whatever was already there, and ran schema changes on the side.

## Decision

- Integration tests use the build tag `integration`.
- Each test gets a new, empty database (`platform_it_<random>_test`), created
  from `TEST_DATABASE_URL` and dropped with `WITH (FORCE)` on cleanup.
- Locally that is the Compose Postgres; in CI, a GitHub Actions service
  container. Testcontainers was considered and rejected for now: it adds a
  large dependency tree, and CI already provides a database service.
- A missing `TEST_DATABASE_URL` fails the test instead of skipping it.

## Consequences

- Tests are isolated, parallel-safe and order-independent.
- Creating a database costs about 100 ms per test. Acceptable now; templates
  (`CREATE DATABASE ... TEMPLATE`) can speed it up later.
