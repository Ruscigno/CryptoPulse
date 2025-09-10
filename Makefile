# CryptoPulse dYdX Order Routing Service Makefile

# Variables
BINARY_NAME=cryptopulse
DOCKER_COMPOSE=docker compose
GO_FILES=$(shell find . -name "*.go" -type f -not -path "./vendor/*")
MIGRATION_DIR=pkg/database/migrations

# Default target
.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development Environment
.PHONY: dev-up
dev-up: ## Start local development environment
	$(DOCKER_COMPOSE) up -d postgres redis
	@echo "Waiting for PostgreSQL to be ready..."
	@until $(DOCKER_COMPOSE) exec postgres pg_isready -U cryptopulse -d cryptopulse; do sleep 1; done
	@echo "Development environment is ready!"

.PHONY: dev-down
dev-down: ## Stop local development environment
	$(DOCKER_COMPOSE) down

.PHONY: dev-logs
dev-logs: ## Show logs from development environment
	$(DOCKER_COMPOSE) logs -f

.PHONY: dev-clean
dev-clean: ## Clean development environment (remove volumes)
	$(DOCKER_COMPOSE) down -v
	docker system prune -f

# Database Management
.PHONY: migrate-up
migrate-up: ## Apply database migrations
	@echo "Applying database migrations..."
	@if ! command -v migrate >/dev/null 2>&1; then \
		echo "Installing golang-migrate..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	migrate -path $(MIGRATION_DIR) -database "postgres://cryptopulse:cryptopulse_dev@localhost:5432/cryptopulse?sslmode=disable" up

.PHONY: migrate-down
migrate-down: ## Rollback database migrations
	@echo "Rolling back database migrations..."
	@if ! command -v migrate >/dev/null 2>&1; then \
		echo "Installing golang-migrate..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	migrate -path $(MIGRATION_DIR) -database "postgres://cryptopulse:cryptopulse_dev@localhost:5432/cryptopulse?sslmode=disable" down

.PHONY: migrate-create
migrate-create: ## Create a new migration (usage: make migrate-create NAME=migration_name)
	@if [ -z "$(NAME)" ]; then \
		echo "Error: NAME is required. Usage: make migrate-create NAME=migration_name"; \
		exit 1; \
	fi
	@if ! command -v migrate >/dev/null 2>&1; then \
		echo "Installing golang-migrate..."; \
		go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest; \
	fi
	migrate create -ext sql -dir $(MIGRATION_DIR) -seq $(NAME)

.PHONY: db-shell
db-shell: ## Connect to database shell
	$(DOCKER_COMPOSE) exec postgres psql -U cryptopulse -d cryptopulse

.PHONY: db-reset
db-reset: migrate-down migrate-up ## Reset database (drop and recreate tables)

# Application Build and Run
.PHONY: build
build: ## Build the application
	go build -o bin/$(BINARY_NAME) cmd/main.go

.PHONY: run
run: ## Run the application locally
	@if [ -f .env.local ]; then \
		echo "Loading environment from .env.local..."; \
		set -a && source .env.local && set +a && go run cmd/main.go; \
	else \
		echo "No .env.local file found. Please copy .env.example to .env.local and configure it."; \
		exit 1; \
	fi

.PHONY: install-air
install-air: ## Install Air for hot reloading
	@if ! command -v air >/dev/null 2>&1; then \
		echo "Installing Air (compatible version)..."; \
		go install github.com/cosmtrek/air@v1.49.0; \
	else \
		echo "Air is already installed"; \
	fi

.PHONY: run-dev
run-dev: install-air ## Run the application with hot reloading
	@if [ -f .env.local ]; then \
		echo "Loading environment from .env.local..."; \
		set -a && source .env.local && set +a && air -c .air.toml; \
	else \
		echo "No .env.local file found. Please copy .env.example to .env.local and configure it."; \
		exit 1; \
	fi

.PHONY: dev
dev: install-air ## Alias for run-dev (run with hot reloading)
	@if [ -f .env.local ]; then \
		echo "Loading environment from .env.local..."; \
		set -a && source .env.local && set +a && air -c .air.toml; \
	else \
		echo "No .env.local file found. Please copy .env.example to .env.local and configure it."; \
		exit 1; \
	fi

# Testing
.PHONY: test
test: ## Run unit tests
	go test -v ./pkg/... ./tests/unit/...

.PHONY: test-unit
test-unit: ## Run unit tests only
	go test -v ./tests/unit/...

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "Running integration tests..."
	INTEGRATION_TESTS=true go test -v ./tests/integration/...

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	@echo "Running end-to-end tests..."
	E2E_TESTS=true go test -v ./tests/e2e/...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./pkg/... ./tests/unit/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: test-all
