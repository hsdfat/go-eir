#!/bin/bash

################################################################################
# EIR Integration Test Teardown Script
# Purpose: Stop and clean up all containerized services
# Usage: ./teardown.sh
################################################################################

set -e  # Exit on any error

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
INTEGRATION_DIR="$PROJECT_ROOT/test/integration"

echo "================================"
echo "EIR Integration Test Teardown"
echo "================================"
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Stop containers
stop_containers() {
    print_info "Stopping containers..."

    cd "$INTEGRATION_DIR"

    if docker-compose ps -q 2>/dev/null | grep -q .; then
        docker-compose stop
        print_info "✓ Containers stopped"
    else
        print_warning "No running containers found"
    fi
}

# Remove containers
remove_containers() {
    print_info "Removing containers..."

    cd "$INTEGRATION_DIR"

    docker-compose down --remove-orphans

    print_info "✓ Containers removed"
}

# Remove volumes (optional)
remove_volumes() {
    if [ "$1" == "--volumes" ] || [ "$1" == "-v" ]; then
        print_info "Removing volumes..."

        cd "$INTEGRATION_DIR"
        docker-compose down -v

        print_info "✓ Volumes removed"
    fi
}

# Remove images (optional)
remove_images() {
    if [ "$1" == "--images" ] || [ "$1" == "-i" ]; then
        print_info "Removing container images..."

        # Remove images built by docker-compose
        docker images | grep -E 'eir-core|diameter-gateway|simulated-dra' | awk '{print $3}' | xargs -r docker rmi -f 2>/dev/null || true

        # Remove dangling images
        docker images -f "dangling=true" -q | xargs -r docker rmi -f 2>/dev/null || true

        print_info "✓ Images removed"
    fi
}

# Remove network
remove_network() {
    print_info "Removing network..."

    if docker network ls | grep -q "eir-integration-network"; then
        docker network rm eir-integration-network 2>/dev/null || true
    fi

    print_info "✓ Network cleaned up"
}

# Display usage
show_usage() {
    echo "Usage: ./teardown.sh [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -v, --volumes    Also remove volumes"
    echo "  -i, --images     Also remove container images"
    echo "  --all            Remove everything (containers, volumes, images, networks)"
    echo "  -h, --help       Show this help message"
    echo ""
}

# Main execution
main() {
    local remove_vols=false
    local remove_imgs=false

    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -v|--volumes)
                remove_vols=true
                shift
                ;;
            -i|--images)
                remove_imgs=true
                shift
                ;;
            --all)
                remove_vols=true
                remove_imgs=true
                shift
                ;;
            -h|--help)
                show_usage
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                show_usage
                exit 1
                ;;
        esac
    done

    print_info "Starting teardown..."
    echo ""

    stop_containers
    remove_containers

    if [ "$remove_vols" = true ]; then
        remove_volumes "--volumes"
    fi

    if [ "$remove_imgs" = true ]; then
        remove_images "--images"
    fi

    remove_network

    echo ""
    print_info "================================"
    print_info "✓ Teardown completed successfully!"
    print_info "================================"
    echo ""
}

# Run main function
main "$@"
