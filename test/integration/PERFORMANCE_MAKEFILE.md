# Performance Test Makefile Guide

This guide explains how to use the Makefile targets for running and reporting performance tests.

## Quick Start

```bash
# Run all performance tests and generate report
make perf
```

This single command will:
1. Clean previous test artifacts
2. Build and start all Docker services
3. Run all performance tests
4. Extract results and generate a markdown report
5. Display a summary

## Available Targets

### Main Targets

#### `make perf`
**Recommended** - Run complete performance test suite with reporting.

```bash
make perf
```

This will:
- Clean old reports
- Start all services (eir-core, diameter-gateway, simulated-dra)
- Run performance-test-runner container
- Generate timestamped report in `test-reports/performance/`
- Display summary of results

#### `make perf-up`
Start all services and run performance tests (keeps containers running).

```bash
make perf-up
```

#### `make perf-down`
Stop all services.

```bash
make perf-down
```

### Reporting Targets

#### `make perf-report`
Extract and display test results from container logs.

```bash
make perf-report
```

Generates:
- `test-reports/performance/performance_YYYYMMDD_HHMMSS.md` - Timestamped report
- `test-reports/performance/latest.md` - Symlink to latest report

#### `make perf-summary`
Display summary of latest test results.

```bash
make perf-summary
```

Shows:
- Latest report location
- First 30 lines of the report
- Container status

#### `make perf-logs`
View performance test runner logs.

```bash
make perf-logs
```

### Utility Targets

#### `make perf-setup`
Build and start services without running tests.

```bash
make perf-setup
```

Useful for:
- Manual testing
- Debugging services
- Running tests manually later

#### `make perf-teardown`
Stop and remove all containers and volumes.

```bash
make perf-teardown
```

#### `make perf-clean`
Clean performance test artifacts (reports and logs).

```bash
make perf-clean
```

#### `make perf-quick`
Run quick performance test (single concurrency level only).

```bash
make perf-quick
```

Runs only `TestPerformance_Throughput/Concurrency_1` for faster feedback.

## Report Structure

Reports are generated in Markdown format with the following sections:

1. **Test Execution Summary**
   - Passed/Failed/Skipped counts
   - Total test count

2. **Test Results**
   - Test execution status
   - Key metrics (Throughput, Latency, etc.)

3. **Detailed Test Output**
   - Full test logs
   - Performance metrics

4. **Performance Metrics**
   - Concurrency levels tested
   - Throughput (RPS)
   - Latency (P50, P95, P99)
   - Success rates

## Report Location

All reports are stored in:
```
eir/test/integration/test-reports/performance/
├── performance_20251218_122503.md  # Timestamped report
├── test_output_20251218_122503.log # Full test output
└── latest.md                        # Symlink to latest report
```

## Examples

### Full Test Suite
```bash
# Run everything
make perf

# View results
make perf-summary

# View detailed logs
make perf-logs
```

### Quick Validation
```bash
# Quick test only
make perf-quick

# Check results
cat test-reports/performance/latest.md
```

### Manual Testing
```bash
# Start services
make perf-setup

# Run tests manually (in another terminal)
docker-compose run --rm performance-test-runner

# Generate report
make perf-report

# Cleanup
make perf-teardown
```

### CI/CD Integration
```bash
# In CI pipeline
make perf
EXIT_CODE=$?

# Extract results
make perf-report

# Archive reports
tar -czf performance-reports-$(date +%Y%m%d).tar.gz test-reports/performance/

exit $EXIT_CODE
```

## Troubleshooting

### Services Not Starting
```bash
# Check service status
docker-compose ps

# View service logs
docker-compose logs eir-core
docker-compose logs diameter-gateway
docker-compose logs simulated-dra
```

### Tests Timing Out
```bash
# Increase timeout in docker-compose.yml
# Or run quick test first
make perf-quick
```

### No Reports Generated
```bash
# Check if tests ran
make perf-logs

# Manually generate report
make perf-report
```

### Clean Start
```bash
# Full cleanup and restart
make perf-teardown
make perf-clean
make perf
```

## Integration with CI/CD

Example GitHub Actions workflow:

```yaml
name: Performance Tests

on: [push, pull_request]

jobs:
  performance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Run Performance Tests
        run: |
          cd eir/test/integration
          make perf
          
      - name: Upload Reports
        uses: actions/upload-artifact@v3
        with:
          name: performance-reports
          path: eir/test/integration/test-reports/performance/
```

## Best Practices

1. **Always use `make perf`** for full test suite
2. **Check `perf-summary`** after tests complete
3. **Review `latest.md`** for detailed metrics
4. **Use `perf-quick`** for rapid iteration during development
5. **Clean artifacts** with `perf-clean` before important runs
6. **Archive reports** for historical comparison

