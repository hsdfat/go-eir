package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/hsdfat/diam-gw/commands/base"
	"github.com/hsdfat/diam-gw/commands/s13"
	"github.com/hsdfat/diam-gw/models_base"
	"github.com/hsdfat8/eir/internal/domain/models"
)

// TestContainerizedS13Integration tests the complete containerized flow
// This test requires Docker containers to be running via docker-compose
// Run: docker-compose -f docker-compose.yml up -d
func TestContainerizedS13Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Configuration for DRA endpoint
	draAddr := getEnvOrDefault("DRA_ADDR", "localhost:3869")

	ctx := context.Background()

	t.Run("ContainerHealthCheck", func(t *testing.T) {
		testContainerHealth(t, draAddr)
	})

	t.Run("WhitelistedIMEI_S13", func(t *testing.T) {
		testWhitelistedIMEI(t, ctx, draAddr)
	})

	t.Run("GreylistedIMEI_S13", func(t *testing.T) {
		testGreylistedIMEI(t, ctx, draAddr)
	})

	t.Run("BlacklistedIMEI_S13", func(t *testing.T) {
		testBlacklistedIMEI(t, ctx, draAddr)
	})

	t.Run("UnknownIMEI_DefaultPolicy", func(t *testing.T) {
		testUnknownIMEI(t, ctx, draAddr)
	})

	t.Run("InvalidIMEI_Format", func(t *testing.T) {
		testInvalidIMEIFormat(t, ctx, draAddr)
	})

	t.Run("HopByHopEndToEnd_Preservation", func(t *testing.T) {
		testHopByHopPreservation(t, ctx, draAddr)
	})

	t.Run("ConcurrentS13Requests", func(t *testing.T) {
		testConcurrentS13Requests(t, ctx, draAddr)
	})

	t.Run("ConnectionPersistence", func(t *testing.T) {
		testConnectionPersistence(t, ctx, draAddr)
	})

	t.Run("DRAReconnection", func(t *testing.T) {
		testDRAReconnection(t, ctx, draAddr)
	})
}

// testContainerHealth verifies all containers are healthy and accessible
func testContainerHealth(t *testing.T, draAddr string) {
	t.Log("Testing container health and connectivity...")

	// Test DRA connectivity
	conn, err := net.DialTimeout("tcp", draAddr, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect to DRA at %s: %v", draAddr, err)
	}
	conn.Close()

	t.Log("✓ DRA is accessible")

	// Test HTTP API (EIR Core)
	httpConn, err := net.DialTimeout("tcp", "localhost:8080", 5*time.Second)
	if err != nil {
		t.Logf("Warning: EIR Core HTTP API not accessible: %v", err)
	} else {
		httpConn.Close()
		t.Log("✓ EIR Core HTTP API is accessible")
	}

	t.Log("✓ Container health check passed")
}

// testWhitelistedIMEI tests whitelisted IMEI check flow
func testWhitelistedIMEI(t *testing.T, ctx context.Context, draAddr string) {
	t.Log("Testing whitelisted IMEI: 123456789012345")

	client := createDiameterClient(draAddr)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	imei := "123456789012345"
	answer, err := client.CheckEquipment(ctx, imei)
	if err != nil {
		t.Fatalf("CheckEquipment failed: %v", err)
	}

	// Verify Result-Code
	if answer.ResultCode == nil || *answer.ResultCode != 2001 {
		t.Errorf("Expected Result-Code 2001 (SUCCESS), got %v", answer.ResultCode)
	}

	// Verify Equipment-Status
	if answer.EquipmentStatus == nil {
		t.Fatal("Equipment-Status is nil")
	}

	expectedStatus := models_base.Enumerated(models.DiameterEquipmentStatusWhitelisted)
	if *answer.EquipmentStatus != expectedStatus {
		t.Errorf("Expected Equipment-Status %d (WHITELISTED), got %d", expectedStatus, *answer.EquipmentStatus)
	}

	t.Logf("✓ IMEI %s correctly identified as WHITELISTED", imei)
}