test-all: ## Run all tests
	@echo "Running unit tests..."
	go test -v ./tests/unit/...
	@echo "Running integration tests (if INTEGRATION_TESTS=true)..."
	@if [ "$$INTEGRATION_TESTS" = "true" ]; then \
		go test -v ./tests/integration/...; \
	else \
		echo "Skipping integration tests (set INTEGRATION_TESTS=true to run)"; \
	fi
	@echo "Running end-to-end tests (if E2E_TESTS=true)..."
	@if [ "$$E2E_TESTS" = "true" ]; then \
		go test -v ./tests/e2e/...; \
	else \
		echo "Skipping e2e tests (set E2E_TESTS=true to run)"; \
	fi

# Code Quality
.PHONY: lint
lint: ## Run code linting
	golangci-lint run

.PHONY: fmt
fmt: ## Format code
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: mod-tidy
mod-tidy: ## Tidy go modules
	go mod tidy

.PHONY: mod-download
mod-download: ## Download go modules
	go mod download

# Cleanup
.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	rm -rf tmp/
	rm -f coverage.out coverage.html
	rm -f build-errors.log

# Docker Operations
.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t $(BINARY_NAME):latest .

.PHONY: docker-run
docker-run: ## Run application in Docker
	$(DOCKER_COMPOSE) up app

# Development Setup
.PHONY: setup
setup: ## Set up development environment
	@echo "Setting up development environment..."
	@if [ ! -f .env.local ]; then cp .env.example .env.local; echo "Created .env.local from .env.example"; fi
	go mod download
	$(MAKE) dev-up
	$(MAKE) migrate-up
	@echo "Development environment setup complete!"

.PHONY: deps
deps: ## Install development dependencies
	go install github.com/air-verse/air@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Utility targets
.PHONY: check
check: fmt vet lint test ## Run all checks (format, vet, lint, test)

.PHONY: pre-commit
pre-commit: mod-tidy fmt vet lint test ## Run pre-commit checks

# Docker Operations - Preprod
.PHONY: docker-build-preprod
docker-build-preprod: ## Build Docker image for preprod
	docker build -f Dockerfile.preprod -t $(BINARY_NAME):preprod .

.PHONY: deploy-preprod
deploy-preprod: ## Deploy to preprod environment
	@if [ ! -f .env.preprod ]; then \
		echo "Error: .env.preprod file not found. Copy .env.preprod.example and configure it."; \
		exit 1; \
	fi
	$(DOCKER_COMPOSE) -f docker-compose.preprod.yml --env-file .env.preprod up -d --build
	@echo "Preprod deployment started. Check logs with: make logs-preprod"

.PHONY: logs-preprod
logs-preprod: ## Show preprod logs
	$(DOCKER_COMPOSE) -f docker-compose.preprod.yml logs -f

.PHONY: stop-preprod
stop-preprod: ## Stop preprod environment
	$(DOCKER_COMPOSE) -f docker-compose.preprod.yml down

# Docker Operations - Production
.PHONY: docker-build-prod
docker-build-prod: ## Build Docker image for production
	docker build -f Dockerfile.prod -t $(BINARY_NAME):prod .

.PHONY: deploy-prod
deploy-prod: ## Deploy to production environment
	@if [ ! -f .env.prod ]; then \
		echo "Error: .env.prod file not found. Copy .env.prod.example and configure it."; \
		exit 1; \
	fi
	@echo "WARNING: This will deploy to PRODUCTION. Are you sure? (y/N)"
	@read -r REPLY; \
	if [ "$$REPLY" = "y" ] || [ "$$REPLY" = "Y" ]; then \
		$(DOCKER_COMPOSE) -f docker-compose.prod.yml --env-file .env.prod up -d --build; \
		echo "Production deployment started. Check logs with: make logs-prod"; \
	else \
		echo "Production deployment cancelled."; \
	fi

.PHONY: logs-prod
logs-prod: ## Show production logs
	$(DOCKER_COMPOSE) -f docker-compose.prod.yml logs -f

.PHONY: stop-prod
stop-prod: ## Stop production environment
	@echo "WARNING: This will stop PRODUCTION services. Are you sure? (y/N)"
	@read -r REPLY; \
	if [ "$$REPLY" = "y" ] || [ "$$REPLY" = "Y" ]; then \
		$(DOCKER_COMPOSE) -f docker-compose.prod.yml down; \
		echo "Production services stopped."; \
	else \
		echo "Operation cancelled."; \
	fi

# Health Checks
.PHONY: health-check
health-check: ## Check application health
	@echo "Checking application health..."
	@curl -f http://localhost:8080/health || echo "Health check failed"

# Production
.PHONY: build-prod
build-prod: ## Build production binary
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/$(BINARY_NAME) cmd/main.go
