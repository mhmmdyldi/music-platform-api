# Architecture

## What this service is

`platform-backend` is the **control plane** of a live music platform. It will own
identities, clubs, events, rooms, Music Sources, access rules and the state of
every live session. It does **not** carry audio.

```
                    ┌───────────────────── platform-backend ─────────────────────┐
 Mobile / web  ─────▶  HTTP API (REST + JSON, contract: api/openapi.yaml)         │
 clients              │  modules: identity · clubs · events · rooms · sources ... │
                      │  PostgreSQL                                               │
                      └────────▲───────────────────────────────┬──────────────────┘
                               │ Source API                    │ authorises publishing,
                               │ (same for every source)       │ receives stream events
             ┌─────────────────┴──────┐                ┌───────▼──────────────┐
             │ Music Sources          │── RTMP / SRT ─▶│ Media server         │── HLS ─▶ CDN ─▶ Listeners
             │ DJ app, AI DJ service, │   (audio)      │ (MediaMTX)           │
             │ future sources         │                └──────────────────────┘
             └────────────────────────┘
```

Only the boxes inside `platform-backend` live in this repository. The AI DJ
is a separate repository and service.

## Code structure

The service is a **modular monolith**: one deployable, with code organised so
that each business area could later be extracted if scale demands it.

```
cmd/<binary>/           thin entrypoints: load config, call internal/app
internal/app/           composition root: the only place that wires components
internal/config/        configuration from environment variables
internal/httpapi/       route table + middleware stack
internal/platform/      infrastructure shared by all modules (no business logic)
internal/modules/       business modules (none yet), one directory each
```

Dependency rules:

- `cmd` → `internal/app` → everything else. Nothing imports `internal/app`.
- `internal/modules/*` may import `internal/platform/*`, never the reverse.
- Modules do not import each other's internals. When one needs another, it
  depends on a small interface it defines itself, satisfied in `internal/app`.
- Database rows, domain types and API payloads are separate types. A schema
  change must never silently change the public API.

A future module looks like:

```
internal/modules/rooms/
  room.go           domain types and rules (pure, unit-tested)
  service.go        use cases; depends on the Store interface
  store.go          Store interface
  postgres.go       Store implementation (integration-tested)
  http.go           handlers, registered in internal/httpapi
  *_test.go
```

## Request path

Every request passes through, in order:

1. **RequestID**: reuses a well-formed `X-Request-ID` or generates one; echoed
   on the response and attached to every log line.
2. **AccessLog**: one structured log line per request (status, duration, bytes).
3. **Recover**: a panic becomes a logged 500 instead of a dropped connection.
4. **Route**: Go's `net/http.ServeMux` with method patterns. Unknown paths
   return 404 and wrong methods return 405, both as problem+json.

Errors use RFC 9457 `application/problem+json` with a stable `code` field.

## Health

| Endpoint | Meaning | Checks |
|---|---|---|
| `GET /livez` | The process is alive. Restart it if this fails. | none (never touches dependencies) |
| `GET /readyz` | This instance should receive traffic. | `database` (ping), `schema` (no unapplied migrations), not shutting down |

On SIGTERM the service fails readiness, stops accepting connections, finishes
in-flight requests (up to `HTTP_SHUTDOWN_TIMEOUT`), then exits.

## Data

PostgreSQL is accessed through `pgx` (connection pool). Schema changes are
versioned SQL files in `db/migrations`, applied by the `migrate` binary with
goose, once per release, before the new API version starts. The migrations
are embedded in the binaries, so a release always migrates with exactly the
files it was built with.

All timestamps are UTC (the pool forces the session time zone).

## Music Sources

The platform treats every audio provider identically. A human DJ and the AI DJ
are both **Music Sources**. The core never asks which kind a source is.

```
Music Source → Audio Ingest → Live Audio Engine → Room → Listeners
```

The model the business modules will implement:

| Concept | Meaning |
|---|---|
| **Principal** | Whoever is acting: a `user` (a person, including a human DJ) or a `service_account` (an automated system, such as the AI DJ service). Authentication and authorisation treat both the same way. |
| **Music Source** | A broadcastable identity owned by a principal: profile, `kind` label (`human`, `ai`, `external`, ...), status. `kind` is metadata for display and analytics only. No code path branches on it. |
| **Source credential** | A rotatable ingest key scoped to one source, used to publish audio. |
| **Room** | A listening space. Its schedule says which source may broadcast when. |
| **Source session** | One live broadcast of one source into one room: `scheduled → connecting → live → ended / failed`. State changes come from the media server's events, not from what the source claims. |
| **Source API** | The single API every source uses: authenticate, get assigned slots, get ingest credentials, publish audio over RTMP or SRT, send heartbeats and **now-playing track metadata**, end the session. |
| **Platform events** | Changes such as `source_session.live` are published as webhooks. The AI DJ subscribes to them like any other integration. |

A new kind of source (a scheduled playlist, a relay of an external stream) is
a new client of the Source API. It needs no change to the core.

### Live audio

Listeners do not interact with the DJ in real time, so delivery is optimised
for one-to-many scale rather than sub-second latency (see
[ADR 0005](adr/0005-live-audio-pipeline.md)):

- **Ingest:** sources publish to MediaMTX over RTMP or SRT. MediaMTX will ask this
  service whether a publish is allowed (its HTTP auth hook), and report
  stream start and stop back.
- **Delivery:** MediaMTX packages Low-Latency HLS; a CDN serves listeners.
  Expected delay is a few seconds.
- **Future interactivity:** if rooms ever need sub-second interaction, a WebRTC
  engine (for example LiveKit) can be added for those rooms behind the same
  Music Source model, without changing the Source API.

The local stack runs MediaMTX and a test source that publishes a tone, so the
pipeline can be exercised end to end on a laptop. No backend code touches media yet.

## Not built yet (on purpose)

Authentication, every business module, WebSockets, background workers and the
transactional outbox, metrics and tracing (OpenTelemetry), object storage
access, rate limiting, and deployment manifests. Each will arrive with the
first feature that needs it.
