# Makefile
ifneq (,$(wildcard .env))
include .env
export
endif

#
ifeq ($(strip $(DATABASE_URL)),)
DATABASE_URL := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)
endif

.PHONY: db-create db-drop db-reset dev run run-worker build test test-race test-coverage verify migrate-up migrate-down migrate-reset migrate-version migrate-status migrate-create lint docker-build docker-run docker-stop

# Development with auto-reload (using air)
dev:
	air

# Run application
run:
	go run cmd/api/main.go

run-worker:
	go run cmd/worker/main.go

run-health:
	go run cmd/healthcheck/main.go

# Build binary
build:
	mkdir -p bin
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/chakam-api ./cmd/api
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/chakam-worker ./cmd/worker

# Run tests
test:
	go test ./...

test-race:
	go test -race ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Database migrations (goose)
MIGRATIONS_DIR := internal/infrastructure/database/migrations
GOOSE := goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)"

.PHONY: require-db-url
require-db-url:
	@test -n "$(strip $(DATABASE_URL))" || { \
		echo "DATABASE_URL is empty. Set it in .env, or set DB_HOST/DB_USER/DB_PASSWORD/DB_NAME."; \
		exit 1; \
	}

migrate-up: require-db-url
	$(GOOSE) up

# Rolls back a single migration. Use migrate-reset to roll back everything.
migrate-down: require-db-url
	$(GOOSE) down

migrate-reset: require-db-url
	$(GOOSE) reset

migrate-version: require-db-url
	$(GOOSE) version

migrate-status: require-db-url
	$(GOOSE) status

# Usage: make migrate-create name=add_widgets
migrate-create:
	@test -n "$(name)" || { echo "usage: make migrate-create name=<migration_name>"; exit 1; }
	goose -dir $(MIGRATIONS_DIR) create $(name) sql

# Run linter
lint:
	golangci-lint run

verify:
	test -z "$$(gofmt -l $$(find cmd internal -name '*.go' -type f))"
	go vet ./...
	go test ./...
	go build ./cmd/...

# Docker commands
docker-build:
	docker build -t chakam:latest .
	docker build --target migrations -t chakam-migrations:latest .

docker-run:
	docker compose up -d

docker-stop:
	docker compose down

# Database commands. Target the same database .env points at, not a hardcoded name.
db-create:
	createdb -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) $(DB_NAME)

db-drop:
	dropdb --if-exists -h $(DB_HOST) -p $(DB_PORT) -U $(DB_USER) $(DB_NAME)

db-reset: db-drop db-create migrate-up

# Install tools
install-tools:
	go install github.com/air-verse/air@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
