# EIR Integration Tests

This directory contains integration tests for the EIR (Equipment Identity Register) application with DRA (Diameter Routing Agent) simulator and mock database implementations.

## Overview

The integration tests validate the complete flow from Diameter client through DRA to EIR service using mock database backends. This allows testing the entire system without requiring a real database or external Diameter infrastructure.

## Architecture

```
┌─────────────────┐      ┌─────────────────┐      ┌─────────────────┐
│  Diameter       │      │  DRA Simulator  │      │  EIR Service    │
│  Client         │─────▶│  (diam-gw)      │─────▶│                 │
│  (Test)         │◀─────│                 │◀─────│                 │
└─────────────────┘      └─────────────────┘      └─────────────────┘
                                                            │
                                                            ▼
                                                   ┌─────────────────┐
                                                   │  Mock Database  │
                                                   │  - IMEI Repo    │
                                                   │  - Audit Repo   │
                                                   └─────────────────┘
```

## Components

### 1. DRA Simulator (`diam-gw/simulator/dra_simulator.go`)

A lightweight Diameter Routing Agent simulator that:
- Listens for TCP connections on a configurable address
- Handles Diameter base protocol messages (CER/CEA, DWR/DWA)
- Routes S13 ME-Identity-Check messages to custom handlers
- Supports concurrent connections
- Provides clean shutdown

**Key Features:**
- Customizable message handlers via callbacks
- Default handlers for base protocol messages
- Proper Diameter header parsing and message routing
- Connection lifecycle management

**Usage:**
```go
draConfig := simulator.DRAConfig{
    Address:     "127.0.0.1:3868",
    OriginHost:  "dra.test.epc.mnc001.mcc001.3gppnetwork.org",
    OriginRealm: "test.epc.mnc001.mcc001.3gppnetwork.org",
    ProductName: "DRA-Simulator/1.0",
}

draSimulator := simulator.NewDRASimulator(draConfig)

// Set custom S13 handler
draSimulator.SetS13Handler(func(ctx context.Context, req *s13.MEIdentityCheckRequest) (*s13.MEIdentityCheckAnswer, error) {
    // Your handler logic here
    return answer, nil
})

err := draSimulator.Start()
defer draSimulator.Stop()
```

### 2. Mock Database Repositories

#### Mock IMEI Repository (`internal/adapters/mocks/imei_repository_mock.go`)

Implements the `IMEIRepository` interface with in-memory storage:
- Thread-safe operations using `sync.RWMutex`
- Automatic ID generation
- Full CRUD operations
- Pagination support
- Function override capability for custom test scenarios

**Helper Methods:**
- `AddEquipment(equipment)` - Add equipment directly for test setup
- `Clear()` - Remove all equipment (test cleanup)
- `Count()` - Get equipment count

**Usage:**
```go
imeiRepo := mocks.NewMockIMEIRepository()

// Seed test data
imeiRepo.AddEquipment(&models.Equipment{
    IMEI:   "123456789012345",
    Status: models.EquipmentStatusWhitelisted,
    AddedBy: "test",
})

// Use in tests
equipment, err := imeiRepo.GetByIMEI(ctx, "123456789012345")
```

#### Mock Audit Repository (`internal/adapters/mocks/audit_repository_mock.go`)

Implements the `AuditRepository` interface:
- Thread-safe audit log storage
- Automatic timestamp assignment
- Query by IMEI and time range
- Function override capability

**Helper Methods:**
- `GetAllLogs()` - Retrieve all audit logs
- `Clear()` - Remove all logs
- `Count()` - Get log count

### 3. Integration Test Suites

#### Test Suite 1: EIR Integration (`eir_integration_test.go`)

Tests EIR service operations with DRA simulator integration:

**Test Cases:**
1. **CheckWhitelistedIMEI** - Verify whitelisted equipment returns correct status
2. **CheckBlacklistedIMEI** - Verify blacklisted equipment is detected
3. **CheckGreylistedIMEI** - Verify greylisted equipment status
4. **CheckUnknownIMEI** - Verify default policy for unknown equipment
5. **CheckInvalidIMEI** - Verify validation error handling
6. **ProvisionAndCheckIMEI** - Test provisioning new equipment
7. **IncrementCheckCounter** - Verify check counter increments correctly
8. **AuditLogging** - Verify audit logs are created with correct fields

**Run Tests:**
```bash
cd eir/test/integration
go test -v -run TestEIRIntegration_WithDRASimulator
```

#### Test Suite 2: Diameter Client Flow (`diameter_client_test.go`)

Tests full end-to-end Diameter protocol flow:

**Components:**
- **DiameterClient** - Simple Diameter client for testing
  - Handles CER/CEA exchange
  - Sends ME-Identity-Check-Request
  - Parses responses

**Test Cases:**
1. **FullFlow_CER_MECheck** - Complete CER/CEA + equipment check flow
2. **MultipleChecks** - Multiple sequential checks with counter verification

