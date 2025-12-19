# Integration Test Architecture

## Overview

The integration tests implement a complete telecommunications stack for testing the EIR (Equipment Identity Register) application.

## Full Stack Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Integration Test Environment                        │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────┐         ┌──────────────────┐         ┌──────────────────┐
│  Diameter       │         │  DRA Simulator   │         │  EIR Diameter    │
│  Client         │         │  (Proxy)         │         │  Server          │
│                 │         │                  │         │  (Gateway)       │
│  Port: Dynamic  │◄───────►│  Port: 13871     │◄───────►│  Port: 13870     │
│                 │   TCP   │                  │   TCP   │                  │
│  Origin:        │         │  Origin:         │         │  Origin:         │
│  mme.test...    │         │  dra.test...     │         │  eir.test...     │
└─────────────────┘         └──────────────────┘         └──────────────────┘
        │                            │                              │
        │ 1. CER/CEA                 │ 2. Proxy CER/CEA            │
        │────────────────────────────┼─────────────────────────────►│
        │                            │                              │
        │ 3. ME-Identity-Check-Req   │ 4. Proxy Request            │
        │────────────────────────────┼─────────────────────────────►│
        │                            │                              │
        │                            │                              ▼
        │                            │                    ┌──────────────────┐
        │                            │                    │  S13Handler      │
        │                            │                    │  (Adapter)       │
        │                            │                    └──────────────────┘
        │                            │                              │
        │                            │                              ▼
        │                            │                    ┌──────────────────┐
        │                            │                    │  EIR Service     │
        │                            │                    │  (Business Logic)│
        │                            │                    └──────────────────┘
        │                            │                         │       │
        │                            │                         ▼       ▼
        │                            │              ┌─────────────┐ ┌─────────────┐
        │                            │              │  IMEI Repo  │ │ Audit Repo  │
        │                            │              │  (Mock)     │ │ (Mock)      │
        │                            │              └─────────────┘ └─────────────┘
        │                            │                              │
        │                            │ 5. ME-Identity-Check-Answer │
        │◄───────────────────────────┼──────────────────────────────┘
        │                            │
        ▼                            ▼
   Test Assertions            Routing Metrics
```

## Component Layers

### Layer 1: Diameter Client (Test Simulator)
**File:** `diameter_client_test.go` (DiameterClient struct)

**Responsibilities:**
- Simulates MME (Mobility Management Entity)
- Establishes TCP connection to DRA
- Performs CER/CEA capability exchange
- Sends ME-Identity-Check-Request messages
- Receives and parses ME-Identity-Check-Answer messages

**Key Functions:**
- `Connect()` - Establish connection and CER/CEA exchange
- `CheckEquipment(imei)` - Send equipment check request
- `Close()` - Clean up connection

**Protocol Details:**
- Diameter Base Protocol (RFC 6733)
- Command Code 257 (CER/CEA)
- Command Code 324 (ME-Identity-Check)
- Application ID: 16777252 (S13)

### Layer 2: DRA Simulator (Diameter Routing Agent)
**File:** `diam-gw/simulator/dra_simulator.go`

**Responsibilities:**
- Accept incoming Diameter connections
- Route messages between clients and servers
- Handle base protocol messages (CER/CEA, DWR/DWA)
- Proxy S13 messages to EIR server
- Maintain connection state

**Features:**
- Concurrent connection handling
- Customizable message handlers
- Graceful shutdown
- Connection lifecycle management

**Configuration:**
```go
DRAConfig{
    Address:     "127.0.0.1:13871",
    OriginHost:  "dra.test.epc.mnc001.mcc001.3gppnetwork.org",
    OriginRealm: "test.epc.mnc001.mcc001.3gppnetwork.org",
    ProductName: "DRA-Simulator/1.0",
}
```

**Proxy Mode:**
The DRA can act as a transparent proxy:
```go
draSimulator.SetS13Handler(func(ctx, req) (*answer, error) {
    // Forward request to EIR server
    conn, _ := net.Dial("tcp", eirServerAddr)
    conn.Write(req.Marshal())

    // Read response
    response := readDiameterMessage(conn)
    return response, nil
})
```

### Layer 3: EIR Diameter Server (Protocol Gateway)
**File:** `full_stack_test.go` (EIRDiameterServer struct)

**Responsibilities:**
- Listen for incoming Diameter connections
- Parse Diameter protocol messages
- Route messages to S13 handler
- Handle base protocol (CER/CEA, DWR/DWA)
- Marshal/unmarshal Diameter AVPs

**Message Flow:**
1. Accept TCP connection
2. Read 20-byte Diameter header
3. Parse message length and command code
4. Read message body
5. Route to appropriate handler
6. Send response back

**Handler Integration:**
```go
s.handler = diameter.NewS13Handler(eirService, originHost, originRealm)
answer, err := s.handler.HandleMEIdentityCheckRequest(ctx, req)
```

### Layer 4: S13 Handler (Adapter Layer)
**File:** `eir/internal/adapters/diameter/s13_handler.go`

**Responsibilities:**
- Convert Diameter messages to domain requests
- Extract IMEI from Terminal-Information AVP
- Call EIR service business logic
- Build Diameter answer messages
- Map equipment status to Diameter enumerations

**Mapping:**
```go
// Diameter -> Domain
IMEI := string(*req.TerminalInformation.Imei)
eirRequest := &ports.CheckEquipmentRequest{
    IMEI: IMEI,
    RequestSource: "DIAMETER_S13",
}