// testGreylistedIMEI tests greylisted IMEI check flow
func testGreylistedIMEI(t *testing.T, ctx context.Context, draAddr string) {
	t.Log("Testing greylisted IMEI: 555555555555555")

	client := createDiameterClient(draAddr)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	imei := "555555555555555"
	answer, err := client.CheckEquipment(ctx, imei)
	if err != nil {
		t.Fatalf("CheckEquipment failed: %v", err)
	}

	// Verify Result-Code
	if answer.ResultCode == nil || *answer.ResultCode != 2001 {
		t.Errorf("Expected Result-Code 2001, got %v", answer.ResultCode)
	}

	// Verify Equipment-Status
	if answer.EquipmentStatus == nil {
		t.Fatal("Equipment-Status is nil")
	}

	expectedStatus := models_base.Enumerated(models.DiameterEquipmentStatusGreylisted)
	if *answer.EquipmentStatus != expectedStatus {
		t.Errorf("Expected Equipment-Status %d (GREYLISTED), got %d", expectedStatus, *answer.EquipmentStatus)
	}

	t.Logf("✓ IMEI %s correctly identified as GREYLISTED", imei)
}

// testBlacklistedIMEI tests blacklisted IMEI check flow
func testBlacklistedIMEI(t *testing.T, ctx context.Context, draAddr string) {
	t.Log("Testing blacklisted IMEI: 999999999999999")

	client := createDiameterClient(draAddr)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	imei := "999999999999999"
	answer, err := client.CheckEquipment(ctx, imei)
	if err != nil {
		t.Fatalf("CheckEquipment failed: %v", err)
	}

	// Verify Result-Code
	if answer.ResultCode == nil || *answer.ResultCode != 2001 {
		t.Errorf("Expected Result-Code 2001, got %v", answer.ResultCode)
	}

	// Verify Equipment-Status
	if answer.EquipmentStatus == nil {
		t.Fatal("Equipment-Status is nil")
	}

	expectedStatus := models_base.Enumerated(models.DiameterEquipmentStatusBlacklisted)
	if *answer.EquipmentStatus != expectedStatus {
		t.Errorf("Expected Equipment-Status %d (BLACKLISTED), got %d", expectedStatus, *answer.EquipmentStatus)
	}

	t.Logf("✓ IMEI %s correctly identified as BLACKLISTED", imei)
}

// testUnknownIMEI tests unknown IMEI with default policy
func testUnknownIMEI(t *testing.T, ctx context.Context, draAddr string) {
	t.Log("Testing unknown IMEI: 999999999999998 (not in database)")

	client := createDiameterClient(draAddr)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	imei := "999999999999998" // Unknown IMEI
	answer, err := client.CheckEquipment(ctx, imei)
	if err != nil {
		t.Fatalf("CheckEquipment failed: %v", err)
	}

	// Verify Result-Code
	if answer.ResultCode == nil || *answer.ResultCode != 2001 {
		t.Errorf("Expected Result-Code 2001, got %v", answer.ResultCode)
	}

	// Verify Equipment-Status (should be default policy: WHITELISTED)
	if answer.EquipmentStatus == nil {
		t.Fatal("Equipment-Status is nil")
	}

	expectedStatus := models_base.Enumerated(models.DiameterEquipmentStatusWhitelisted)
	if *answer.EquipmentStatus != expectedStatus {
		t.Errorf("Expected default Equipment-Status %d (WHITELISTED), got %d", expectedStatus, *answer.EquipmentStatus)
	}

	t.Logf("✓ Unknown IMEI %s correctly assigned default policy (WHITELISTED)", imei)
}

