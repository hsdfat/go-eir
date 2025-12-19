#!/bin/bash

# EIR Performance Test Runner
# Executes comprehensive performance tests and generates reports

set -e

cd "$(dirname "$0")/.."

echo "═══════════════════════════════════════════════"
echo "  EIR Performance Test Suite"
echo "═══════════════════════════════════════════════"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
REPORT_DIR="./test-reports/performance"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
JSON_REPORT="${REPORT_DIR}/performance_${TIMESTAMP}.json"
MD_REPORT="${REPORT_DIR}/performance_${TIMESTAMP}.md"
LATEST_LINK="${REPORT_DIR}/latest.md"

# Create report directory
mkdir -p "$REPORT_DIR"

echo -e "${BLUE}📊 Test Configuration${NC}"
echo "  DRA Address: ${DRA_ADDR:-localhost:3869}"
echo "  EIR Address: ${EIR_ADDR:-localhost:8080}"
echo "  Gateway Address: ${GATEWAY_ADDR:-localhost:3868}"
echo "  Report Directory: $REPORT_DIR"
echo ""

# Check if containers are running
echo -e "${BLUE}🔍 Checking Container Health${NC}"
if ! docker-compose ps | grep -q "Up"; then
    echo -e "${RED}✗ Containers are not running${NC}"
    echo "  Please start containers first:"
    echo "    docker-compose up -d"
    exit 1
fi

HEALTHY=$(docker-compose ps | grep "(healthy)" | wc -l)
echo -e "${GREEN}✓ $HEALTHY containers are healthy${NC}"
echo ""

# Wait for services to be ready
echo -e "${BLUE}⏳ Waiting for services to stabilize${NC}"
sleep 3
echo ""

# Run performance tests
echo -e "${BLUE}🚀 Running Performance Tests${NC}"
echo "  This may take several minutes..."
echo ""

# Function to run a test category
run_test_category() {
    local category=$1
    local test_name=$2

    echo -e "${YELLOW}━━━ $category ━━━${NC}"

    if go test -v -run "$test_name" -timeout 30m 2>&1 | tee -a "${REPORT_DIR}/test_output_${TIMESTAMP}.log"; then
        echo -e "${GREEN}✓ $category completed${NC}"
        return 0
    else
        echo -e "${RED}✗ $category failed${NC}"
        return 1
    fi
}

# Track test results
TOTAL_CATEGORIES=0
PASSED_CATEGORIES=0

# 1. Throughput Tests
TOTAL_CATEGORIES=$((TOTAL_CATEGORIES + 1))
if run_test_category "Throughput Tests" "TestPerformance_Throughput"; then
    PASSED_CATEGORIES=$((PASSED_CATEGORIES + 1))
fi
echo ""

# 2. Latency Tests
TOTAL_CATEGORIES=$((TOTAL_CATEGORIES + 1))
if run_test_category "Latency Tests" "TestPerformance_Latency"; then
    PASSED_CATEGORIES=$((PASSED_CATEGORIES + 1))
fi
echo ""

# 3. Sustained Load Tests
TOTAL_CATEGORIES=$((TOTAL_CATEGORIES + 1))
if run_test_category "Sustained Load Tests" "TestPerformance_SustainedLoad"; then
    PASSED_CATEGORIES=$((PASSED_CATEGORIES + 1))
fi
echo ""

# 4. Stress Tests
TOTAL_CATEGORIES=$((TOTAL_CATEGORIES + 1))
if run_test_category "Stress Tests" "TestPerformance_StressTest"; then
    PASSED_CATEGORIES=$((PASSED_CATEGORIES + 1))
fi
echo ""

# 5. Connection Pooling Tests
TOTAL_CATEGORIES=$((TOTAL_CATEGORIES + 1))
if run_test_category "Connection Pooling Tests" "TestPerformance_ConnectionPooling"; then
    PASSED_CATEGORIES=$((PASSED_CATEGORIES + 1))
fi
echo ""

# 6. Message Size Tests
TOTAL_CATEGORIES=$((TOTAL_CATEGORIES + 1))
if run_test_category "Message Size Tests" "TestPerformance_MessageSize"; then
    PASSED_CATEGORIES=$((PASSED_CATEGORIES + 1))
fi
echo ""

# Summary
echo "═══════════════════════════════════════════════"
echo -e "${BLUE}  Test Execution Summary${NC}"
echo "═══════════════════════════════════════════════"
echo "  Total Categories: $TOTAL_CATEGORIES"
echo -e "  ${GREEN}Passed: $PASSED_CATEGORIES${NC}"
echo -e "  ${RED}Failed: $((TOTAL_CATEGORIES - PASSED_CATEGORIES))${NC}"
echo ""

# Check if we should generate report
if [ $PASSED_CATEGORIES -eq 0 ]; then
    echo -e "${RED}✗ All test categories failed. Skipping report generation.${NC}"
    exit 1
