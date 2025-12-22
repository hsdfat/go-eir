# PCAP Testing Guide

This guide explains how to write unit tests that generate PCAP files for network traffic analysis while testing your functionality.

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [HTTP/1.1 Tests with PCAP](#http11-tests-with-pcap)
4. [Diameter Tests with PCAP](#diameter-tests-with-pcap)
5. [PCAP Writer API](#pcap-writer-api)
6. [Best Practices](#best-practices)
7. [Analyzing PCAP Files](#analyzing-pcap-files)

---

## Overview

The PCAP testing framework allows you to:
- ✅ Test your network protocols (HTTP, Diameter, etc.)
- ✅ Capture actual network traffic during tests
- ✅ Generate Wireshark-compatible PCAP files
- ✅ Verify protocol correctness with packet analysis
- ✅ Debug network issues with real packet captures

**Key Features:**
- Automatic TCP handshake generation (SYN, SYN-ACK, ACK)
- Proper sequence number tracking
- Support for HTTP/1.1, HTTP/2, and Diameter protocols
- Thread-safe PCAP writing

---

## Quick Start

### 1. Import Required Packages

```go
import (
    "testing"
    "github.com/hsdfat8/eir/internal/adapters/testutil"
)
```

### 2. Create PCAP Writer

```go
func TestMyFunction(t *testing.T) {
    // Create PCAP writer in the same directory as test file
    pcapFile := "my_test_8080.pcap"
    pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
    if err != nil {
        t.Fatalf("Failed to create PCAP writer: %v", err)
    }
    defer pcapWriter.Close()

    // Your test code here...
}
```

### 3. Wrap Network Connections

```go
// Wrap your connection with PCAP capture
captureConn := testutil.NewCaptureConnection(conn, pcapWriter)

// Use captureConn instead of conn
// All read/write operations will be captured to PCAP
```

---

## HTTP/1.1 Tests with PCAP

HTTP/1.1 is recommended for easy Wireshark decoding (plaintext protocol).

### Complete Example

```go
package http

import (
    "context"
    "fmt"
    "net"
    "net/http"
    "testing"
    "time"

    "github.com/hsdfat8/eir/internal/adapters/testutil"
)

func TestHTTP1WithPCAP(t *testing.T) {
    // Step 1: Create PCAP writer
    pcapFile := "http1_test_8080.pcap"
    pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
    if err != nil {
        t.Fatalf("Failed to create PCAP writer: %v", err)
    }
    defer pcapWriter.Close()

    // Step 2: Start your HTTP server
    config := ServerConfig{
        ListenAddr: "127.0.0.1:8080",
        EnableH2C:  false, // HTTP/1.1 mode
    }

    server := NewServer(config, mockService)
    if err := server.Start(); err != nil {
        t.Fatalf("Failed to start server: %v", err)
    }
    defer server.Stop()

    time.Sleep(100 * time.Millisecond) // Wait for server to start

    // Step 3: Create HTTP client with PCAP capture
    dialer := &net.Dialer{}
    transport := &http.Transport{
        DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            conn, err := dialer.DialContext(ctx, network, addr)
            if err != nil {
                return nil, err
            }
            // Wrap connection with PCAP capture
            return testutil.NewCaptureConnection(conn, pcapWriter), nil
        },
    }

    client := &http.Client{
        Transport: transport,
        Timeout:   5 * time.Second,
    }

    // Step 4: Perform your HTTP requests (traffic is captured)
    t.Run("GetRequest", func(t *testing.T) {
        resp, err := client.Get("http://127.0.0.1:8080/health")
        if err != nil {
            t.Fatalf("Request failed: %v", err)
        }
        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            t.Errorf("Expected 200, got %d", resp.StatusCode)
        }

        t.Logf("✓ HTTP/%d.%d request successful", resp.ProtoMajor, resp.ProtoMinor)
    })

    // Step 5: PCAP file is automatically saved
    t.Logf("PCAP file saved: %s", pcapFile)
}
```

### What Gets Captured

The PCAP file will contain:
1. **TCP Handshake** (3 packets):
   - Client → Server: SYN
   - Server → Client: SYN-ACK
   - Client → Server: ACK

2. **HTTP Request** (readable in Wireshark):
   ```
   GET /health HTTP/1.1
   Host: 127.0.0.1:8080
   User-Agent: Go-http-client/1.1
   ```

3. **HTTP Response** (readable in Wireshark):
   ```
   HTTP/1.1 200 OK
   Content-Type: application/json

   {"status":"healthy"}
   ```

---

## Diameter Tests with PCAP

Diameter protocol testing with PCAP capture for S13 interface.

### Complete Example

```go
package diameter

import (
    "net"
    "testing"
    "time"

    "github.com/hsdfat8/eir/internal/adapters/testutil"
)

func TestDiameterS13WithPCAP(t *testing.T) {
    // Step 1: Create PCAP writer
    pcapFile := "diameter_s13_3868.pcap"
    pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
    if err != nil {
        t.Fatalf("Failed to create PCAP writer: %v", err)
    }
    defer pcapWriter.Close()

    // Step 2: Start Diameter server
    config := ServerConfig{
        ListenAddr:  "127.0.0.1:3868", // Standard Diameter port
        OriginHost:  "eir-test.example.com",
        OriginRealm: "example.com",
        ProductName: "EIR-Test/1.0",
        VendorID:    10415,
    }

    server := NewServer(config, mockService)
    if err := server.Start(); err != nil {
        t.Fatalf("Failed to start server: %v", err)
    }
    defer server.Stop()

    time.Sleep(100 * time.Millisecond)

    // Step 3: Create client connection with PCAP capture
    conn, err := net.Dial("tcp", "127.0.0.1:3868")
    if err != nil {
        t.Fatalf("Failed to connect: %v", err)
    }

    // Wrap connection with PCAP capture
    captureConn := testutil.NewCaptureConnection(conn, pcapWriter)
    defer captureConn.Close()

    // Step 4: Send Diameter messages (traffic is captured)

    // Send CER (Capabilities-Exchange-Request)
    cer := buildCER() // Your CER message builder
    cerBytes, _ := cer.Marshal()

    if _, err := captureConn.Write(cerBytes); err != nil {
        t.Fatalf("Failed to send CER: %v", err)
    }
    t.Log("✓ Sent CER")

    // Read CEA (Capabilities-Exchange-Answer)
    ceaBytes := readDiameterMessage(t, captureConn)
    cea := parseCEA(ceaBytes) // Your CEA parser

    if cea.ResultCode != 2001 {
        t.Errorf("Expected ResultCode 2001, got %d", cea.ResultCode)
    }
    t.Log("✓ Received CEA")

    // Send ECR (Equipment-Check-Request)
    ecr := buildECR("123456789012345") // Your ECR message builder
    ecrBytes, _ := ecr.Marshal()

    if _, err := captureConn.Write(ecrBytes); err != nil {
        t.Fatalf("Failed to send ECR: %v", err)
    }
    t.Log("✓ Sent ECR")

    // Read ECA (Equipment-Check-Answer)
    ecaBytes := readDiameterMessage(t, captureConn)
    eca := parseECA(ecaBytes) // Your ECA parser

    if eca.ResultCode != 2001 {
        t.Errorf("Expected ResultCode 2001, got %d", eca.ResultCode)
    }
    t.Logf("✓ Received ECA: Equipment-Status=%d", eca.EquipmentStatus)

    // Step 5: PCAP file is automatically saved
    t.Logf("PCAP file saved: %s", pcapFile)
}
```

### What Gets Captured

The PCAP file will contain:
1. **TCP Handshake** (3 packets)
2. **Diameter CER** (Capabilities-Exchange-Request)
   - Command Code: 257
   - Application ID: 0
   - Origin-Host, Origin-Realm AVPs
3. **Diameter CEA** (Capabilities-Exchange-Answer)
   - Result-Code: 2001 (DIAMETER_SUCCESS)
4. **Diameter ECR** (Equipment-Check-Request)
   - Command Code: 324 (S13 interface)
   - IMEI AVP with device identifier
5. **Diameter ECA** (Equipment-Check-Answer)
   - Equipment-Status AVP

---

## PCAP Writer API

### PCAPWriter Methods

#### `NewPCAPWriter(filename string) (*PCAPWriter, error)`

Creates a new PCAP writer.

```go
pcapWriter, err := testutil.NewPCAPWriter("my_test.pcap")
if err != nil {
    return err
}
defer pcapWriter.Close()
```

**Features:**
- Automatically creates PCAP file
- Writes PCAP file header
- Initializes TCP sequence tracking

---

#### `WritePacket(data []byte, srcIP, dstIP net.IP, srcPort, dstPort uint16, isInbound bool) error`

Writes a single packet to PCAP file.

```go
srcIP := net.IPv4(127, 0, 0, 1)
dstIP := net.IPv4(127, 0, 0, 1)

// Write outbound packet (client → server)
err := pcapWriter.WritePacket(
    requestData,
    srcIP,
    dstIP,
    12345,  // source port
    8080,   // destination port
    false,  // isInbound = false (outbound)
)

// Write inbound packet (server → client)
err = pcapWriter.WritePacket(
    responseData,
    dstIP,
    srcIP,
    8080,   // source port
    12345,  // destination port
    true,   // isInbound = true
)
```

**Parameters:**
- `data`: Packet payload (HTTP, Diameter, etc.)
- `srcIP`: Source IP address
- `dstIP`: Destination IP address
- `srcPort`: Source port
- `dstPort`: Destination port
- `isInbound`: `true` for server→client, `false` for client→server

**Automatic Features:**
- TCP handshake on first packet
- Sequence number tracking
- Proper TCP flags (SYN, ACK, PSH)

---

#### `Close() error`

Closes the PCAP writer and flushes data to disk.

```go
defer pcapWriter.Close()
```

---

### CaptureConnection Methods

#### `NewCaptureConnection(conn net.Conn, pcapWriter *PCAPWriter) *CaptureConnection`

Wraps a network connection to automatically capture traffic.

```go
conn, _ := net.Dial("tcp", "127.0.0.1:8080")
captureConn := testutil.NewCaptureConnection(conn, pcapWriter)

// All Read() and Write() operations are automatically captured
captureConn.Write(requestData)
captureConn.Read(responseBuffer)
```

**Implements:** `net.Conn` interface

**Captured Operations:**
- `Read()` - Inbound packets (server → client)
- `Write()` - Outbound packets (client → server)

---

## Best Practices

### 1. Naming Convention

Use descriptive filenames with protocol and port:

```go
// Good ✅
"http1_8080_test.pcap"
"http2_h2c_8080_test.pcap"
"diameter_s13_3868_test.pcap"

// Bad ❌
"test.pcap"
"output.pcap"
```

### 2. File Location

Place PCAP files in the same directory as test files:

```go
// ✅ Correct - relative to test file
pcapFile := "my_test_8080.pcap"

// ❌ Wrong - absolute paths
pcapFile := "/tmp/test.pcap"
```

### 3. Use Standard Ports

Use standard protocol ports for Wireshark auto-detection:

```go
// ✅ Wireshark auto-detects HTTP
ListenAddr: "127.0.0.1:8080"

// ✅ Wireshark auto-detects Diameter
ListenAddr: "127.0.0.1:3868"

// ❌ Random port - requires manual dissector selection
ListenAddr: "127.0.0.1:0"
```

### 4. Always Close PCAP Writer

Use `defer` to ensure PCAP files are properly closed:

```go
pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
if err != nil {
    t.Fatalf("Failed to create PCAP writer: %v", err)
}
defer pcapWriter.Close() // ✅ Always close
```

### 5. HTTP/1.1 for Readability

Use HTTP/1.1 instead of HTTP/2 for easier debugging:

```go
// ✅ HTTP/1.1 - plaintext, easy to read
config := ServerConfig{
    EnableH2C: false, // HTTP/1.1 mode
}

// ⚠️ HTTP/2 - binary protocol, harder to debug
config := ServerConfig{
    EnableH2C: true, // HTTP/2 mode
}
```

### 6. Log PCAP File Path

Always log the PCAP file path at the end of the test:

```go
t.Logf("PCAP file saved: %s", pcapFile)
```

### 7. Subtests for Multiple Scenarios

Use subtests to organize multiple test cases in one PCAP file:

```go
t.Run("HealthCheck", func(t *testing.T) {
    // Test health endpoint
})

t.Run("GetEquipmentStatus", func(t *testing.T) {
    // Test equipment status
})

t.Run("ProvisionEquipment", func(t *testing.T) {
    // Test provisioning
})
```

---

## Analyzing PCAP Files

### Using tcpdump

**View all packets:**
```bash
tcpdump -r http1_8080_test.pcap -n
```

**View with ASCII content:**
```bash
tcpdump -r http1_8080_test.pcap -n -A
```

**View with hex dump:**
```bash
tcpdump -r http1_8080_test.pcap -n -X
```

**Filter by port:**
```bash
tcpdump -r diameter_s13_3868.pcap -n port 3868
```

**Count packets:**
```bash
tcpdump -r http1_8080_test.pcap -n | wc -l
```

### Using Wireshark

**Open PCAP file:**
```bash
wireshark http1_8080_test.pcap
```

**Useful Wireshark Filters:**

```
# HTTP traffic only
http

# Specific HTTP method
http.request.method == "GET"

# Diameter traffic
diameter

# Diameter command code
diameter.cmd.code == 324

# TCP stream
tcp.stream eq 0

# Specific port
tcp.port == 8080
```

**Follow TCP Stream:**
1. Right-click on any packet
2. Select "Follow" → "TCP Stream"
3. View complete conversation

### Using tshark (CLI Wireshark)

**List protocols:**
```bash
tshark -r http1_8080_test.pcap
```

**Extract HTTP:**
```bash
tshark -r http1_8080_test.pcap -Y http -T fields -e http.request.method -e http.request.uri
```

**Extract Diameter:**
```bash
tshark -r diameter_s13_3868.pcap -Y diameter -T fields -e diameter.cmd.code -e diameter.resultcode
```

---

## Verification Test Example

Create a test to verify PCAP file structure:

```go
package testutil

import (
    "testing"
    "github.com/google/gopacket"
    "github.com/google/gopacket/layers"
    "github.com/google/gopacket/pcap"
)

func TestPCAPVerification(t *testing.T) {
    // Open PCAP file
    handle, err := pcap.OpenOffline("my_test.pcap")
    if err != nil {
        t.Fatalf("Failed to open PCAP: %v", err)
    }
    defer handle.Close()

    packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

    tcpHandshakeFound := false
    httpFound := false

    for packet := range packetSource.Packets() {
        // Check for TCP handshake
        if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
            tcp, _ := tcpLayer.(*layers.TCP)
            if tcp.SYN && !tcp.ACK {
                tcpHandshakeFound = true
                t.Log("✓ Found TCP SYN handshake")
            }
        }

        // Check for HTTP
        if appLayer := packet.ApplicationLayer(); appLayer != nil {
            payload := appLayer.Payload()
            if len(payload) > 0 && string(payload[:4]) == "GET " {
                httpFound = true
                t.Log("✓ Found HTTP GET request")
            }
        }
    }

    if !tcpHandshakeFound {
        t.Error("❌ TCP handshake not found")
    }
    if !httpFound {
        t.Error("❌ HTTP request not found")
    }
}
```

---

## Complete Real-World Example

See the following files for complete examples:

1. **HTTP/1.1 Test:**
   - File: `internal/adapters/http/server_test.go`
   - Function: `TestServerHTTP1WithPCAP`
   - PCAP: `http1_8080_test.pcap`

2. **Diameter S13 Test:**
   - File: `internal/adapters/diameter/server_test.go`
   - Function: `TestServerS13MEIdentityCheck`
   - PCAP: `diameter_s13_3868_test.pcap`

3. **PCAP Verification:**
   - File: `internal/adapters/testutil/pcap_test.go`
   - Function: `TestPCAPWriteRead`

---

## Troubleshooting

### PCAP File is Empty

**Cause:** PCAP writer not closed properly.

**Solution:** Always use `defer pcapWriter.Close()`

### Wireshark Shows "Malformed Packet"

**Cause:** Incorrect TCP sequence numbers or missing handshake.

**Solution:** The PCAP writer automatically handles this. Make sure you're using the latest version.

### Protocol Not Auto-Detected in Wireshark

**Cause:** Non-standard port used.

**Solution:**
1. Use standard ports (8080 for HTTP, 3868 for Diameter)
2. Or manually set dissector: Right-click → Decode As → Select protocol

### Connection Capture Not Working

**Cause:** Forgot to wrap connection with `NewCaptureConnection`.

**Solution:**
```go
// ❌ Wrong - not captured
conn, _ := net.Dial("tcp", "127.0.0.1:8080")

// ✅ Correct - captured
conn, _ := net.Dial("tcp", "127.0.0.1:8080")
captureConn := testutil.NewCaptureConnection(conn, pcapWriter)
```

---

## Summary

**To write a unit test with PCAP generation:**

1. Create `PCAPWriter` at test start
2. Wrap network connections with `NewCaptureConnection`
3. Perform your test operations normally
4. Close PCAP writer with `defer`
5. Analyze PCAP file with Wireshark or tcpdump

**Key Benefits:**
- ✅ Real packet captures for debugging
- ✅ Protocol compliance verification
- ✅ Automatic TCP stream reconstruction
- ✅ Wireshark-compatible format
- ✅ No impact on test logic

---

**For more examples, see:**
- `/internal/adapters/http/server_test.go`
- `/internal/adapters/diameter/server_test.go`
- `/internal/adapters/testutil/pcap_test.go`