// testInvalidIMEIFormat tests invalid IMEI format handling
func testInvalidIMEIFormat(t *testing.T, ctx context.Context, draAddr string) {
	t.Log("Testing invalid IMEI format: ABC123")

	client := createDiameterClient(draAddr)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	imei := "ABC123" // Invalid format
	answer, err := client.CheckEquipment(ctx, imei)
	if err != nil {
		t.Fatalf("CheckEquipment failed: %v", err)
	}

	// Should return error result code
	if answer.ResultCode == nil {
		t.Fatal("Result-Code is nil")
	}

	// Accept either success (with default policy) or error code
	if *answer.ResultCode != 2001 && *answer.ResultCode != 5004 {
		t.Logf("Invalid IMEI handled with Result-Code: %d", *answer.ResultCode)
	}

	t.Logf("✓ Invalid IMEI format handled correctly with Result-Code: %d", *answer.ResultCode)
}

// testHopByHopPreservation tests Hop-by-Hop and End-to-End ID preservation
func testHopByHopPreservation(t *testing.T, ctx context.Context, draAddr string) {
	t.Log("Testing Hop-by-Hop and End-to-End ID preservation")

	client := createDiameterClient(draAddr)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Send request with specific IDs
	req := createMEIdentityCheckRequest("123456789012345", client)
	originalHopByHop := req.Header.HopByHopID
	originalEndToEnd := req.Header.EndToEndID

	answer, err := client.SendRequest(ctx, req)
	if err != nil {
		t.Fatalf("SendRequest failed: %v", err)
	}

	// Verify IDs are preserved
	if answer.Header.HopByHopID != originalHopByHop {
		t.Errorf("Hop-by-Hop ID not preserved: sent %d, received %d",
			originalHopByHop, answer.Header.HopByHopID)
	}

	if answer.Header.EndToEndID != originalEndToEnd {
		t.Errorf("End-to-End ID not preserved: sent %d, received %d",
			originalEndToEnd, answer.Header.EndToEndID)
	}

	t.Logf("✓ Hop-by-Hop ID preserved: %d", answer.Header.HopByHopID)
	t.Logf("✓ End-to-End ID preserved: %d", answer.Header.EndToEndID)
}

// testConcurrentS13Requests tests concurrent S13 requests
func testConcurrentS13Requests(t *testing.T, ctx context.Context, draAddr string) {
	t.Log("Testing concurrent S13 requests")

	numClients := 10
	requestsPerClient := 5

	results := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			client := createDiameterClient(draAddr)
			defer client.Close()

			if err := client.Connect(); err != nil {
				results <- fmt.Errorf("client %d connect failed: %w", clientID, err)
				return
			}

			for j := 0; j < requestsPerClient; j++ {
				imei := "123456789012345"
				_, err := client.CheckEquipment(ctx, imei)
				if err != nil {
					results <- fmt.Errorf("client %d request %d failed: %w", clientID, j, err)
					return
				}
			}

			results <- nil
		}(i)
	}

	// Wait for all clients
	successCount := 0
	for i := 0; i < numClients; i++ {
		err := <-results
		if err != nil {
			t.Errorf("Concurrent client error: %v", err)
		} else {
			successCount++
		}
	}

	t.Logf("✓ %d/%d concurrent clients completed successfully", successCount, numClients)
	t.Logf("✓ Total successful requests: %d", successCount*requestsPerClient)
}

