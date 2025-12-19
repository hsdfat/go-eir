#!/bin/bash

################################################################################
# EIR Integration Test Runner Script
# Purpose: Run integration tests against containerized services
# Usage: ./run-tests.sh [test-pattern]
################################################################################

set -e  # Exit on any error

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
INTEGRATION_DIR="$PROJECT_ROOT/test/integration"

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

echo "================================"
echo "EIR Integration Test Runner"
echo "================================"
echo ""

# Check if containers are running
check_containers() {
    print_info "Checking if containers are running..."

    cd "$INTEGRATION_DIR"

    local required_containers=("eir-core" "diameter-gateway" "simulated-dra")
    local all_running=true

    for container in "${required_containers[@]}"; do
        if ! docker ps --format '{{.Names}}' | grep -q "^${container}$"; then
            print_error "Container $container is not running"
            all_running=false
        fi
    done

    if [ "$all_running" = false ]; then
        print_error "Not all required containers are running"
        print_info "Please run: ./setup.sh"
        exit 1
    fi

    print_info "✓ All required containers are running"
}

# Run tests
run_tests() {
    local test_pattern="${1:-TestContainerizedS13Integration}"

    print_info "Running integration tests..."
    print_info "Test pattern: $test_pattern"
    echo ""

    cd "$INTEGRATION_DIR"

    # Set environment variables for tests
    export DRA_ADDR="localhost:3869"
    export EIR_HTTP_ADDR="localhost:8080"

    # Run tests with verbose output
    if go test -v -timeout 5m -run "$test_pattern" ./containerized_integration_test.go; then
        echo ""
        print_info "================================"
        print_info "✓ All tests passed!"
        print_info "================================"
        echo ""
        exit 0
    else
        echo ""
        print_error "================================"
        print_error "✗ Tests failed!"
        print_error "================================"
        echo ""
        print_info "Check container logs for details:"
        echo "  docker-compose logs eir-core"
        echo "  docker-compose logs diameter-gateway"
        echo "  docker-compose logs simulated-dra"
        exit 1
    fi
}

# Display container logs
show_logs() {
    print_info "Displaying recent container logs..."
    echo ""

    cd "$INTEGRATION_DIR"

    echo "--- EIR Core (last 20 lines) ---"
    docker-compose logs --tail=20 eir-core
    echo ""

    echo "--- Diameter Gateway (last 20 lines) ---"
    docker-compose logs --tail=20 diameter-gateway
    echo ""

    echo "--- Simulated DRA (last 20 lines) ---"
    docker-compose logs --tail=20 simulated-dra
    echo ""
}

# Main execution
main() {
    local test_pattern="${1:-TestContainerizedS13Integration}"

    check_containers
    run_tests "$test_pattern"
}

# Handle script arguments
if [ "$1" == "--logs" ]; then
    show_logs
    exit 0
elif [ "$1" == "--help" ] || [ "$1" == "-h" ]; then
    echo "Usage: ./run-tests.sh [test-pattern|--logs]"
    echo ""
    echo "Arguments:"
    echo "  test-pattern    Regex pattern for tests to run (default: TestContainerizedS13Integration)"
    echo "  --logs          Display recent container logs"
    echo "  --help, -h      Show this help message"
    echo ""
    echo "Examples:"
    echo "  ./run-tests.sh                                    # Run all integration tests"
    echo "  ./run-tests.sh TestWhitelistedIMEI                # Run specific test"
    echo "  ./run-tests.sh 'Test.*IMEI'                       # Run tests matching pattern"
    echo "  ./run-tests.sh --logs                             # Show container logs"
    echo ""
    exit 0
fi

# Run main function
main "$@"
