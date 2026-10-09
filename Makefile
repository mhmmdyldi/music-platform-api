# Every routine command for this repository. Run `make help` for the list.
#
# Targets that need the local database start the Compose Postgres themselves,
# so each target works from a fresh clone with only Go and Docker installed.

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

# Admin connection used by integration tests to create throwaway databases.
TEST_DATABASE_URL ?= postgres://platform:local-only-password@localhost:$(or $(POSTGRES_HOST_PORT),5433)/postgres?sslmode=disable
# Loads config/local.env into the environment of a native (non-Docker) command.
LOCAL_ENV := set -a && . ./config/local.env && set +a

.PHONY: help
help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------- build & run

.PHONY: build
build: ## Build all binaries into ./bin
	CGO_ENABLED=0 go build -trimpath -o bin/ ./cmd/...

.PHONY: run
run: db-up ## Run the API natively against the Compose Postgres (config/local.env)
	$(LOCAL_ENV) && go run ./cmd/migrate up && go run ./cmd/api

.PHONY: migrate-up migrate-down migrate-status
migrate-up: db-up ## Apply migrations to the local database
	$(LOCAL_ENV) && go run ./cmd/migrate up
migrate-down: db-up ## Roll back the latest migration on the local database
	$(LOCAL_ENV) && go run ./cmd/migrate down
migrate-status: db-up ## Show migration status of the local database
	$(LOCAL_ENV) && go run ./cmd/migrate status

.PHONY: migration-new
migration-new: ## Create a new migration file: make migration-new NAME=create_rooms
	@test -n "$(NAME)" || { echo "usage: make migration-new NAME=snake_case_description"; exit 1; }
	@last=$$(ls db/migrations/*.sql | sed -E 's#.*/([0-9]+)_.*#\1#' | sort -n | tail -1); \
	 next=$$(printf '%05d' $$((10#$$last + 1))); \
	 file=db/migrations/$${next}_$(NAME).sql; \
	 printf -- '-- +goose Up\n\n-- +goose Down\n' > $$file; \
	 echo "created $$file"

# ---------------------------------------------------------------- quality

.PHONY: fmt
fmt: ## Format all Go code
	gofmt -w .

.PHONY: lint
lint: ## gofmt check, go vet and golangci-lint (if installed)
	@unformatted=$$(gofmt -l .); if [[ -n "$$unformatted" ]]; then echo "not gofmt-ed:"; echo "$$unformatted"; exit 1; fi
	go vet ./...
	go vet -tags=integration ./...
	@if command -v golangci-lint >/dev/null; then golangci-lint run ./...; \
	 else echo "golangci-lint not installed; skipped (CI runs it). Install: https://golangci-lint.run/welcome/install/"; fi

.PHONY: test
test: ## Unit tests (no Docker needed)
	go test -race -count=1 ./...

.PHONY: test-integration
test-integration: db-up ## Integration tests against the Compose Postgres
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test -race -count=1 -tags=integration ./...

.PHONY: cover
cover: db-up ## Unit + integration tests with a coverage report (coverage.html)
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test -count=1 -tags=integration -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1

.PHONY: check
check: lint test test-integration ## Everything CI checks, except the Docker stack

# ---------------------------------------------------------------- docker stack

.PHONY: db-up
db-up: ## Start only Postgres and wait until it is healthy
	docker compose up -d --wait postgres

.PHONY: up
up: ## Build and start the full local stack
	docker compose up -d --build --wait --wait-timeout 180 postgres minio mediamtx api
	docker compose up -d minio-init test-source

.PHONY: verify
verify: ## Verify the running local stack end to end
	./scripts/verify-local-stack.sh

.PHONY: down
down: ## Stop the local stack (keeps data)
	docker compose down

.PHONY: reset
reset: ## Stop the local stack and delete its data volumes
	docker compose down -v

.PHONY: logs
logs: ## Follow logs of the local stack (SERVICE=api to narrow)
	docker compose logs -f $(SERVICE)

.PHONY: ps
ps: ## Show local stack containers, including exited jobs
	docker compose ps -a

.PHONY: docker-build
docker-build: ## Build the production image
	docker build --build-arg VERSION=$$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
	  --build-arg COMMIT=$$(git rev-parse HEAD 2>/dev/null || echo unknown) -t platform-backend:dev .
