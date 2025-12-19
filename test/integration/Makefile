.PHONY: test test-full test-service test-client test-verbose test-race test-coverage clean help
.PHONY: perf perf-up perf-down perf-logs perf-report perf-clean perf-setup perf-teardown perf-quick perf-summary

# Default target
help:
	@echo "EIR Integration Test Makefile"
	@echo ""
	@echo "Available targets:"
	@echo "  test          - Run all integration tests"
	@echo "  test-full     - Run full stack integration test (recommended)"
	@echo "  test-service  - Run EIR service integration tests"
	@echo "  test-client   - Run Diameter client flow tests"
	@echo "  test-verbose  - Run all tests with verbose output"
	@echo "  test-race     - Run tests with race detector"
	@echo "  test-coverage - Run tests with coverage report"
	@echo ""
	@echo "Performance Test Targets (Docker Compose):"
	@echo "  perf          - Run performance tests and generate report (recommended)"
	@echo "  perf-up       - Start all services and run performance tests"
	@echo "  perf-down     - Stop all services"
	@echo "  perf-logs     - Show performance test logs"
	@echo "  perf-report   - Extract and display test results"
	@echo "  perf-summary  - Show summary of latest test results"
	@echo "  perf-quick    - Run quick test (single concurrency level)"
	@echo "  perf-clean    - Clean performance test artifacts"
	@echo "  perf-setup    - Build and start services (without tests)"
	@echo "  perf-teardown - Stop and remove all containers"
	@echo ""
	@echo "  clean         - Clean test cache and coverage files"
	@echo ""

# Run all integration tests
test:
	@echo "Running all integration tests..."
	go test -v ./...

# Run full stack integration test (recommended)
test-full:
	@echo "Running full stack integration test..."
	@echo "Testing: Client <-> DRA <-> EIR Server <-> Service <-> Mock DB"
	go test -v -run TestFullStackIntegration

# Run EIR service integration tests
test-service:
	@echo "Running EIR service integration tests..."
	go test -v -run TestEIRIntegration_WithDRASimulator

# Run Diameter client flow tests
test-client:
	@echo "Running Diameter client flow tests..."
	go test -v -run TestDiameterClientToEIR

# Run all tests with verbose output
test-verbose:
	@echo "Running all tests (verbose mode)..."
	go test -v -count=1 ./...

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	go test -v -race ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run specific test scenario
test-scenario:
	@echo "Available scenarios:"
	@echo "  - CompleteFlow_ClientToDRA_ToEIR"
	@echo "  - MultipleClients_ConcurrentChecks"
	@echo "  - BlacklistedDevice_Rejected"
	@echo "  - UnknownDevice_DefaultPolicy"
	@echo "  - AuditTrail_FullStack"
	@echo ""
	@echo "Usage: make test-scenario SCENARIO=CompleteFlow_ClientToDRA_ToEIR"
	@if [ -n "$(SCENARIO)" ]; then \
		go test -v -run TestFullStackIntegration/$(SCENARIO); \
	fi

# Clean test artifacts
clean:
	@echo "Cleaning test artifacts..."
	go clean -testcache
	rm -f coverage.out coverage.html
	@echo "Clean complete"

# Quick smoke test
smoke:
	@echo "Running quick smoke test..."
	go test -v -run TestFullStackIntegration/CompleteFlow -timeout 30s

# Benchmark tests (if available)
bench:
	@echo "Running benchmark tests..."
	go test -bench=. -benchmem

# Run tests in specific order
test-ordered:
	@echo "Running tests in recommended order..."
	@echo ""
	@echo "1. Full Stack Integration"
	@make test-full
	@echo ""
	@echo "2. Service Integration"
	@make test-service
	@echo ""
	@echo "3. Client Flow"
	@make test-client
	@echo ""
	@echo "All tests completed!"

# Check for common issues
check:
	@echo "Checking for common issues..."
	@echo ""
	@echo "Checking if ports are available..."
	@! lsof -i :13870 > /dev/null 2>&1 && echo "✓ Port 13870 available" || echo "✗ Port 13870 in use"
	@! lsof -i :13871 > /dev/null 2>&1 && echo "✓ Port 13871 available" || echo "✗ Port 13871 in use"
	@! lsof -i :13868 > /dev/null 2>&1 && echo "✓ Port 13868 available" || echo "✗ Port 13868 in use"
	@! lsof -i :13869 > /dev/null 2>&1 && echo "✓ Port 13869 available" || echo "✗ Port 13869 in use"
	@echo ""
	@echo "Checking Go version..."
	@go version
	@echo ""
	@echo "Checking dependencies..."
	@go mod verify && echo "✓ Go modules verified" || echo "✗ Go modules verification failed"

