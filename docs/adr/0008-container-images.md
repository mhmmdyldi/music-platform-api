# 0008. Scratch runtime image; Docker Hub-only builds

**Status:** accepted

## Context

The image must be small, have no shell, and run as non-root. It must build
from any network: during setup, `gcr.io` (the distroless base images) was not
reachable from a developer network. Upstream MinIO no longer publishes
community container images.

## Decision

- Build stage: `golang:<version>-alpine`. Runtime stage: `FROM scratch` with
  the static binaries, CA certificates, embedded time zone data
  (`-tags timetzdata`) and an unprivileged user (UID 65532).
- One image carries `api` (entrypoint), `migrate` and `healthcheck`. The same
  image runs locally, in stage and in production.
- Third-party images are pinned to versions: `postgres:17-bookworm`,
  `bluenviron/mediamtx:1.21.1-ffmpeg`, and `pgsty/minio:RELEASE.2026-08-04T00-00-00Z`
  (the community build maintained by Pigsty).

## Consequences

- Builds need only Docker Hub.
- No shell in the container: debugging uses logs and the HTTP endpoints, and
  health checks use the bundled `healthcheck` binary.
- The MinIO source should be revisited if Pigsty stops maintaining it. Any
  S3-compatible server works for local development; stage and production use AWS S3.