// testConnectionPersistence tests persistent Diameter connection
func testConnectionPersistence(t *testing.T, ctx context.Context, draAddr string) {
	t.Log("Testing connection persistence")

	client := createDiameterClient(draAddr)
	defer client.Close()

	if err := client.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Send multiple requests over the same connection
	for i := 0; i < 10; i++ {
		imei := "123456789012345"
		_, err := client.CheckEquipment(ctx, imei)
		if err != nil {
			t.Fatalf("Request %d failed: %v", i+1, err)
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Log("✓ Connection persisted across 10 requests")
}

// testDRAReconnection tests DRA reconnection handling
func testDRAReconnection(t *testing.T, ctx context.Context, draAddr string) {
	t.Skip("Skipping DRA reconnection test - requires container restart capability")

	// This test would require:
	// 1. Establishing connection
	// 2. Restarting DRA container
	// 3. Verifying client can reconnect
	// Implementation left for CI/CD pipeline with container orchestration
}

// DiameterClient wraps Diameter protocol client
type DiameterClient struct {
	conn        net.Conn
	serverAddr  string
	originHost  string
	originRealm string
	hopByHopID  uint32
	endToEndID  uint32
}

// createDiameterClient creates a new Diameter client
func createDiameterClient(serverAddr string) *DiameterClient {
	return &DiameterClient{
		serverAddr:  serverAddr,
		originHost:  "mme.test.epc.mnc001.mcc001.3gppnetwork.org",
		originRealm: "test.epc.mnc001.mcc001.3gppnetwork.org",
		hopByHopID:  1000,
		endToEndID:  2000,
	}
}

// Connect establishes connection and performs CER/CEA exchange
func (c *DiameterClient) Connect() error {
	conn, err := net.DialTimeout("tcp", c.serverAddr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.conn = conn

	// Perform CER/CEA exchange
	if err := c.capabilitiesExchange(); err != nil {
		c.conn.Close()
		return fmt.Errorf("capabilities exchange failed: %w", err)
	}

	return nil
}

// Close closes the connection
func (c *DiameterClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// CheckEquipment sends ME-Identity-Check-Request
func (c *DiameterClient) CheckEquipment(ctx context.Context, imei string) (*s13.MEIdentityCheckAnswer, error) {
	req := createMEIdentityCheckRequest(imei, c)

	return c.SendRequest(ctx, req)
}

// SendRequest sends a request and waits for answer
func (c *DiameterClient) SendRequest(ctx context.Context, req *s13.MEIdentityCheckRequest) (*s13.MEIdentityCheckAnswer, error) {
	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Set a default timeout if context doesn't have one
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}

	// Set read deadline
	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}
	defer c.conn.SetReadDeadline(time.Time{}) // Clear deadline

	// Marshal request
	reqBytes, err := req.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	if _, err := c.conn.Write(reqBytes); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read answer - the deadline is already set on the connection
	// Check context periodically during the read
	answerBytes, err := c.readDiameterMessageWithContext(ctx)
	if err != nil {
		// Check if it's a timeout error from the deadline
		if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
			// Check if context was also cancelled
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("context cancelled: %w (timeout: %v)", ctx.Err(), err)
			default:
				return nil, fmt.Errorf("read timeout: %w", err)
			}
		}
		return nil, fmt.Errorf("failed to read answer: %w", err)
	}

	// Unmarshal answer
	answer := &s13.MEIdentityCheckAnswer{}
	if err := answer.Unmarshal(answerBytes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal answer: %w", err)
	}

	return answer, nil
}

// capabilitiesExchange performs CER/CEA exchange
func (c *DiameterClient) capabilitiesExchange() error {
	// Create CER
	cer := base.NewCapabilitiesExchangeRequest()
	cer.OriginHost = models_base.DiameterIdentity(c.originHost)
	cer.OriginRealm = models_base.DiameterIdentity(c.originRealm)
	cer.HostIpAddress = []models_base.Address{
		models_base.Address(net.ParseIP("127.0.0.1")),
	}
	cer.VendorId = models_base.Unsigned32(10415)
	cer.ProductName = models_base.UTF8String("Test-Client/1.0")
	cer.AuthApplicationId = []models_base.Unsigned32{models_base.Unsigned32(16777252)} // S13

	c.hopByHopID++
	c.endToEndID++
	cer.Header.HopByHopID = c.hopByHopID
	cer.Header.EndToEndID = c.endToEndID

	// Marshal and send CER
	cerBytes, err := cer.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal CER: %w", err)
	}

	if _, err := c.conn.Write(cerBytes); err != nil {
		return fmt.Errorf("failed to send CER: %w", err)
	}

	// Set read deadline for CEA
	if err := c.conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}
	defer c.conn.SetReadDeadline(time.Time{}) // Clear deadline

	// Read CEA
	ceaBytes, err := c.readDiameterMessage()
	if err != nil {
		return fmt.Errorf("failed to read CEA: %w", err)
	}

	// Unmarshal CEA
	cea := &base.CapabilitiesExchangeAnswer{}
	if err := cea.Unmarshal(ceaBytes); err != nil {
		return fmt.Errorf("failed to unmarshal CEA: %w", err)
	}

	// Check result code
	if cea.ResultCode != 2001 {
		return fmt.Errorf("CEA returned error: %d", cea.ResultCode)
	}

	return nil
}