# Watch for changes and re-run tests (requires fswatch or entr)
watch:
	@echo "Watching for changes... (requires fswatch)"
	@command -v fswatch >/dev/null 2>&1 || (echo "Error: fswatch not installed. Install with: brew install fswatch" && exit 1)
	@fswatch -o . | xargs -n1 -I{} make test-full

# Debug specific test with delve (requires dlv)
debug:
	@echo "Starting debugger... (requires delve)"
	@command -v dlv >/dev/null 2>&1 || (echo "Error: delve not installed. Install with: go install github.com/go-delve/delve/cmd/dlv@latest" && exit 1)
	dlv test -- -test.run TestFullStackIntegration

# Generate test report
report:
	@echo "Generating test report..."
	@go test -v -json ./... > test-report.json 2>&1
	@echo "Test report generated: test-report.json"

# Run tests in CI mode (no color, machine readable)
ci:
	@echo "Running tests in CI mode..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@if [ -f coverage.out ]; then \
		go tool cover -func=coverage.out; \
	fi

# =================================================================
# Performance Test Targets (Docker Compose)
# =================================================================

# Run performance tests and generate report (main target)
perf: perf-clean
	@echo "═══════════════════════════════════════════════════════════"
	@echo "  EIR Performance Test Suite"
	@echo "═══════════════════════════════════════════════════════════"
	@echo ""
	@echo "Starting services and running performance tests..."
	@echo ""
	@mkdir -p test-reports/performance
	@docker-compose up --build performance-test-runner 2>&1 | tee test-reports/performance/test_output_$$(date +%Y%m%d_%H%M%S).log || true
	@echo ""
	@echo "═══════════════════════════════════════════════════════════"
	@echo "  Extracting Test Results"
	@echo "═══════════════════════════════════════════════════════════"
	@$(MAKE) perf-report
	@echo ""
	@echo "✓ Performance tests completed. Check test-reports/performance/ for detailed results."

# Start all services and run performance tests
perf-up:
	@echo "Starting all services and running performance tests..."
	@docker-compose up --build

# Stop all services
perf-down:
	@echo "Stopping all services..."
	@docker-compose down

# Show performance test logs
perf-logs:
	@echo "Performance Test Runner Logs:"
	@echo "═══════════════════════════════════════════════════════════"
	@docker-compose logs performance-test-runner || echo "No logs available. Run 'make perf' first."

