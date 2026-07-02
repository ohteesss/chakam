# Makefile

.PHONY: dev run build test migrate-up migrate-down

# Development with auto-reload (using air)
dev:
	air

# Run application
run:
	go run cmd/api/main.go

run-worker:
	go run cmd/worker/main.go

# Build binary
build:
	go build -o bin/trustconfirm-api cmd/api/main.go

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Database migrations
migrate-up:
	migrate -path internal/infrastructure/database/migrations \
		-database "${DATABASE_URL}" up

migrate-down:
	migrate -path internal/infrastructure/database/migrations \
		-database "${DATABASE_URL}" down

# Run linter
lint:
	golangci-lint run

# Docker commands
docker-build:
	docker build -t trustconfirm:latest .

docker-run:
	docker-compose up -d

docker-stop:
	docker-compose down

# Database commands
db-create:
	createdb chakam

db-drop:
	dropdb chakam

db-reset: db-drop db-create migrate-up

# Install tools
install-tools:
	go install github.com/air-verse/air@latest
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest