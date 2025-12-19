# Pre-Flight Verification Checklist

## ✅ File Structure Verification

Run this to verify all files are in place:

```bash
cd /Users/loannt70/Documents/phatlc/telco/eir/test/integration

echo "=== Checking Directory Structure ==="
test -f docker-compose.yml && echo "✓ docker-compose.yml" || echo "✗ docker-compose.yml MISSING"
test -f containerized_integration_test.go && echo "✓ containerized_integration_test.go" || echo "✗ Test file MISSING"
test -f scripts/setup.sh && echo "✓ scripts/setup.sh" || echo "✗ setup.sh MISSING"
test -f scripts/run-tests.sh && echo "✓ scripts/run-tests.sh" || echo "✗ run-tests.sh MISSING"
test -f scripts/teardown.sh && echo "✓ scripts/teardown.sh" || echo "✗ teardown.sh MISSING"

echo ""
echo "=== Checking Container Definitions ==="
test -f containers/eir-core/Dockerfile && echo "✓ EIR Core Dockerfile" || echo "✗ EIR Core Dockerfile MISSING"
test -f containers/eir-core/main.go && echo "✓ EIR Core main.go" || echo "✗ EIR Core main.go MISSING"
test -f containers/diameter-gateway/Dockerfile && echo "✓ Gateway Dockerfile" || echo "✗ Gateway Dockerfile MISSING"
test -f containers/diameter-gateway/main.go && echo "✓ Gateway main.go" || echo "✗ Gateway main.go MISSING"
test -f containers/simulated-dra/Dockerfile && echo "✓ DRA Dockerfile" || echo "✗ DRA Dockerfile MISSING"
test -f containers/simulated-dra/main.go && echo "✓ DRA main.go" || echo "✗ DRA main.go MISSING"

echo ""
echo "=== Checking Script Permissions ==="
test -x scripts/setup.sh && echo "✓ setup.sh is executable" || echo "✗ setup.sh NOT executable (run: chmod +x scripts/setup.sh)"
test -x scripts/run-tests.sh && echo "✓ run-tests.sh is executable" || echo "✗ run-tests.sh NOT executable"
test -x scripts/teardown.sh && echo "✓ teardown.sh is executable" || echo "✗ teardown.sh NOT executable"
```

## ✅ System Requirements

```bash
echo "=== System Requirements Check ==="

# Docker
if command -v docker &> /dev/null; then
    echo "✓ Docker installed: $(docker --version)"
else
    echo "✗ Docker NOT installed - Install from https://www.docker.com/get-started"
fi

# Docker Compose
if command -v docker-compose &> /dev/null || docker compose version &> /dev/null; then
    echo "✓ Docker Compose installed"
else
    echo "✗ Docker Compose NOT installed"
fi

# Go
if command -v go &> /dev/null; then
    echo "✓ Go installed: $(go version)"
else
    echo "✗ Go NOT installed - Required for running tests"
fi

# Docker daemon
if docker info &> /dev/null; then
    echo "✓ Docker daemon is running"
else
    echo "✗ Docker daemon NOT running - Start Docker Desktop"
fi
```

## ✅ Docker Compose Validation

```bash
cd /Users/loannt70/Documents/phatlc/telco/eir/test/integration

echo "=== Validating docker-compose.yml ==="
if docker-compose config --quiet 2>&1; then
    echo "✓ docker-compose.yml is valid"
else
    echo "✗ docker-compose.yml has syntax errors"
    docker-compose config
fi
```

## ✅ Port Availability

```bash
echo "=== Checking Port Availability ==="

check_port() {
    local port=$1
    if lsof -i :$port > /dev/null 2>&1 || netstat -an | grep -q ":$port.*LISTEN"; then
        echo "✗ Port $port is IN USE - Need to stop conflicting service"
        lsof -i :$port 2>/dev/null || netstat -an | grep ":$port.*LISTEN"
    else
        echo "✓ Port $port is available"
    fi
}

check_port 3868
check_port 3869
check_port 8080
check_port 9090
```

