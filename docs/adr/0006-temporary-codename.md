# 0006. Temporary codename confined to the module path

**Status:** accepted

## Context

The product has no final name. `tempo` is the temporary codename. A rename
must be cheap.

## Decision

- The codename appears only in the Go module path
  (`github.com/tempo-music/platform-backend`) and in the documentation that
  explains it.
- Configuration variables, database names, image names, log fields and the
  Compose project use the neutral name `platform` / `platform-backend`.
- The Dockerfile reads the module path from `go.mod` rather than repeating it.
- `scripts/rename-module.sh <new-path>` rewrites `go.mod` and every import, then
  builds and tests.

## Consequences

- A rename is one script run plus a GitHub repository rename.
- The module path points at a GitHub organisation that may not exist yet. It
  must be created, or the path renamed, before anything imports this module
  from outside.

## Update

The organisation `tempo-music` was never created. The module was renamed with
`scripts/rename-module.sh` to `github.com/mhmmdyldi/music-platform-api`, the
repository it is hosted in. The codename no longer appears in code; every
other part of this decision still holds.
