#!/bin/bash

################################################################################
# EIR Integration Test Setup Script
# Purpose: Build and start all containerized services for integration testing
# Usage: ./setup.sh
################################################################################

set -e  # Exit on any error

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
INTEGRATION_DIR="$PROJECT_ROOT/test/integration"

echo "================================"
echo "EIR Integration Test Setup"
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

# Check Docker availability
check_docker() {
    print_info "Checking Docker installation..."

    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed. Please install Docker first."
        exit 1
    fi

    if ! docker info &> /dev/null; then
        print_error "Docker daemon is not running. Please start Docker."
        exit 1
    fi

    print_info "✓ Docker is available"
}

# Check Docker Compose availability
check_docker_compose() {
    print_info "Checking Docker Compose installation..."

    if ! command -v docker-compose &> /dev/null && ! docker compose version &> /dev/null; then
        print_error "Docker Compose is not installed. Please install Docker Compose."
        exit 1
    fi

    print_info "✓ Docker Compose is available"
}

# Clean up existing containers
cleanup_existing() {
    print_info "Cleaning up existing containers..."

    cd "$INTEGRATION_DIR"

    # Stop and remove containers if they exist
    if docker-compose ps -q 2>/dev/null | grep -q .; then
        print_warning "Stopping existing containers..."
        docker-compose down --remove-orphans
    fi

    # Remove dangling images
    if docker images -f "dangling=true" -q | grep -q .; then
        print_info "Removing dangling images..."
        docker images -f "dangling=true" -q | xargs docker rmi -f 2>/dev/null || true
    fi

    print_info "✓ Cleanup completed"
}

# Build container images
build_images() {
    print_info "Building container images..."

    cd "$INTEGRATION_DIR"

    # Build with verbose output
    docker-compose build --progress=plain

    if [ $? -eq 0 ]; then
        print_info "✓ All images built successfully"
    else
        print_error "Failed to build container images"
        exit 1
    fi
}

# Start containers
start_containers() {
    print_info "Starting containers..."

    cd "$INTEGRATION_DIR"

    # Start containers in detached mode
    docker-compose up -d

    if [ $? -eq 0 ]; then
        print_info "✓ Containers started"
    else
        print_error "Failed to start containers"
        exit 1
    fi
}

# Wait for containers to be healthy
wait_for_health() {
    print_info "Waiting for containers to be healthy..."

    local max_attempts=30
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        local healthy=true

        # Check EIR Core health
        if ! docker exec eir-core curl -sf http://localhost:8080/health > /dev/null 2>&1; then
            healthy=false
        fi

        # Check Diameter Gateway health
        if ! docker exec diameter-gateway sh -c "netstat -an | grep 3868" > /dev/null 2>&1; then
            healthy=false
        fi

        # Check Simulated DRA health
        if ! docker exec simulated-dra sh -c "netstat -an | grep 3869" > /dev/null 2>&1; then
            healthy=false
        fi

        if [ "$healthy" = true ]; then
            print_info "✓ All containers are healthy"
            return 0
        fi

        echo -n "."
        sleep 2
        ((attempt++))
    done

    echo ""
    print_error "Containers did not become healthy within timeout"
    print_info "Showing container status:"
    docker-compose ps
    return 1
}

# Display container status
show_status() {
    print_info "Container status:"
    echo ""

    cd "$INTEGRATION_DIR"
    docker-compose ps

    echo ""
    print_info "Container logs (last 10 lines):"
    echo ""

    echo "--- EIR Core ---"
    docker-compose logs --tail=10 eir-core

    echo ""
    echo "--- Diameter Gateway ---"
    docker-compose logs --tail=10 diameter-gateway

    echo ""
    echo "--- Simulated DRA ---"
    docker-compose logs --tail=10 simulated-dra
}

# Display connection information
show_connection_info() {
    echo ""
    print_info "================================"
    print_info "Service Endpoints"
    print_info "================================"
    echo ""
    echo "  EIR Core HTTP API:      http://localhost:8080"
    echo "  EIR Core Prometheus:    http://localhost:9090/metrics"
    echo "  Diameter Gateway:       localhost:3868"
    echo "  Simulated DRA:          localhost:3869"
    echo ""
    print_info "Test clients should connect to: localhost:3869 (Simulated DRA)"
    echo ""
}

# Main execution
main() {
    print_info "Starting EIR integration test environment setup..."
    echo ""

    check_docker
    check_docker_compose
    cleanup_existing
    build_images
    start_containers

    if wait_for_health; then
        echo ""
        print_info "================================"
        print_info "✓ Setup completed successfully!"
        print_info "================================"
        echo ""

        show_status
        show_connection_info

        print_info "You can now run integration tests:"
        echo "  cd $INTEGRATION_DIR"
        echo "  go test -v ./containerized_integration_test.go"
        echo ""

        exit 0
    else
        print_error "Setup failed. Please check container logs:"
        echo "  docker-compose logs"
        exit 1
    fi
}

# Run main function
main
