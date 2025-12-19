# Performance Test Runner

This container automatically runs performance tests against the EIR system when started via docker-compose.

## Usage

### Run all services including performance tests:
```bash
cd eir/test/integration
docker-compose up
```

The performance test runner will:
1. Wait for all services (eir-core, diameter-gateway, simulated-dra) to be healthy
2. Wait an additional 5 seconds for services to stabilize
3. Run all performance tests automatically
4. Exit with the test result code

### Run services without performance tests:
```bash
docker-compose up eir-core diameter-gateway simulated-dra
```

### Run only performance tests (assuming services are already running):
```bash
docker-compose up performance-test-runner
```

### View test logs:
```bash
docker-compose logs performance-test-runner
```

### Follow test logs in real-time:
```bash
docker-compose up performance-test-runner --follow
```

## Test Configuration

The performance tests run with:
- **Timeout**: 15 minutes
- **Test Pattern**: `^TestPerformance` (all performance tests)
- **Verbose Output**: Enabled (`-v` flag)

## Environment Variables

- `DRA_ADDR`: Address of the simulated DRA (default: `simulated-dra:3869`)
- `EIR_HTTP_ADDR`: Address of EIR Core HTTP API (default: `eir-core:8080`)

## Exit Codes

- `0`: All tests passed
- `1`: Tests failed or timed out
- `2`: Test compilation failed