// Domain -> Diameter
answer.EquipmentStatus = models.ToDialDialStatus(response.Status)
```

### Layer 5: EIR Service (Business Logic)
**File:** `eir/internal/domain/service/eir_service.go`

**Responsibilities:**
- IMEI validation (Luhn algorithm, format)
- Equipment lookup in repository
- Apply default policy for unknown equipment
- Increment check counter atomically
- Log audit trail
- Cache management (if enabled)

**Core Logic Flow:**
```
1. Validate IMEI format
2. Check cache (optional)
3. Query IMEI repository
4. If not found, apply default policy (WHITELISTED)
5. Increment check counter
6. Log to audit repository
7. Return equipment status
```

### Layer 6: Mock Repositories (Data Layer)
**Files:**
- `eir/internal/adapters/mocks/imei_repository_mock.go`
- `eir/internal/adapters/mocks/audit_repository_mock.go`

**Responsibilities:**
- In-memory data storage
- Thread-safe operations (sync.RWMutex)
- CRUD operations
- Pagination support
- Test data seeding

**Thread Safety:**
```go
func (m *MockIMEIRepository) GetByIMEI(ctx, imei) (*Equipment, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    equipment, exists := m.equipment[imei]
    // ...
}
```

## Message Flow Example

### Equipment Check Request Flow

```
1. Client creates ME-Identity-Check-Request
   ├─ SessionId: "mme.test.epc;123456;1"
   ├─ OriginHost: "mme.test.epc.mnc001.mcc001.3gppnetwork.org"
   ├─ OriginRealm: "test.epc.mnc001.mcc001.3gppnetwork.org"
   ├─ DestinationRealm: "test.epc.mnc001.mcc001.3gppnetwork.org"
   └─ TerminalInformation:
      └─ IMEI: "123456789012345"

2. Client marshals to Diameter wire format
   ├─ Header: 20 bytes (version, length, flags, command code, IDs)
   └─ AVPs: Variable length (session-id, origin, terminal-info)

3. Client sends via TCP to DRA (port 13871)

4. DRA receives and parses message
   ├─ Reads header (20 bytes)
   ├─ Extracts command code: 324 (ME-Identity-Check)
   └─ Routes to S13 handler

5. DRA proxies to EIR server (port 13870)
   ├─ Establishes TCP connection
   ├─ Forwards request bytes
   └─ Waits for response

6. EIR Diameter Server receives message
   ├─ Parses Diameter header
   ├─ Routes to S13Handler
   └─ Calls HandleMEIdentityCheckRequest()

7. S13Handler converts to domain request
   ├─ Extracts IMEI: "123456789012345"
   ├─ Builds CheckEquipmentRequest
   └─ Calls eirService.CheckEquipment()

8. EIR Service processes request
   ├─ Validates IMEI format (✓)
   ├─ Queries IMEIRepository.GetByIMEI()
   ├─ Found: WHITELISTED
   ├─ Increments check counter
   └─ Logs to AuditRepository

9. S13Handler builds Diameter answer
   ├─ ResultCode: 2001 (DIAMETER_SUCCESS)
   ├─ EquipmentStatus: 0 (WHITELISTED)
   └─ Copies session info from request

10. EIR Server marshals and sends response

11. DRA receives response from EIR
    └─ Forwards to client

12. Client receives ME-Identity-Check-Answer
    ├─ Unmarshals Diameter message
    ├─ Checks ResultCode: 2001 ✓
    └─ Checks EquipmentStatus: 0 (WHITELISTED) ✓

13. Test assertions verify
    ├─ Response status is correct
    ├─ Check counter was incremented
    └─ Audit log was created
```

## Test Data Setup

### Predefined Equipment

```go
// Whitelisted device
Equipment{
    IMEI:             "123456789012345",
    Status:           WHITELISTED,
    Reason:           "Verified legitimate device",
    ManufacturerTAC:  "12345678",
    ManufacturerName: "Samsung",
}

