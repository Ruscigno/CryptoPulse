# Variables
APP_NAME = crypto-pulse
BINARY_NAME = crypto-pulse
DOCKER_COMPOSE = docker compose
GO = go
GOFLAGS = -v

# Default target
.PHONY: all
all: build

# Build the Go binary
.PHONY: build
build:
	$(GO) build $(GOFLAGS) -o $(BINARY_NAME) ./cmd/server/main.go

# Run the app locally (without Docker)
.PHONY: run-crypto-pulse
run-crypto-pulse: build
	./$(BINARY_NAME) serve

.PHONY: run-finviz-scraper
run-finviz-scraper: build
	./$(BINARY_NAME) finviz-scraper

# Run tests
.PHONY: test
test:
	$(GO) test $(GOFLAGS) ./...

# Format code
.PHONY: fmt
fmt:
	$(GO) fmt ./...

# Lint code (requires golangci-lint)
.PHONY: lint
lint:
	golangci-lint run

# Build the Docker image for the app
.PHONY: docker-build
docker-build:
	$(DOCKER_COMPOSE) build app

# Start MongoDB services only (database profile)
.PHONY: database-up
database-up:
	$(DOCKER_COMPOSE) --profile database up -d

# Start the app and MongoDB (app profile includes mongodb dependency)
.PHONY: crypto-pulse-app-up
crypto-pulse-app-up: docker-build
	$(DOCKER_COMPOSE) --profile crypto-pulse-app up -d

.PHONY: finviz-scraper-app-up
finviz-scraper-app-up: docker-build
	$(DOCKER_COMPOSE) --profile finviz-scraper-app up -d

# Start all services (database and app profiles)
.PHONY: all-up
all-up: docker-build database-up crypto-pulse-app-up finviz-scraper-app-up

# Stop all services
.PHONY: down
down:
	$(DOCKER_COMPOSE) down

# Stop and remove volumes (clean slate)
.PHONY: down-clean
down-clean:
	$(DOCKER_COMPOSE) down -v

# View logs for all services
.PHONY: logs
logs:
	$(DOCKER_COMPOSE) logs -f

# View logs for MongoDB only
.PHONY: logs-mongo
logs-mongo:
	$(DOCKER_COMPOSE) logs -f mongodb mongo-express

# View logs for the app only
.PHONY: logs-app
logs-app:
	$(DOCKER_COMPOSE) logs -f app

# Clean up build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME)

# Install dependencies
.PHONY: deps
deps:
	$(GO) mod tidy
	$(GO) mod download

# Help
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make build        - Build the Go binary"
	@echo "  make run          - Run the app locally"
	@echo "  make test         - Run tests"
	@echo "  make fmt          - Format Go code"
	@echo "  make lint         - Lint Go code (requires golangci-lint)"
	@echo "  make docker-build - Build the Docker image for the app"
	@echo "  make database-up  - Start all database services"
	@echo "  make app-up       - Start the app and its dependencies"
	@echo "  make all-up       - Start all services (MongoDB + app)"
	@echo "  make down         - Stop all services"
	@echo "  make down-clean   - Stop all services and remove volumes"
	@echo "  make logs         - View logs for all services"
	@echo "  make logs-mongo   - View logs for MongoDB services"
	@echo "  make logs-app     - View logs for the app"
	@echo "  make clean        - Remove build artifacts"
	@echo "  make deps         - Install Go dependencies"
	@echo "  make help         - Show this help message"