## ✅ Go Module Verification

```bash
cd /Users/loannt70/Documents/phatlc/telco/eir

echo "=== Verifying Go Modules ==="
if go mod verify; then
    echo "✓ All Go modules verified"
else
    echo "✗ Go module verification failed"
    echo "  Try: go mod tidy"
fi
```

## ✅ Complete Pre-Flight Script

Save this as `preflight.sh` and run it:

```bash
#!/bin/bash

cd /Users/loannt70/Documents/phatlc/telco/eir/test/integration

echo "════════════════════════════════════════════"
echo "  EIR Integration Test - Pre-Flight Check"
echo "════════════════════════════════════════════"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
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
```

## Make it executable

```bash
chmod +x preflight.sh
./preflight.sh
```

## Expected Output

```
════════════════════════════════════════════
  EIR Integration Test - Pre-Flight Check
════════════════════════════════════════════

📁 File Structure
✓ docker-compose.yml exists
✓ Integration test file exists
✓ setup.sh exists and is executable
✓ run-tests.sh exists and is executable
✓ teardown.sh exists and is executable
✓ EIR Core Dockerfile exists
✓ Diameter Gateway Dockerfile exists
✓ Simulated DRA Dockerfile exists

🔧 System Requirements
✓ Docker is installed
✓ Docker daemon is running
✓ Docker Compose is installed
✓ Go is installed

🌐 Port Availability
✓ Port 3868 is available
✓ Port 3869 is available
✓ Port 8080 is available
✓ Port 9090 is available

📋 Configuration Validation
✓ docker-compose.yml is valid

════════════════════════════════════════════
  Results: 16 passed, 0 failed
════════════════════════════════════════════
✓ All checks passed! Ready to run tests.

Next steps:
  1. ./scripts/setup.sh
  2. ./scripts/run-tests.sh
  3. ./scripts/teardown.sh
```

---

## If Any Check Fails

### Docker not installed
```bash
# macOS
brew install --cask docker

# Or download from https://www.docker.com/get-started
```

### Docker daemon not running
```bash
# Start Docker Desktop application
open -a Docker
```

### Ports in use
```bash
# Find what's using the port
lsof -i :3869

# Kill the process
kill -9 <PID>

# Or change ports in docker-compose.yml
```

### Go modules issues
```bash
cd /Users/loannt70/Documents/phatlc/telco/eir
go mod tidy
go mod verify
```

---

## Manual Validation Steps

If you want to validate without running the full test:

### 1. Test Docker Compose Syntax
```bash
cd /Users/loannt70/Documents/phatlc/telco/eir/test/integration
docker-compose config
```

### 2. Build Images (without starting)
```bash
docker-compose build
```

### 3. Check Image Sizes
```bash
docker images | grep -E '(eir-core|diameter-gateway|simulated-dra)'
```

### 4. Start One Container
```bash
docker-compose up eir-core
# Ctrl+C to stop
```

### 5. Validate Go Test Syntax
```bash
cd /Users/loannt70/Documents/phatlc/telco/eir/test/integration
go test -c ./containerized_integration_test.go
# Should create a test binary without errors
```

---

## Full Workflow Test (Without Running Containers)

```bash
# 1. Validate all files exist
./preflight.sh

# 2. Validate docker-compose
docker-compose config --quiet && echo "✓ Valid"

# 3. Compile test (don't run)
go test -c ./containerized_integration_test.go && echo "✓ Test compiles"

# 4. Check scripts are executable
test -x scripts/*.sh && echo "✓ Scripts executable"

echo "✅ All validations passed!"
```

---

## When Everything Checks Out

Run the actual tests:

```bash
./scripts/setup.sh && ./scripts/run-tests.sh && ./scripts/teardown.sh
```

Expected duration: **~30 seconds total**