fi

# Generate performance report (placeholder - would integrate with Go test results)
echo -e "${BLUE}📝 Generating Performance Report${NC}"
echo ""

# Create a summary report
cat > "$MD_REPORT" << EOF
# EIR S13 Interface Performance Test Report

**Execution Date**: $(date '+%Y-%m-%d %H:%M:%S %Z')

## Executive Summary

| Metric | Value |
|--------|-------|
| Test Categories | $TOTAL_CATEGORIES |
| Passed | $PASSED_CATEGORIES |
| Failed | $((TOTAL_CATEGORIES - PASSED_CATEGORIES)) |
| Success Rate | $(echo "scale=1; $PASSED_CATEGORIES * 100 / $TOTAL_CATEGORIES" | bc)% |

## Test Environment

| Component | Address |
|-----------|---------|
| DRA | ${DRA_ADDR:-localhost:3869} |
| EIR Core | ${EIR_ADDR:-localhost:8080} |
| Gateway | ${GATEWAY_ADDR:-localhost:3868} |

## Test Categories Executed

### 1. Throughput Tests
Tests maximum throughput capacity with varying concurrency levels (1, 5, 10, 25, 50, 100 clients).

**Objectives:**
- Measure maximum requests per second (RPS)
- Identify optimal concurrency level
- Detect throughput saturation point

### 2. Latency Tests
Tests latency characteristics under different load patterns including burst traffic.

**Objectives:**
- Measure P50, P95, P99 latency under various loads
- Validate SLA compliance (<100ms P95)
- Identify latency variance patterns

### 3. Sustained Load Tests
Tests system behavior under continuous load for extended duration (60 seconds).

**Objectives:**
- Verify sustained throughput stability
- Detect memory leaks or resource exhaustion
- Measure long-term latency consistency

### 4. Stress Tests
Gradually increases load until performance degradation is observed (10 to 200 concurrent clients).

**Objectives:**
- Identify breaking point
- Measure degradation patterns
- Establish safe operating limits

### 5. Connection Pooling Tests
Compares performance with and without connection reuse.

**Objectives:**
- Quantify connection pooling benefits
- Validate connection handling efficiency
- Measure overhead of connection establishment

### 6. Message Size Tests
Tests performance impact of varying message sizes.

**Objectives:**
- Measure throughput vs message size relationship
- Identify message size limitations
- Validate protocol handling efficiency

## Detailed Results

See full test output in: \`test_output_${TIMESTAMP}.log\`

## Key Performance Indicators

Based on the test execution:

EOF

# Add container resource usage
echo "### Resource Usage" >> "$MD_REPORT"
echo "" >> "$MD_REPORT"
echo "\`\`\`" >> "$MD_REPORT"
docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" >> "$MD_REPORT" 2>&1 || true
echo "\`\`\`" >> "$MD_REPORT"
echo "" >> "$MD_REPORT"

# Add recommendations
cat >> "$MD_REPORT" << 'EOF'
## Recommendations

### Performance Optimization
- Monitor P95/P99 latencies in production
- Implement circuit breakers for resilience
- Consider horizontal scaling for higher throughput demands

### Monitoring & Alerting
- Establish baseline metrics from these tests
- Set up alerts for P95 latency >100ms
- Monitor success rate and alert on <99%

### Capacity Planning
- Use stress test results to determine safe operating capacity
- Plan for 2x peak load capacity as buffer
- Regular performance regression testing

---

*Report generated at $(date '+%Y-%m-%d %H:%M:%S %Z')*
EOF

# Create symlink to latest report
ln -sf "$(basename "$MD_REPORT")" "$LATEST_LINK"

echo -e "${GREEN}✓ Report generated: $MD_REPORT${NC}"
echo -e "${GREEN}✓ Latest report: $LATEST_LINK${NC}"
echo ""

# Display quick summary
echo "═══════════════════════════════════════════════"
echo -e "${BLUE}  Quick Summary${NC}"
echo "═══════════════════════════════════════════════"
if [ -f "$LATEST_LINK" ]; then
    head -30 "$LATEST_LINK"
fi
echo ""

# Final status
if [ $PASSED_CATEGORIES -eq $TOTAL_CATEGORIES ]; then
    echo -e "${GREEN}✓ All performance tests completed successfully${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Review detailed report: $MD_REPORT"
    echo "  2. Compare with previous baseline"
    echo "  3. Update monitoring thresholds based on findings"
    exit 0
else
    echo -e "${YELLOW}⚠ Some test categories failed${NC}"
    echo ""
    echo "Next steps:"
    echo "  1. Review failures in: ${REPORT_DIR}/test_output_${TIMESTAMP}.log"
    echo "  2. Check container logs for errors"
    echo "  3. Verify system resources and configuration"
    exit 1
fi