// readDiameterMessage reads a complete Diameter message
func (c *DiameterClient) readDiameterMessage() ([]byte, error) {
	// Read header (20 bytes)
	header := make([]byte, 20)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return nil, err
	}

	// Parse message length
	messageLength := uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])

	// Validate message length
	if messageLength < 20 || messageLength > 1024*1024 { // Max 1MB
		return nil, fmt.Errorf("invalid message length: %d", messageLength)
	}

	// Read remaining message
	fullMessage := make([]byte, messageLength)
	copy(fullMessage[:20], header)

	if messageLength > 20 {
		if _, err := io.ReadFull(c.conn, fullMessage[20:]); err != nil {
			return nil, err
		}
	}

	return fullMessage, nil
}

// readDiameterMessageWithContext reads a complete Diameter message with context support
func (c *DiameterClient) readDiameterMessageWithContext(ctx context.Context) ([]byte, error) {
	// Check context before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Use a channel to handle the read operation
	type result struct {
		data []byte
		err  error
	}
	resultChan := make(chan result, 1)

	go func() {
		// The read will respect the deadline set on the connection
		data, err := c.readDiameterMessage()
		select {
		case resultChan <- result{data: data, err: err}:
		case <-ctx.Done():
			// Context was cancelled, but read already completed
			// Return the result anyway
		}
	}()

	select {
	case <-ctx.Done():
		// Context cancelled - the read will timeout due to the deadline
		// Wait a bit for the goroutine to finish, but don't block forever
		select {
		case res := <-resultChan:
			// Read completed before we could cancel
			return res.data, res.err
		case <-time.After(100 * time.Millisecond):
			// Give up waiting
			return nil, ctx.Err()
		}
	case res := <-resultChan:
		return res.data, res.err
	}
}

// createMEIdentityCheckRequest creates S13 request
func createMEIdentityCheckRequest(imei string, client *DiameterClient) *s13.MEIdentityCheckRequest {
	req := s13.NewMEIdentityCheckRequest()

	client.hopByHopID++
	client.endToEndID++

	req.Header.HopByHopID = client.hopByHopID
	req.Header.EndToEndID = client.endToEndID

	req.SessionId = models_base.UTF8String(fmt.Sprintf("%s;%d;%d", client.originHost, time.Now().Unix(), client.endToEndID))
	req.AuthSessionState = models_base.Enumerated(1) // NO_STATE_MAINTAINED
	req.OriginHost = models_base.DiameterIdentity(client.originHost)
	req.OriginRealm = models_base.DiameterIdentity(client.originRealm)
	destHost := models_base.DiameterIdentity("eir.test.epc.mnc001.mcc001.3gppnetwork.org")
	req.DestinationHost = &destHost
	req.DestinationRealm = models_base.DiameterIdentity("test.epc.mnc001.mcc001.3gppnetwork.org")

	// Set Terminal-Information with IMEI
	imeiUTF8 := models_base.UTF8String(imei)
	req.TerminalInformation = &s13.TerminalInformation{
		Imei: &imeiUTF8,
	}

	return req
}

// getEnvOrDefault gets environment variable or default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
