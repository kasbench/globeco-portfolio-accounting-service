# GlobeCo Portfolio Accounting Service Makefile

# Variables
APP_NAME := globeco-portfolio-accounting-service
DOCKER_IMAGE := $(APP_NAME):latest
GO_VERSION := 1.23.4
PORT := 8087

# Go commands
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOLINT := golangci-lint

# Binary names
SERVER_BINARY := bin/server
CLI_BINARY := bin/cli

# Default target
.DEFAULT_GOAL := help

## help: Display this help message
.PHONY: help
help:
	@echo "Available commands:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## setup: Initialize development environment
.PHONY: setup
setup:
	@echo "Setting up development environment..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Development environment ready!"

## build: Build the application binaries
.PHONY: build
build: build-server build-cli

## build-server: Build the server binary
.PHONY: build-server
build-server:
	@echo "Building server..."
	mkdir -p bin
	$(GOBUILD) -o $(SERVER_BINARY) ./cmd/server

## build-cli: Build the CLI binary
.PHONY: build-cli
build-cli:
	@echo "Building CLI..."
	mkdir -p bin
	$(GOBUILD) -o $(CLI_BINARY) ./cmd/cli

## run: Run the server locally
.PHONY: run
run:
	@echo "Starting server on port $(PORT)..."
	$(GOCMD) run ./cmd/server

## run-cli: Run the CLI application
.PHONY: run-cli
run-cli:
	@echo "Running CLI..."
	$(GOCMD) run ./cmd/cli $(ARGS)

## test: Run all tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

## test-unit: Run unit tests only
.PHONY: test-unit
test-unit:
	@echo "Running unit tests..."
	$(GOTEST) -v -race ./internal/...

## test-integration: Run integration tests
.PHONY: test-integration
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -v -race ./tests/...

## test-api-integration: Run API integration tests that catch service initialization bugs
.PHONY: test-api-integration
test-api-integration:
	@echo "Running API integration tests..."
	$(GOTEST) -v -race ./tests/integration/api_integration_test.go ./tests/integration/database_integration_test.go

## test-database-integration: Run database integration tests only
.PHONY: test-database-integration
test-database-integration:
	@echo "Running database integration tests..."
	$(GOTEST) -v -race ./tests/integration/database_integration_test.go

## coverage: Generate test coverage report
.PHONY: coverage
coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## fmt: Format Go code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

## lint: Run linter
.PHONY: lint
lint:
	@echo "Running linter..."
	$(GOLINT) run

## clean: Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf bin/
	rm -f coverage.out coverage.html

## deps: Download and tidy dependencies
.PHONY: deps
deps:
	@echo "Managing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

## docker-build: Build Docker image
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE) .

## docker-run: Run application in Docker
.PHONY: docker-run
docker-run:
	@echo "Running Docker container..."
	docker run -p $(PORT):$(PORT) --rm $(DOCKER_IMAGE)

## docker-compose-up: Start services with docker-compose
.PHONY: docker-compose-up
docker-compose-up:
	@echo "Starting services with docker-compose..."
	docker-compose up -d

## docker-compose-down: Stop services with docker-compose
.PHONY: docker-compose-down
docker-compose-down:
	@echo "Stopping services with docker-compose..."
	docker-compose down

## migrate-up: Run database migrations up
.PHONY: migrate-up
migrate-up:
	@echo "Running database migrations up..."
	migrate -path migrations -database "postgres://user:password@localhost:5432/dbname?sslmode=disable" up

## migrate-down: Run database migrations down
.PHONY: migrate-down
migrate-down:
	@echo "Running database migrations down..."
	migrate -path migrations -database "postgres://user:password@localhost:5432/dbname?sslmode=disable" down

## migrate-create: Create a new migration file
.PHONY: migrate-create
migrate-create:
	@if [ -z "$(NAME)" ]; then echo "Usage: make migrate-create NAME=migration_name"; exit 1; fi
	@echo "Creating migration: $(NAME)"
	migrate create -ext sql -dir migrations $(NAME)

## dev: Start development environment
.PHONY: dev
dev: docker-compose-up
	@echo "Development environment started!"
	@echo "Database: localhost:5432"
	@echo "Cache: localhost:5701"
	@echo "Server will run on: localhost:$(PORT)"

## install-tools: Install development tools
.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/golangci-lint/golangci-lint/cmd/golangci-lint@latest
	@echo "Tools installed!"

## check: Run all checks (format, lint, test)
.PHONY: check
check: fmt lint test
	@echo "All checks passed!"

## ci: Run CI pipeline checks
.PHONY: ci
ci: deps fmt lint test coverage
	@echo "CI pipeline completed!"

## release: Build release binaries
.PHONY: release
release: clean
	@echo "Building release binaries..."
	mkdir -p bin
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o bin/$(APP_NAME)-linux-amd64 ./cmd/server
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o bin/$(APP_NAME)-darwin-amd64 ./cmd/server
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o bin/$(APP_NAME)-windows-amd64.exe ./cmd/server
	@echo "Release binaries built in bin/" 