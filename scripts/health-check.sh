#!/bin/bash

# CryptoPulse dYdX Order Routing Service - Health Check Script
# This script checks the health of all development services

set -e

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

# Check service health
check_service_health() {
    local service_name="$1"
    local health_command="$2"
    
    log_info "Checking $service_name..."
    
    if eval "$health_command" >/dev/null 2>&1; then
        log_success "$service_name is healthy"
        return 0
    else
        log_error "$service_name is not healthy"
        return 1
    fi
}

# Check PostgreSQL
check_postgres() {
    check_service_health "PostgreSQL" "docker-compose exec -T postgres pg_isready -U cryptopulse -d cryptopulse"
}

# Check Redis
check_redis() {
    check_service_health "Redis" "docker-compose exec -T redis redis-cli ping"
}

# Check application (if running)
check_application() {
    log_info "Checking application..."
    
    if curl -s http://localhost:8080/health >/dev/null 2>&1; then
        log_success "Application is healthy"
        return 0
    else
        log_warning "Application is not running or not healthy"
        return 1
    fi
}

# Show service status
show_service_status() {
    log_info "Service status:"
    echo ""
    docker-compose ps
    echo ""
}

# Show resource usage
show_resource_usage() {
    log_info "Resource usage:"
    echo ""
    docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}\t{{.BlockIO}}"
    echo ""
}

# Main execution
main() {
    log_info "Running health checks for CryptoPulse development environment..."
    echo ""
    
    local failed_checks=0
    
    # Check individual services
    check_postgres || ((failed_checks++))
    check_redis || ((failed_checks++))
    check_application || ((failed_checks++))
    
    echo ""
    show_service_status
    show_resource_usage
    
    # Summary
    if [ $failed_checks -eq 0 ]; then
        log_success "All health checks passed!"
        exit 0
    else
        log_error "$failed_checks health check(s) failed"
        exit 1
    fi
}

# Run main function
main "$@"
