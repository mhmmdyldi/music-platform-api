# syntax=docker/dockerfile:1

# Production image: one image, three binaries.
#
#   /app/api          the HTTP API (default entrypoint)
#   /app/migrate      schema migrations; run once per release before the API
#   /app/healthcheck  liveness probe for container HEALTHCHECKs
#
# The same image is used locally (docker-compose.yml), in stage and in
# production, so what was tested is exactly what ships.

ARG GO_VERSION=1.26

# ---------------------------------------------------------------- build
FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src

# CA certificates for outbound TLS (the database in stage and production), and
# an unprivileged user, both copied into the empty runtime image below.
RUN apk add --no-cache ca-certificates && \
    echo "nonroot:x:65532:65532:nonroot:/nonexistent:/sbin/nologin" > /etc/passwd.runtime

# Dependencies first, so they are cached across source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=unknown
# The module path is read from go.mod instead of being repeated here, so a
# rename (scripts/rename-module.sh) does not have to touch this file.
# `timetzdata` embeds the time zone database, which the empty image lacks.
RUN MODULE="$(go list -m)" && \
    CGO_ENABLED=0 go build -trimpath -tags timetzdata \
      -ldflags "-s -w -X ${MODULE}/internal/buildinfo.Version=${VERSION} -X ${MODULE}/internal/buildinfo.Commit=${COMMIT}" \
      -o /out/ ./cmd/...

# ---------------------------------------------------------------- runtime
# An empty image: no shell, no package manager, nothing but the static
# binaries, CA certificates and an unprivileged user. Built FROM scratch rather
# than a distroless base so that building needs no registry beyond Docker Hub.
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /etc/passwd.runtime /etc/passwd
COPY --from=build /out/api /out/migrate /out/healthcheck /app/
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 CMD ["/app/healthcheck"]
ENTRYPOINT ["/app/api"]
