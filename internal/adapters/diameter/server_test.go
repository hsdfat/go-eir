package diameter

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/hsdfat/diam-gw/commands/base"
	"github.com/hsdfat/diam-gw/commands/s13"
	"github.com/hsdfat/diam-gw/models_base"
	"github.com/hsdfat8/eir/internal/adapters/testutil"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
)

// mockEIRService is a mock implementation of EIRService for testing
type mockEIRService struct{}

func (m *mockEIRService) CheckImei(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckImeiResult, error) {
	// Return whitelisted status for test IMEI
	return &ports.CheckImeiResult{
		Status: "ok",
		IMEI:   imei,
		Color:  "w", // white = whitelisted
	}, nil
}

func (m *mockEIRService) CheckTac(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckTacResult, error) {
	// Return whitelisted status for test IMEI
	return &ports.CheckTacResult{
		Status: "ok",
		IMEI:   imei,
		Color:  "white", // whitelisted
	}, nil
}

func (m *mockEIRService) InsertImei(ctx context.Context, imei string, color string, status models.SystemStatus) (*ports.InsertImeiResult, error) {
	return &ports.InsertImeiResult{
		Status: "ok",
		IMEI:   imei,
		Error:  nil,
	}, nil
}

func (m *mockEIRService) InsertTac(ctx context.Context, tacInfo *ports.TacInfo) (*ports.InsertTacResult, error) {
	return &ports.InsertTacResult{
		Status:  "ok",
		Error:   nil,
		TacInfo: tacInfo,
	}, nil
}

func (m *mockEIRService) RemoveEquipment(ctx context.Context, imei string) error {
	return nil
}

func (m *mockEIRService) GetEquipment(ctx context.Context, imei string) (*models.Equipment, error) {
	return &models.Equipment{
		IMEI:   imei,
		Status: models.EquipmentStatusWhitelisted,
	}, nil
}

func (m *mockEIRService) ListEquipment(ctx context.Context, offset, limit int) ([]*models.Equipment, error) {
	return []*models.Equipment{}, nil
}

func (m *mockEIRService) SetLogger(l logger.Logger) {
	// Mock implementation - no-op for testing
}

