#!/bin/bash

# CryptoPulse dYdX Order Routing Service - Development Environment Setup Script
# This script sets up the complete development environment

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing_deps=()
    
    if ! command_exists docker; then
        missing_deps+=("docker")
    fi
    
    # Check for Docker Compose (modern version)
    if ! docker compose version >/dev/null 2>&1; then
        missing_deps+=("docker compose (Docker Compose v2)")
    fi
    
    if ! command_exists go; then
        missing_deps+=("go")
    fi
    
    if ! command_exists make; then
        missing_deps+=("make")
    fi
    
    if [ ${#missing_deps[@]} -ne 0 ]; then
        log_error "Missing required dependencies: ${missing_deps[*]}"
        log_error "Please install the missing dependencies and run this script again."
        exit 1
    fi
    
    log_success "All prerequisites are installed"
}

# Setup environment files
setup_environment() {
    log_info "Setting up environment configuration..."
    
    if [ ! -f .env.local ]; then
        if [ -f .env.example ]; then
            cp .env.example .env.local
            log_success "Created .env.local from .env.example"
            log_warning "Please review and update .env.local with your specific configuration"
        else
            log_error ".env.example not found. Cannot create .env.local"
            exit 1
        fi
    else
        log_info ".env.local already exists, skipping creation"
    fi
}

# Install Go dependencies
install_go_dependencies() {
    log_info "Installing Go dependencies..."
    
    go mod download
    go mod tidy
    
    log_success "Go dependencies installed"
}

# Install development tools
install_dev_tools() {
    log_info "Installing development tools..."
    
    # Install Air for hot reloading
    if ! command_exists air; then
        go install github.com/cosmtrek/air@latest
        log_success "Installed Air for hot reloading"
    else
        log_info "Air already installed"
    fi
    
    # Install golangci-lint for linting
    if ! command_exists golangci-lint; then
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
        log_success "Installed golangci-lint"
    else
        log_info "golangci-lint already installed"
    fi
    
    # Install migrate for database migrations
    if ! command_exists migrate; then
        go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
        log_success "Installed migrate tool"
    else
        log_info "migrate tool already installed"
    fi
}

# Start development services
start_services() {
    log_info "Starting development services..."
    
    # Start PostgreSQL and Redis
    docker compose up -d postgres redis

    # Wait for PostgreSQL to be ready
    log_info "Waiting for PostgreSQL to be ready..."
    local max_attempts=30
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        if docker compose exec -T postgres pg_isready -U cryptopulse -d cryptopulse >/dev/null 2>&1; then
            log_success "PostgreSQL is ready"
            break
        fi
        
        if [ $attempt -eq $max_attempts ]; then
            log_error "PostgreSQL failed to start within expected time"
            exit 1
        fi
        
        log_info "Attempt $attempt/$max_attempts - waiting for PostgreSQL..."
        sleep 2
        ((attempt++))
    done
    
    # Wait for Redis to be ready
    log_info "Waiting for Redis to be ready..."
    attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if docker compose exec -T redis redis-cli ping >/dev/null 2>&1; then
            log_success "Redis is ready"
            break
        fi
        
        if [ $attempt -eq $max_attempts ]; then
            log_error "Redis failed to start within expected time"
            exit 1
        fi
        
        log_info "Attempt $attempt/$max_attempts - waiting for Redis..."
        sleep 2
        ((attempt++))
    done
}

# Run database migrations
run_migrations() {
    log_info "Running database migrations..."
    
    # Check if migration files exist
    if [ ! -d "pkg/database/migrations" ]; then
        log_error "Migration directory not found: pkg/database/migrations"
        exit 1
    fi
    
    # Apply migrations using docker compose
    if docker compose exec -T postgres psql -U cryptopulse -d cryptopulse -f /docker-entrypoint-initdb.d/001_initial_schema.sql >/dev/null 2>&1; then
        log_success "Database migrations applied successfully"
    else
        log_warning "Migrations may have already been applied or there was an issue"
    fi
}

# Verify setup
verify_setup() {
    log_info "Verifying setup..."
    
    # Check if services are running
    if ! docker compose ps postgres | grep -q "Up"; then
        log_error "PostgreSQL service is not running"
        exit 1
    fi

    if ! docker compose ps redis | grep -q "Up"; then
        log_error "Redis service is not running"
        exit 1
    fi

    # Test database connection
    if docker compose exec -T postgres psql -U cryptopulse -d cryptopulse -c "SELECT 1;" >/dev/null 2>&1; then
        log_success "Database connection test passed"
    else
        log_error "Database connection test failed"
        exit 1
    fi

    # Test Redis connection
    if docker compose exec -T redis redis-cli ping >/dev/null 2>&1; then
        log_success "Redis connection test passed"
    else
        log_error "Redis connection test failed"
        exit 1
    fi
    
    log_success "All services are running correctly"
}

# Print next steps
print_next_steps() {
    log_success "Development environment setup complete!"
    echo ""
    log_info "Next steps:"
    echo "  1. Review and update .env.local with your configuration"
    echo "  2. Run 'make run-dev' to start the application with hot reloading"
    echo "  3. Run 'make test' to run the test suite"
    echo "  4. Visit http://localhost:8080 to access the API"
    echo ""
    log_info "Useful commands:"
    echo "  - make dev-up      : Start development services"
    echo "  - make dev-down    : Stop development services"
    echo "  - make dev-logs    : View service logs"
    echo "  - make db-shell    : Connect to database shell"
    echo "  - make test        : Run tests"
    echo "  - make lint        : Run code linting"
    echo ""
    log_info "For more commands, run 'make help'"
}

# Main execution
main() {
    log_info "Starting CryptoPulse development environment setup..."
    echo ""
    
    check_prerequisites
    setup_environment
    install_go_dependencies
    install_dev_tools
    start_services
    run_migrations
    verify_setup
    
    echo ""
    print_next_steps
}

# Run main function
main "$@"