// Blacklisted device (stolen)
Equipment{
    IMEI:   "999999999999999",
    Status: BLACKLISTED,
    Reason: "Stolen device",
}

// Greylisted device (under investigation)
Equipment{
    IMEI:   "555555555555555",
    Status: GREYLISTED,
    Reason: "Under investigation",
}
```

## Network Configuration

| Component           | Address          | Port  | Origin Host                                    |
|---------------------|------------------|-------|------------------------------------------------|
| Diameter Client     | Dynamic          | -     | mme.test.epc.mnc001.mcc001.3gppnetwork.org     |
| DRA Simulator       | 127.0.0.1        | 13871 | dra.test.epc.mnc001.mcc001.3gppnetwork.org     |
| EIR Diameter Server | 127.0.0.1        | 13870 | eir.test.epc.mnc001.mcc001.3gppnetwork.org     |

## Diameter Protocol Details

### Command Codes
- **257**: Capabilities-Exchange-Request/Answer (CER/CEA)
- **280**: Device-Watchdog-Request/Answer (DWR/DWA)
- **324**: ME-Identity-Check-Request/Answer (S13)

### Application IDs
- **16777252**: S13 (EIR interface)

### AVP Codes (S13 specific)
- **1402**: IMEI (Terminal-Information)
- **1403**: Software-Version (Terminal-Information)
- **1445**: Equipment-Status

### Result Codes
- **2001**: DIAMETER_SUCCESS
- **5004**: DIAMETER_INVALID_AVP_VALUE
- **5012**: DIAMETER_UNABLE_TO_COMPLY

### Equipment Status Values
- **0**: WHITELISTED (permitted)
- **1**: BLACKLISTED (prohibited)
- **2**: GREYLISTED (under observation)

## Concurrency Model

### Thread Safety Guarantees

1. **Mock Repositories**
   - Protected by `sync.RWMutex`
   - Read operations: multiple concurrent readers
   - Write operations: exclusive access

2. **DRA Simulator**
   - One goroutine per connection
   - Shared state protected by mutex
   - Graceful shutdown via channels

3. **EIR Diameter Server**
   - Concurrent connection handling
   - Stateless request processing
   - Thread-safe service calls

### Concurrent Test Example

```go
// 5 clients, 3 checks each = 15 total operations
for i := 0; i < 5; i++ {
    go func(clientID int) {
        client := createClient(clientID)
        client.Connect()
        defer client.Close()

        for j := 0; j < 3; j++ {
            client.CheckEquipment("123456789012345")
        }
    }(i)
}
```

## Performance Characteristics

### Expected Latencies (local testing)
- **CER/CEA exchange**: < 10ms
- **ME-Identity-Check**: < 20ms
- **Concurrent 5 clients**: < 100ms total

### Resource Usage
- **Memory**: ~5MB per test suite
- **Ports**: 2 ports per test (DRA + EIR)
- **Goroutines**: ~5-10 per active test

## Error Handling

### Connection Errors
- Retry logic in client
- Graceful shutdown on server
- Timeout protection (30s read timeout)

### Protocol Errors
- Invalid message format: Return error result code
- Missing required AVPs: DIAMETER_INVALID_AVP_VALUE (5004)
- Service errors: DIAMETER_UNABLE_TO_COMPLY (5012)

### Data Errors
- Invalid IMEI: Validation error before database query
- Unknown IMEI: Apply default policy (WHITELISTED)
- Database errors: Mocked as test scenarios

## Cleanup and Shutdown

### Proper Cleanup Order
```go
1. Stop test assertions
2. Close client connections
3. Stop DRA simulator (closes listener, connections)
4. Stop EIR server (closes listener, connections)
5. Clear mock repositories
```

### Defer Pattern
```go
defer draSimulator.Stop()
defer eirServer.Stop()
defer client.Close()
defer imeiRepo.Clear()
defer auditRepo.Clear()
```

## Debugging Tips

### Enable Verbose Logging
```bash
go test -v ./test/integration/...
```

### Network Debugging
```bash
# Monitor traffic
sudo tcpdump -i lo0 -X port 13870 or port 13871

# Check port usage
lsof -i :13870
lsof -i :13871
```

### Test Specific Scenario
```bash
go test -v -run TestFullStackIntegration/CompleteFlow
```

## References

- [3GPP TS 29.272](https://www.3gpp.org/ftp/Specs/html-info/29272.htm) - S13 Interface
- [RFC 6733](https://tools.ietf.org/html/rfc6733) - Diameter Base Protocol
- [3GPP TS 23.003](https://www.3gpp.org/ftp/Specs/html-info/23003.htm) - IMEI Format
