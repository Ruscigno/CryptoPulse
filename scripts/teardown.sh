#!/bin/bash

# CryptoPulse dYdX Order Routing Service - Development Environment Teardown Script
# This script tears down the development environment

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

# Parse command line arguments
CLEAN_VOLUMES=false
CLEAN_IMAGES=false
CLEAN_ALL=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --volumes)
            CLEAN_VOLUMES=true
            shift
            ;;
        --images)
            CLEAN_IMAGES=true
            shift
            ;;
        --all)
            CLEAN_ALL=true
            CLEAN_VOLUMES=true
            CLEAN_IMAGES=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --volumes    Remove Docker volumes (will delete all data)"
            echo "  --images     Remove Docker images"
            echo "  --all        Remove everything (volumes, images, containers)"
            echo "  -h, --help   Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                    # Stop services only"
            echo "  $0 --volumes          # Stop services and remove volumes"
            echo "  $0 --all              # Complete cleanup"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# Confirm destructive operations
confirm_action() {
    local message="$1"
    local default="${2:-n}"
    
    if [ "$default" = "y" ]; then
        local prompt="$message [Y/n]: "
    else
        local prompt="$message [y/N]: "
    fi
    
    read -p "$prompt" -n 1 -r
    echo
    
    if [ "$default" = "y" ]; then
        [[ $REPLY =~ ^[Nn]$ ]] && return 1 || return 0
    else
        [[ $REPLY =~ ^[Yy]$ ]] && return 0 || return 1
    fi
}

# Stop running containers
stop_containers() {
    log_info "Stopping Docker containers..."
    
    if docker-compose ps -q | grep -q .; then
        docker-compose down
        log_success "Containers stopped"
    else
        log_info "No running containers found"
    fi
}

# Remove volumes
remove_volumes() {
    if [ "$CLEAN_VOLUMES" = true ]; then
        log_warning "This will remove all Docker volumes and DELETE ALL DATA!"
        
        if confirm_action "Are you sure you want to remove all volumes?"; then
            log_info "Removing Docker volumes..."
            docker-compose down -v
            
            # Remove any orphaned volumes
            local volumes=$(docker volume ls -q --filter name=cryptopulse)
            if [ -n "$volumes" ]; then
                echo "$volumes" | xargs docker volume rm
                log_success "Volumes removed"
            else
                log_info "No volumes to remove"
            fi
        else
            log_info "Volume removal cancelled"
        fi
    fi
}

# Remove images
remove_images() {
    if [ "$CLEAN_IMAGES" = true ]; then
        log_info "Removing Docker images..."
        
        # Remove application images
        local app_images=$(docker images --filter reference="cryptopulse*" -q)
        if [ -n "$app_images" ]; then
            echo "$app_images" | xargs docker rmi -f
            log_success "Application images removed"
        fi
        
        # Remove unused images
        docker image prune -f
        log_success "Unused images removed"
    fi
}

# Clean build artifacts
clean_build_artifacts() {
    log_info "Cleaning build artifacts..."
    
    # Remove Go build cache
    go clean -cache -modcache -testcache 2>/dev/null || true
    
    # Remove local build artifacts
    rm -rf bin/
    rm -rf tmp/
    rm -f coverage.out coverage.html
    rm -f build-errors.log
    
    log_success "Build artifacts cleaned"
}

# Clean temporary files
clean_temp_files() {
    log_info "Cleaning temporary files..."
    
    # Remove editor temporary files
    find . -name "*.swp" -delete 2>/dev/null || true
    find . -name "*.swo" -delete 2>/dev/null || true
    find . -name "*~" -delete 2>/dev/null || true
    find . -name ".DS_Store" -delete 2>/dev/null || true
    
    log_success "Temporary files cleaned"
}

# System cleanup
system_cleanup() {
    if [ "$CLEAN_ALL" = true ]; then
        log_info "Performing system cleanup..."
        
        # Clean Docker system
        docker system prune -f
        
        # Clean unused networks
        docker network prune -f
        
        log_success "System cleanup completed"
    fi
}

# Show current status
show_status() {
    log_info "Current Docker status:"
    echo ""
    
    echo "Running containers:"
    docker-compose ps 2>/dev/null || echo "  None"
    echo ""
    
    echo "Docker volumes:"
    docker volume ls --filter name=cryptopulse 2>/dev/null || echo "  None"
    echo ""
    
    echo "Docker images:"
    docker images --filter reference="cryptopulse*" 2>/dev/null || echo "  None"
    echo ""
}

# Print summary
print_summary() {
    log_success "Teardown completed!"
    echo ""
    
    if [ "$CLEAN_VOLUMES" = true ]; then
        log_warning "All data has been removed. You will need to run setup again to restore the database."
    fi
    
    if [ "$CLEAN_IMAGES" = true ]; then
        log_info "Docker images have been removed. Next startup may take longer to rebuild images."
    fi
    
    echo ""
    log_info "To restart the development environment:"
    echo "  ./scripts/setup.sh"
    echo ""
    log_info "To start services without full setup:"
    echo "  make dev-up"
}

# Main execution
main() {
    log_info "Starting CryptoPulse development environment teardown..."
    echo ""
    
    # Show what will be done
    log_info "Teardown plan:"
    echo "  - Stop Docker containers: YES"
    echo "  - Remove volumes (data): $([ "$CLEAN_VOLUMES" = true ] && echo "YES" || echo "NO")"
    echo "  - Remove images: $([ "$CLEAN_IMAGES" = true ] && echo "YES" || echo "NO")"
    echo "  - System cleanup: $([ "$CLEAN_ALL" = true ] && echo "YES" || echo "NO")"
    echo ""
    
    if [ "$CLEAN_VOLUMES" = true ] || [ "$CLEAN_IMAGES" = true ]; then
        if ! confirm_action "Continue with teardown?"; then
            log_info "Teardown cancelled"
            exit 0
        fi
    fi
    
    stop_containers
    remove_volumes
    remove_images
    clean_build_artifacts
    clean_temp_files
    system_cleanup
    
    echo ""
    show_status
    print_summary
}

# Run main function
main "$@"
