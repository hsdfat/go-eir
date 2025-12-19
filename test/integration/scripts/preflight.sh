#!/bin/bash

cd "$(dirname "$0")/.."

echo "════════════════════════════════════════════"
echo "  EIR Integration Test - Pre-Flight Check"
echo "════════════════════════════════════════════"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0

check() {
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $1"
        ((PASS++))
    else
        echo -e "${RED}✗${NC} $1"
        ((FAIL++))
    fi
}

# File checks
echo "📁 File Structure"
test -f docker-compose.yml
check "docker-compose.yml exists"

test -f containerized_integration_test.go
check "Integration test file exists"

test -f scripts/setup.sh -a -x scripts/setup.sh
check "setup.sh exists and is executable"

test -f scripts/run-tests.sh -a -x scripts/run-tests.sh
check "run-tests.sh exists and is executable"

test -f scripts/teardown.sh -a -x scripts/teardown.sh
check "teardown.sh exists and is executable"

test -f containers/eir-core/Dockerfile
check "EIR Core Dockerfile exists"

test -f containers/diameter-gateway/Dockerfile
check "Diameter Gateway Dockerfile exists"

test -f containers/simulated-dra/Dockerfile
check "Simulated DRA Dockerfile exists"

echo ""
echo "🔧 System Requirements"

command -v docker &> /dev/null
check "Docker is installed"

docker info &> /dev/null
check "Docker daemon is running"

command -v docker-compose &> /dev/null || docker compose version &> /dev/null
check "Docker Compose is installed"

command -v go &> /dev/null
check "Go is installed"

echo ""
echo "🌐 Port Availability"

! lsof -i :3868 &> /dev/null && ! netstat -an 2>/dev/null | grep -q ":3868.*LISTEN"
check "Port 3868 is available"

! lsof -i :3869 &> /dev/null && ! netstat -an 2>/dev/null | grep -q ":3869.*LISTEN"
check "Port 3869 is available"

! lsof -i :8080 &> /dev/null && ! netstat -an 2>/dev/null | grep -q ":8080.*LISTEN"
check "Port 8080 is available"

! lsof -i :9090 &> /dev/null && ! netstat -an 2>/dev/null | grep -q ":9090.*LISTEN"
check "Port 9090 is available"

echo ""
echo "📋 Configuration Validation"

docker-compose config --quiet &> /dev/null
check "docker-compose.yml is valid"

echo ""
echo "════════════════════════════════════════════"
echo "  Results: ${PASS} passed, ${FAIL} failed"
echo "════════════════════════════════════════════"

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}✓ All checks passed! Ready to run tests.${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. ./scripts/setup.sh"
    echo "  2. ./scripts/run-tests.sh"
    echo "  3. ./scripts/teardown.sh"
    exit 0
else
    echo -e "${RED}✗ Some checks failed. Please fix the issues above.${NC}"
    exit 1
fi