// TestServerBasicSetup tests basic server creation and startup
func TestServerBasicSetup(t *testing.T) {
	config := ServerConfig{
		ListenAddr:  "127.0.0.1:3868", // Standard Diameter port
		OriginHost:  "eir-test.example.com",
		OriginRealm: "example.com",
		ProductName: "EIR-Test/1.0",
		VendorID:    10415,
	}

	mockService := &mockEIRService{}
	server := NewServer(config, mockService)

	if server == nil {
		t.Fatal("Failed to create server")
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	if server.diamServer.GetListener() == nil {
		t.Fatal("Server listener is nil")
	}

	t.Logf("Server started successfully on %s", server.diamServer.GetListener().Addr().String())

	// Stop server
	if err := server.Stop(); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	t.Log("Server stopped successfully")
}

// TestServerS13MEIdentityCheck tests S13 ME-Identity-Check-Request/Answer with PCAP capture
func TestServerS13MEIdentityCheck(t *testing.T) {
	// Create PCAP writer in the same directory as the test file
	pcapFile := "diameter_s13_3868_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	config := ServerConfig{
		ListenAddr:  "127.0.0.1:3868", // Standard Diameter port
		OriginHost:  "eir-test.example.com",
		OriginRealm: "example.com",
		ProductName: "EIR-Test/1.0",
		VendorID:    10415,
	}

	mockService := &mockEIRService{}
	server := NewServer(config, mockService)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	addr := "127.0.0.1:3868"
	t.Logf("Server listening on %s", addr)

	// Create test client with PCAP capture
	client := newTestClientWithPCAP(t, addr, pcapWriter)
	defer client.Close()

	// Send CER first
	client.sendCERWithS13(t)
	client.receiveCEA(t)

	t.Log("CER/CEA exchange completed")

	// Build ME-Identity-Check-Request (S13 ECR)
	ecr := s13.NewMEIdentityCheckRequest()
	ecr.SessionId = models_base.UTF8String("eir-test.example.com;1234567890;1")
	ecr.AuthSessionState = models_base.Enumerated(1) // NO_STATE_MAINTAINED
	ecr.OriginHost = models_base.DiameterIdentity("test-client.example.com")
	ecr.OriginRealm = models_base.DiameterIdentity("example.com")
	ecr.DestinationRealm = models_base.DiameterIdentity("example.com")
	ecr.TerminalInformation = &s13.TerminalInformation{
		Imei:            ptrUTF8String("123456789012345"),
		SoftwareVersion: ptrUTF8String("01"),
	}

	ecrBytes, err := ecr.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal ECR: %v", err)
	}

	// Send ECR
	if _, err := client.conn.Write(ecrBytes); err != nil {
		t.Fatalf("Failed to send ECR: %v", err)
	}

	t.Log("Sent ME-Identity-Check-Request")

	// Read ECA (ME-Identity-Check-Answer)
	ecaBytes := client.readMessage(t)
	if len(ecaBytes) == 0 {
		t.Fatal("Received empty ECA")
	}

	eca := &s13.MEIdentityCheckAnswer{}
	if err := eca.Unmarshal(ecaBytes); err != nil {
		t.Fatalf("Failed to unmarshal ECA: %v", err)
	}

	t.Logf("Received ME-Identity-Check-Answer: ResultCode=%d, EquipmentStatus=%d",
		*eca.ResultCode, *eca.EquipmentStatus)

	// Verify ECA
	if *eca.ResultCode != 2001 {
		t.Errorf("Expected ResultCode 2001 (DIAMETER_SUCCESS), got %d", *eca.ResultCode)
	}

	if eca.Header.HopByHopID != ecr.Header.HopByHopID {
		t.Errorf("HopByHopID mismatch: sent=%d, received=%d",
			ecr.Header.HopByHopID, eca.Header.HopByHopID)
	}

	if eca.Header.EndToEndID != ecr.Header.EndToEndID {
		t.Errorf("EndToEndID mismatch: sent=%d, received=%d",
			ecr.Header.EndToEndID, eca.Header.EndToEndID)
	}

	if eca.EquipmentStatus == nil {
		t.Error("EquipmentStatus is nil")
	} else {
		// Equipment status 0 = Whitelisted
		expectedStatus := models_base.Enumerated(0)
		if *eca.EquipmentStatus != expectedStatus {
			t.Errorf("Expected EquipmentStatus %d, got %d", expectedStatus, *eca.EquipmentStatus)
		}
	}

	t.Log("S13 ME-Identity-Check test passed")
	t.Logf("PCAP file saved: %s", pcapFile)
}

// TestServerS13MultipleRequests tests multiple S13 requests with PCAP capture
func TestServerS13MultipleRequests(t *testing.T) {
	// Create PCAP writer in the same directory as the test file
	pcapFile := "diameter_s13_multiple_3868_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	config := ServerConfig{
		ListenAddr:  "127.0.0.1:3868", // Standard Diameter port
		OriginHost:  "eir-test.example.com",
		OriginRealm: "example.com",
		ProductName: "EIR-Test/1.0",
		VendorID:    10415,
	}

	mockService := &mockEIRService{}
	server := NewServer(config, mockService)

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	addr := "127.0.0.1:3868"

	// Create test client with PCAP capture
	client := newTestClientWithPCAP(t, addr, pcapWriter)
	defer client.Close()

	// Send CER first
	client.sendCERWithS13(t)
	client.receiveCEA(t)

	// Send multiple ECR messages
	numRequests := 10
	startTime := time.Now()

	for i := 0; i < numRequests; i++ {
		ecr := s13.NewMEIdentityCheckRequest()
		ecr.SessionId = models_base.UTF8String(fmt.Sprintf("eir-test.example.com;%d;1", i))
		ecr.AuthSessionState = models_base.Enumerated(1)
		ecr.OriginHost = models_base.DiameterIdentity("test-client.example.com")
		ecr.OriginRealm = models_base.DiameterIdentity("example.com")
		ecr.DestinationRealm = models_base.DiameterIdentity("example.com")
		ecr.TerminalInformation = &s13.TerminalInformation{
			Imei:            ptrUTF8String(fmt.Sprintf("12345678901234%d", i)),
			SoftwareVersion: ptrUTF8String("01"),
		}

		ecrBytes, err := ecr.Marshal()
		if err != nil {
			t.Fatalf("Failed to marshal ECR: %v", err)
		}

		if _, err := client.conn.Write(ecrBytes); err != nil {
			t.Fatalf("Failed to send ECR: %v", err)
		}

		// Read ECA
		ecaBytes := client.readMessage(t)
		eca := &s13.MEIdentityCheckAnswer{}
		if err := eca.Unmarshal(ecaBytes); err != nil {
			t.Fatalf("Failed to unmarshal ECA: %v", err)
		}

		if *eca.ResultCode != 2001 {
			t.Errorf("Request %d: Expected ResultCode 2001, got %d", i, *eca.ResultCode)
		}
	}

	duration := time.Since(startTime)
	throughput := float64(numRequests) / duration.Seconds()

	t.Logf("Processed %d requests in %v (%.2f req/sec)",
		numRequests, duration, throughput)

	// Get server stats
	stats := server.diamServer.GetStats()
	t.Logf("Server stats: TotalConn=%d, TotalMsg=%d, Recv=%d, Sent=%d, Errors=%d",
		stats.TotalConnections, stats.TotalMessages,
		stats.MessagesReceived, stats.MessagesSent, stats.Errors)

	if stats.Errors > 0 {
		t.Errorf("Expected no errors, got %d", stats.Errors)
	}

	t.Logf("PCAP file saved: %s", pcapFile)
}

