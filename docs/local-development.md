# Local development

## Prerequisites

- Docker with Compose v2 (Docker Desktop, OrbStack or Colima on macOS)
- Go 1.26 or newer (`go.mod` pins the toolchain; older Go downloads it automatically)
- `make`, `bash`, `curl`

## The local stack

`make up` builds the backend image and starts:

| Service | Image | Host port(s) | Role |
|---|---|---|---|
| `postgres` | `postgres:17-bookworm` | 5433 | Database `platform_local` |
| `migrate` | backend image | none | Applies migrations, then exits |
| `api` | backend image | 8080 | The HTTP API; starts after `migrate` succeeds |
| `minio` | `pgsty/minio` | 9100 (S3 API), 9101 (console) | S3-compatible storage (not used by the backend yet) |
| `minio-init` | `pgsty/minio` | none | Creates bucket `platform-local-media`, then exits |
| `mediamtx` | `bluenviron/mediamtx:1.21.1-ffmpeg` | 1935 (RTMP), 8890/udp (SRT), 8888 (HLS), 9997 (API) | Media server |
| `test-source` | same as `mediamtx` | none | Publishes a 440 Hz tone to `live/test-source` over RTMP |

All credentials in the stack are throwaway local values (`local-only-password`).

```bash
make up        # start (or update) everything
make verify    # end-to-end checks
make ps        # container states, including finished jobs
make logs SERVICE=api
make down      # stop, keep data
make reset     # stop and delete the Postgres and MinIO volumes
```

### Listening to the test stream

```bash
ffplay http://localhost:8888/live/test-source/index.m3u8
# or open the URL in Safari, or in VLC: Media > Open Network Stream
```

To publish your own audio as another source:

```bash
ffmpeg -re -i some-track.mp3 -c:a aac -b:a 128k -f flv \
  "rtmp://localhost:1935/live/my-source?user=local-source&pass=local-only-password"
# listen: http://localhost:8888/live/my-source/index.m3u8
```

### Port conflicts

Every host port can be changed with an environment variable when you run `make up`
and `make verify`:

`API_HOST_PORT`, `POSTGRES_HOST_PORT`, `MINIO_API_HOST_PORT`, `MINIO_CONSOLE_HOST_PORT`,
`MEDIAMTX_RTMP_HOST_PORT`, `MEDIAMTX_SRT_HOST_PORT`, `MEDIAMTX_HLS_HOST_PORT`,
`MEDIAMTX_API_HOST_PORT`.

```bash
API_HOST_PORT=18080 make up && API_HOST_PORT=18080 make verify
```

If you change `POSTGRES_HOST_PORT`, also change the port in `config/local.env`
for `make run` (the Makefile picks it up for integration tests automatically).

## Running the API natively

For a faster edit-run loop than rebuilding the image:

```bash
make down     # frees port 8080 (or keep the stack and set HTTP_ADDR below)
make run      # starts the Compose Postgres, migrates, then `go run ./cmd/api`
```

`make run` loads `config/local.env`. To override one value:

```bash
set -a && . ./config/local.env && set +a
HTTP_ADDR=:8081 LOG_LEVEL=info go run ./cmd/api
```

## Database

```bash
make migrate-status
make migrate-up
make migrate-down                  # roll back the latest migration
make migration-new NAME=create_rooms
docker compose exec postgres psql -U platform -d platform_local
```

## Troubleshooting

| Symptom | Check |
|---|---|
| `make up` fails on `api` | `make ps`: did `migrate` exit 0? `make logs SERVICE=migrate` |
| `/readyz` returns 503 | The body names the failing check: `database` (Postgres down) or `schema` (migrations not applied) |
| `test source is publishing` fails | `make logs SERVICE=test-source` and `make logs SERVICE=mediamtx` |
| Port already allocated | Override the host port (see above) |
| Image pull fails | Check Docker can reach Docker Hub (`docker pull postgres:17-bookworm`) |