**Run Tests:**
```bash
cd eir/test/integration
go test -v -run TestDiameterClientToEIR
```

## Test Data

The tests use predefined test equipment:

| IMEI              | Status       | Reason                    | Manufacturer |
|-------------------|--------------|---------------------------|--------------|
| 123456789012345   | WHITELISTED  | Verified legitimate device| Samsung      |
| 999999999999999   | BLACKLISTED  | Stolen device             | -            |
| 555555555555555   | GREYLISTED   | Under investigation       | -            |

## Running the Tests

### Run All Integration Tests
```bash
cd eir
go test ./test/integration/... -v
```

### Run Specific Test
```bash
go test ./test/integration/... -v -run TestEIRIntegration_WithDRASimulator/CheckWhitelistedIMEI
```

### Run with Race Detection
```bash
go test ./test/integration/... -v -race
```

### Run with Coverage
```bash
go test ./test/integration/... -v -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Test Configuration

The tests use the following configuration:

- **DRA Address:** `127.0.0.1:13868` (Test 1), `127.0.0.1:13869` (Test 2)
- **DRA Origin-Host:** `dra.test.epc.mnc001.mcc001.3gppnetwork.org`
- **DRA Origin-Realm:** `test.epc.mnc001.mcc001.3gppnetwork.org`
- **Client Origin-Host:** `mme.test.epc.mnc001.mcc001.3gppnetwork.org`
- **Client Origin-Realm:** `test.epc.mnc001.mcc001.3gppnetwork.org`

## Extending the Tests

### Adding New Test Scenarios

1. **Add test data** in `seedTestData()` function:
```go
func seedTestData(repo *mocks.MockIMEIRepository) {
    repo.AddEquipment(&models.Equipment{
        IMEI:   "123456789012345",
        Status: models.EquipmentStatusWhitelisted,
        // ...
    })
}
```

2. **Create test function**:
```go
func testMyNewScenario(t *testing.T, ctx context.Context, eirService ports.EIRService, ...) {
    // Your test logic
}
```

3. **Add to test suite**:
```go
t.Run("MyNewScenario", func(t *testing.T) {
    testMyNewScenario(t, ctx, eirService, imeiRepo, auditRepo)
})
```

### Custom Mock Behavior

Override mock functions for specific test scenarios:

```go
imeiRepo := mocks.NewMockIMEIRepository()

// Override GetByIMEI to simulate database error
imeiRepo.GetByIMEIFunc = func(ctx context.Context, imei string) (*models.Equipment, error) {
    return nil, errors.New("database connection failed")
}

// Test error handling
_, err := eirService.CheckEquipment(ctx, request)
if err == nil {
    t.Error("Expected error, got nil")
}
```

## Integration with CI/CD

### GitHub Actions Example
```yaml
name: Integration Tests
on: [push, pull_request]

jobs:
  integration-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: '1.21'
      - name: Run Integration Tests
        run: |
          cd eir
          go test ./test/integration/... -v -race
```

## Troubleshooting

### Port Already in Use
If you get "address already in use" errors:
```bash
# Find process using the port
lsof -i :13868
# Kill the process
kill -9 <PID>
```

### Tests Hang
- Check that `defer draSimulator.Stop()` is called
- Verify no goroutine leaks with `-race` flag
- Increase timeouts if running on slow machines

### Connection Refused
- Ensure DRA simulator is started before client connection
- Add small sleep after `draSimulator.Start()`: `time.Sleep(100 * time.Millisecond)`

## Performance Considerations

- **Concurrent Tests:** Each test uses a different port to allow parallel execution
- **Mock Repository:** Thread-safe with RWMutex for concurrent access
- **Resource Cleanup:** All tests properly clean up resources using `defer`

## Best Practices

1. **Isolation:** Each test should be independent and not rely on others
2. **Cleanup:** Always defer cleanup operations (Stop, Close, Clear)
3. **Assertions:** Use clear, descriptive error messages
4. **Test Data:** Keep test data minimal but representative
5. **Timeouts:** Use contexts with timeouts for network operations

## Future Enhancements

Potential improvements to the test suite:

1. **Load Testing:** Add benchmarks for high-throughput scenarios
2. **Error Injection:** Test network failures, timeouts, malformed messages
3. **Protocol Validation:** Verify strict Diameter RFC compliance
4. **Multi-Client:** Test concurrent clients to DRA
5. **Metrics Validation:** Verify Prometheus metrics are updated correctly
6. **Cache Testing:** Add Redis mock for cache repository tests

## References

- [3GPP TS 29.272](https://www.3gpp.org/ftp/Specs/html-info/29272.htm) - S13 Interface Specification
- [RFC 6733](https://tools.ietf.org/html/rfc6733) - Diameter Base Protocol
- [3GPP TS 23.003](https://www.3gpp.org/ftp/Specs/html-info/23003.htm) - IMEI Specification
- [Go Testing Package](https://pkg.go.dev/testing) - Go Testing Documentation

## License

Same as parent project.