// testClient is a helper struct for test clients
type testClient struct {
	conn       net.Conn
	t          *testing.T
	pcapWriter *testutil.PCAPWriter
}

func newTestClient(t *testing.T, addr string) *testClient {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	return &testClient{conn: conn, t: t}
}

func newTestClientWithPCAP(t *testing.T, addr string, pcapWriter *testutil.PCAPWriter) *testClient {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	// Wrap connection with PCAP capture
	captureConn := testutil.NewCaptureConnection(conn, pcapWriter)
	return &testClient{conn: captureConn, t: t, pcapWriter: pcapWriter}
}

func (c *testClient) Close() {
	c.conn.Close()
}

func (c *testClient) sendCERWithS13(t *testing.T) {
	cer := base.NewCapabilitiesExchangeRequest()
	cer.OriginHost = models_base.DiameterIdentity("test-client.example.com")
	cer.OriginRealm = models_base.DiameterIdentity("example.com")
	cer.HostIpAddress = []models_base.Address{
		models_base.Address(net.ParseIP("127.0.0.1")),
	}
	cer.VendorId = models_base.Unsigned32(10415)
	cer.ProductName = models_base.UTF8String("TestClient/1.0")

	// Add S13 support (Application ID 16777252)
	cer.AuthApplicationId = []models_base.Unsigned32{16777252}

	cerBytes, err := cer.Marshal()
	if err != nil {
		t.Fatalf("Failed to marshal CER: %v", err)
	}

	if _, err := c.conn.Write(cerBytes); err != nil {
		t.Fatalf("Failed to send CER: %v", err)
	}
}

func (c *testClient) receiveCEA(t *testing.T) {
	ceaBytes := c.readMessage(t)
	if len(ceaBytes) == 0 {
		t.Fatal("Received empty CEA")
	}

	cea := &base.CapabilitiesExchangeAnswer{}
	if err := cea.Unmarshal(ceaBytes); err != nil {
		t.Fatalf("Failed to unmarshal CEA: %v", err)
	}

	if cea.ResultCode != 2001 {
		t.Errorf("Expected CEA ResultCode 2001, got %d", cea.ResultCode)
	}
}

func (c *testClient) readMessage(t *testing.T) []byte {
	// Set read timeout
	c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read header (20 bytes)
	header := make([]byte, 20)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		t.Fatalf("Failed to read message header: %v", err)
	}

	// Parse message length
	msgLen := uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])

	// Read body
	body := make([]byte, msgLen-20)
	if len(body) > 0 {
		if _, err := io.ReadFull(c.conn, body); err != nil {
			t.Fatalf("Failed to read message body: %v", err)
		}
	}

	return append(header, body...)
}

// Helper function to create pointer to UTF8String
func ptrUTF8String(s string) *models_base.UTF8String {
	v := models_base.UTF8String(s)
	return &v
}
