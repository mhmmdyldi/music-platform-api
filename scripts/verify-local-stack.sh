#!/usr/bin/env bash
#
# Verifies the local Docker stack end to end. Run it after `make up`.
#
# Every check prints PASS or FAIL with the reason; the script exits non-zero
# if any check fails. Services that are still starting are retried for up to
# VERIFY_TIMEOUT seconds (default 90).
#
# Requires: bash, curl, docker.

set -uo pipefail

API="http://localhost:${API_HOST_PORT:-8080}"
MINIO="http://localhost:${MINIO_API_HOST_PORT:-9100}"
MEDIAMTX_API="http://localhost:${MEDIAMTX_API_HOST_PORT:-9997}"
HLS="http://localhost:${MEDIAMTX_HLS_HOST_PORT:-8888}"
TIMEOUT="${VERIFY_TIMEOUT:-90}"

failures=0

# check NAME COMMAND... : retries COMMAND until it succeeds or TIMEOUT passes.
check() {
  local name="$1"; shift
  local deadline=$((SECONDS + TIMEOUT)) output
  while true; do
    if output="$("$@" 2>&1)"; then
      printf 'PASS  %s\n' "$name"
      return 0
    fi
    if ((SECONDS >= deadline)); then
      printf 'FAIL  %s\n      %s\n' "$name" "$(printf '%s' "$output" | head -c 500)"
      failures=$((failures + 1))
      return 1
    fi
    sleep 2
  done
}

# http_expect URL STATUS [BODY_SUBSTRING]
http_expect() {
  local url="$1" want_status="$2" want_body="${3:-}" body status
  # -L: MediaMTX answers the first HLS request with a cookie-check redirect.
  body="$(curl -sSL -m 5 -w '\n%{http_code}' "$url")" || { echo "request to $url failed"; return 1; }
  status="${body##*$'\n'}"
  body="${body%$'\n'*}"
  if [[ "$status" != "$want_status" ]]; then
    echo "$url returned $status, want $want_status: $body"
    return 1
  fi
  if [[ -n "$want_body" && "$body" != *"$want_body"* ]]; then
    echo "$url body does not contain '$want_body': $body"
    return 1
  fi
}

# hls_has_audio: the multivariant playlist names a media playlist, and that
# playlist lists segments, i.e. audio is actually being packaged.
hls_has_audio() {
  local master media_uri media
  master="$(curl -sSL -m 5 "$HLS/live/test-source/index.m3u8")" || { echo "cannot fetch playlist"; return 1; }
  media_uri="$(printf '%s\n' "$master" | grep -v '^#' | grep -m1 . | tr -d '\r')"
  [[ -n "$media_uri" ]] || { echo "no media playlist in: $master"; return 1; }
  media="$(curl -sSL -m 10 "$HLS/live/test-source/$media_uri")" || { echo "cannot fetch $media_uri"; return 1; }
  [[ "$media" == *"#EXTINF"* ]] || { echo "media playlist has no segments yet: $media"; return 1; }
}

container_exited_ok() {
  local id code
  id="$(docker compose ps -aq "$1")"
  [[ -n "$id" ]] || { echo "service $1 has no container"; return 1; }
  code="$(docker inspect -f '{{.State.Status}} {{.State.ExitCode}}' "$id")"
  [[ "$code" == "exited 0" ]] || { echo "service $1 is '$code', want 'exited 0'"; return 1; }
}

echo "Verifying the local stack (timeout ${TIMEOUT}s per check)"

check "migrate job completed"            container_exited_ok migrate
check "api liveness    GET /livez"       http_expect "$API/livez" 200 '"status":"ok"'
check "api readiness   GET /readyz"      http_expect "$API/readyz" 200 '"status":"ready"'
check "api errors are problem+json"      http_expect "$API/no-such-route" 404 '"code":"route_not_found"'
check "minio is ready"                   http_expect "$MINIO/minio/health/ready" 200
check "minio bucket created"             container_exited_ok minio-init
check "mediamtx api answers"             http_expect "$MEDIAMTX_API/v3/paths/list" 200
check "test source is publishing"        http_expect "$MEDIAMTX_API/v3/paths/get/live/test-source" 200 '"ready":true'
check "hls playlist is served"           http_expect "$HLS/live/test-source/index.m3u8" 200 '#EXTM3U'
check "hls audio segments are produced"  hls_has_audio

if ((failures > 0)); then
  echo "$failures check(s) failed. Inspect with: docker compose ps -a && docker compose logs <service>"
  exit 1
fi
echo "All checks passed."