# Extract and display test results
perf-report:
	@echo "Extracting test results from container logs..."
	@mkdir -p test-reports/performance
	@TIMESTAMP=$$(date +%Y%m%d_%H%M%S); \
	REPORT_DIR="test-reports/performance"; \
	REPORT_FILE="$$REPORT_DIR/performance_$$TIMESTAMP.md"; \
	LOG_FILE="$$REPORT_DIR/test_output_$$TIMESTAMP.log"; \
	echo "# EIR Performance Test Report" > "$$REPORT_FILE"; \
	echo "" >> "$$REPORT_FILE"; \
	echo "**Generated**: $$(date '+%Y-%m-%d %H:%M:%S %Z')" >> "$$REPORT_FILE"; \
	echo "" >> "$$REPORT_FILE"; \
	echo "## Test Execution Summary" >> "$$REPORT_FILE"; \
	echo "" >> "$$REPORT_FILE"; \
	if docker-compose logs performance-test-runner 2>/dev/null | grep -qE "(PASS|FAIL|RUN)"; then \
		PASSED=$$(docker-compose logs performance-test-runner 2>/dev/null | grep -cE "--- PASS:" || echo "0"); \
		FAILED=$$(docker-compose logs performance-test-runner 2>/dev/null | grep -cE "--- FAIL:" || echo "0"); \
		SKIPPED=$$(docker-compose logs performance-test-runner 2>/dev/null | grep -cE "--- SKIP:" || echo "0"); \
		TOTAL=$$((PASSED + FAILED + SKIPPED)); \
		echo "| Status | Count |" >> "$$REPORT_FILE"; \
		echo "|--------|-------|" >> "$$REPORT_FILE"; \
		echo "| ✅ Passed | $$PASSED |" >> "$$REPORT_FILE"; \
		echo "| ❌ Failed | $$FAILED |" >> "$$REPORT_FILE"; \
		if [ $$SKIPPED -gt 0 ]; then \
			echo "| ⏭️  Skipped | $$SKIPPED |" >> "$$REPORT_FILE"; \
		fi; \
		echo "| **Total** | **$$TOTAL** |" >> "$$REPORT_FILE"; \
		echo "" >> "$$REPORT_FILE"; \
		echo "## Test Results" >> "$$REPORT_FILE"; \
		echo "" >> "$$REPORT_FILE"; \
		docker-compose logs performance-test-runner 2>/dev/null | grep -E "(RUN|PASS|FAIL|SKIP|Throughput|Latency|RPS|req/sec)" | tail -50 >> "$$REPORT_FILE" || true; \
		echo "" >> "$$REPORT_FILE"; \
		echo "## Detailed Test Output" >> "$$REPORT_FILE"; \
		echo "" >> "$$REPORT_FILE"; \
		echo "\`\`\`" >> "$$REPORT_FILE"; \
		docker-compose logs performance-test-runner 2>/dev/null | tail -200 >> "$$REPORT_FILE" || true; \
		echo "\`\`\`" >> "$$REPORT_FILE"; \
		echo "" >> "$$REPORT_FILE"; \
		echo "## Performance Metrics" >> "$$REPORT_FILE"; \
		echo "" >> "$$REPORT_FILE"; \
		docker-compose logs performance-test-runner 2>/dev/null | grep -E "(Concurrency|Total Requests|Successful|Throughput|Latency|P50|P95|P99)" | tail -30 >> "$$REPORT_FILE" || echo "No detailed metrics found." >> "$$REPORT_FILE"; \
		echo "" >> "$$REPORT_FILE"; \
		echo "## Full Logs" >> "$$REPORT_FILE"; \
		echo "" >> "$$REPORT_FILE"; \
		echo "See \`$$LOG_FILE\` for complete test output." >> "$$REPORT_FILE"; \
		ln -sf "performance_$$TIMESTAMP.md" "$$REPORT_DIR/latest.md" 2>/dev/null || true; \
		echo ""; \
		echo "═══════════════════════════════════════════════════════════"; \
		echo "  Test Results Summary"; \
		echo "═══════════════════════════════════════════════════════════"; \
		echo "  ✅ Passed: $$PASSED"; \
		echo "  ❌ Failed: $$FAILED"; \
		if [ $$SKIPPED -gt 0 ]; then \
			echo "  ⏭️  Skipped: $$SKIPPED"; \
		fi; \
		echo "  📊 Total: $$TOTAL"; \
		echo ""; \
		echo "✓ Report generated: $$REPORT_FILE"; \
		echo "✓ Latest report: $$REPORT_DIR/latest.md"; \
	else \
		echo "⚠ No test results found in container logs." >> "$$REPORT_FILE"; \
		echo "⚠ No test results found. Run 'make perf' first."; \
	fi

# Clean performance test artifacts
perf-clean:
	@echo "Cleaning performance test artifacts..."
	@rm -rf test-reports/performance/*.log test-reports/performance/*.md 2>/dev/null || true
	@echo "✓ Cleaned performance test artifacts"

# Setup services without running tests
perf-setup:
	@echo "Building and starting services (without tests)..."
	@docker-compose up --build -d eir-core diameter-gateway simulated-dra
	@echo "Waiting for services to be healthy..."
	@timeout=60; \
	while [ $$timeout -gt 0 ]; do \
		if docker-compose ps | grep -q "healthy"; then \
			echo "✓ All services are healthy"; \
			break; \
		fi; \
		sleep 2; \
		timeout=$$((timeout - 2)); \
	done; \
	if [ $$timeout -le 0 ]; then \
		echo "⚠ Timeout waiting for services to be healthy"; \
	fi

# Teardown all containers
perf-teardown:
	@echo "Stopping and removing all containers..."
	@docker-compose down -v
	@echo "✓ All containers stopped and removed"

# Quick performance test (single concurrency level)
perf-quick:
	@echo "Running quick performance test (Concurrency 1 only)..."
	@mkdir -p test-reports/performance
	@docker-compose run --rm --no-deps performance-test-runner sh -c \
		"echo 'Waiting for services...' && sleep 5 && \
		 go test -v -timeout 5m -run '^TestPerformance_Throughput/Concurrency_1$$' ." \
		2>&1 | tee test-reports/performance/quick_test_$$(date +%Y%m%d_%H%M%S).log
	@echo "✓ Quick test completed"

# Show performance test summary
perf-summary:
	@echo "═══════════════════════════════════════════════════════════"
	@echo "  Performance Test Summary"
	@echo "═══════════════════════════════════════════════════════════"
	@echo ""
	@if [ -f test-reports/performance/latest.md ]; then \
		echo "Latest Report: test-reports/performance/latest.md"; \
		echo ""; \
		head -30 test-reports/performance/latest.md; \
	else \
		echo "No performance test reports found."; \
		echo "Run 'make perf' to generate a report."; \
	fi
	@echo ""
	@echo "Container Status:"
	@docker-compose ps 2>/dev/null || echo "  No containers running"
