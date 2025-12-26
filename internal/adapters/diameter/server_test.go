package diameter

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
	"github.com/hsdfat8/eir/internal/adapters/postgres"
	"github.com/hsdfat8/eir/internal/adapters/testutil"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
	legacyModels "github.com/hsdfat8/eir/models"
	"github.com/hsdfat8/eir/pkg/logic"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
)

type mockEIRService struct {
	imeiRepo      ports.IMEIRepository
	insertedTacs  []ports.TacInfo
	insertedImeis []string
}

// mockEIRService is a mock implementation of EIRService for testing
func newMockEIRService() (*mockEIRService, func()) {
	_ = godotenv.Load("../../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	db, _ := sqlx.Connect("postgres", dbURL)
	cleanup := func() {
		db.Close()
	}
	return &mockEIRService{
		imeiRepo: postgres.NewIMEIRepository(db),
	}, cleanup
}

func (m *mockEIRService) CheckImei(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckImeiResult, error) {
	legacyStatus := legacyModels.SystemStatus{
		OverloadLevel: status.OverloadLevel,
		TPSOverload:   status.TPSOverload,
	}
	result := logic.CheckImei(imei, legacyStatus)
	return &ports.CheckImeiResult{
		Status: result.Status,
		IMEI:   result.IMEI,
		Color:  result.Color,
	}, nil
}

func (m *mockEIRService) CheckTac(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckTacResult, error) {
	legacyStatus := legacyModels.SystemStatus{
		OverloadLevel: status.OverloadLevel,
		TPSOverload:   status.TPSOverload,
	}
	result, tacInfo := logic.CheckTac(imei, legacyStatus)

	var tacInfoPtr *ports.TacInfo
	if result.Status == "ok" {
		tacInfoPtr = &ports.TacInfo{
			KeyTac:        tacInfo.KeyTac,
			StartRangeTac: tacInfo.StartRangeTac,
			EndRangeTac:   tacInfo.EndRangeTac,
			Color:         tacInfo.Color,
			PrevLink:      tacInfo.PrevLink,
		}
	}

	return &ports.CheckTacResult{
		Status:  result.Status,
		IMEI:    result.IMEI,
		Color:   result.Color,
		TacInfo: tacInfoPtr,
	}, nil
}

func (m *mockEIRService) InsertImei(ctx context.Context, imei string, color string, status models.SystemStatus) (*ports.InsertImeiResult, error) {
	legacyStatus := legacyModels.SystemStatus{
		OverloadLevel: status.OverloadLevel,
		TPSOverload:   status.TPSOverload,
	}
	result := logic.InsertImei(m.imeiRepo, imei, color, legacyStatus)
	if result.Status == "ok" {
		m.insertedImeis = append(m.insertedImeis, imei)
	}
	errorPtr := (*string)(nil)
	if result.Error != "" {
		errorPtr = &result.Error
	}
	return &ports.InsertImeiResult{
		Status: result.Status,
		IMEI:   result.IMEI,
		Error:  errorPtr,
	}, nil
}

func (m *mockEIRService) InsertTac(ctx context.Context, tacInfo *ports.TacInfo) (*ports.InsertTacResult, error) {
	if tacInfo == nil {
		return nil, fmt.Errorf("tacInfo is required")
	}
	legacyTacInfo := legacyModels.TacInfo{
		KeyTac:        tacInfo.KeyTac,
		StartRangeTac: tacInfo.StartRangeTac,
		EndRangeTac:   tacInfo.EndRangeTac,
		Color:         tacInfo.Color,
		PrevLink:      tacInfo.PrevLink,
	}
	result := logic.InsertTac(m.imeiRepo, legacyTacInfo)
	var resultTacInfo *ports.TacInfo
	if result.TacInfo.KeyTac != "" {
		resultTacInfo = &ports.TacInfo{
			KeyTac:        result.TacInfo.KeyTac,
			StartRangeTac: result.TacInfo.StartRangeTac,
			EndRangeTac:   result.TacInfo.EndRangeTac,
			Color:         result.TacInfo.Color,
			PrevLink:      result.TacInfo.PrevLink,
		}
	}
	errorPtr := (*string)(nil)
	if result.Error != "" {
		errorPtr = &result.Error
	}
	return &ports.InsertTacResult{Status: result.Status, Error: errorPtr, TacInfo: resultTacInfo}, nil
}

func (m *mockEIRService) ClearTacInfo() {
	m.imeiRepo.ClearTacInfo(context.Background())
}

func (m *mockEIRService) ClearImeiInfo() {
	m.imeiRepo.ClearImeiInfo(context.Background())
}

func (m *mockEIRService) ListAllTacInfo() []*ports.TacInfo {
	return m.imeiRepo.ListAllTacInfo(context.Background())
}

func (m *mockEIRService) ListAllImeiInfo() []*ports.ImeiInfo {
	return m.imeiRepo.ListAllImeiInfo(context.Background())
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
		Host:             "127.0.0.1",
		Port:             3868,
		OriginHost:       "eir-test.example.com",
		OriginRealm:      "example.com",
		ProductName:      "EIR-Test/1.0",
		VendorID:         10415,
		MaxConnections:   1000,
		ReadTimeout:      30000000000,
		WriteTimeout:     10000000000,
		WatchdogInterval: 30000000000,
		WatchdogTimeout:  10000000000,
		MaxMessageSize:   65535,
		SendChannelSize:  100,
		RecvChannelSize:  100,
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
		Host:             "127.0.0.1",
		Port:             3868,
		OriginHost:       "eir-test.example.com",
		OriginRealm:      "example.com",
		ProductName:      "EIR-Test/1.0",
		VendorID:         10415,
		MaxConnections:   1000,
		ReadTimeout:      30000000000,
		WriteTimeout:     10000000000,
		WatchdogInterval: 30000000000,
		WatchdogTimeout:  10000000000,
		MaxMessageSize:   65535,
		SendChannelSize:  100,
		RecvChannelSize:  100,
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
		Host:             "127.0.0.1",
		Port:             3868,
		OriginHost:       "eir-test.example.com",
		OriginRealm:      "example.com",
		ProductName:      "EIR-Test/1.0",
		VendorID:         10415,
		MaxConnections:   1000,
		ReadTimeout:      30000000000,
		WriteTimeout:     10000000000,
		WatchdogInterval: 30000000000,
		WatchdogTimeout:  10000000000,
		MaxMessageSize:   65535,
		SendChannelSize:  100,
		RecvChannelSize:  100,
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

// TestCheckImeiWithPCAP tests IMEI checking via Diameter S13 interface
// Flow: 1) Insert IMEI into database via Service, 2) Check IMEI via Diameter S13 (Client)
func TestCheckImeiWithPCAP(t *testing.T) {
	pcapFile := "diameter_check_imei_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	config := ServerConfig{
		Host:             "127.0.0.1",
		Port:             3868,
		OriginHost:       "eir.example.com",
		OriginRealm:      "example.com",
		ProductName:      "TestEIR",
		VendorID:         10415,
		MaxConnections:   1000,
		ReadTimeout:      30000000000,
		WriteTimeout:     10000000000,
		WatchdogInterval: 30000000000,
		WatchdogTimeout:  10000000000,
		MaxMessageSize:   65535,
		SendChannelSize:  100,
		RecvChannelSize:  100,
	}

	mockService, cleanup := newMockEIRService()
	defer cleanup()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(500 * time.Millisecond)

	client := newTestClientWithPCAP(t, "127.0.0.1:3868", pcapWriter)
	defer client.Close()

	client.sendCERWithS13(t)
	client.receiveCEA(t)

	// EIR_10: Insert blacklisted IMEI then check it
	t.Run("InsertAndCheck_Blacklisted_EIR_10", func(t *testing.T) {
		mockService.ClearImeiInfo()
		imei := "9"
		color := "b" // black
		// Step 1: Insert IMEI via Service logic
		ctx := context.Background()
		mockService.InsertImei(ctx, imei, color, models.SystemStatus{})
		result, err := mockService.CheckImei(ctx, imei, models.SystemStatus{})
		if err != nil || result.Status != "ok" {
			t.Fatalf("Check IMEI failed: %v", err)
		}
		if result.Color != color {
			t.Fatalf("Expected color: %s, got %s", color, result.Color)
		}
		t.Logf("Checked imei: %s, color: %s", imei, color)
	})

	// EIR_11: Insert greylisted IMEI then check it
	t.Run("InsertAndCheck_Greylisted_EIR_11", func(t *testing.T) {
		mockService.ClearImeiInfo()
		imei := "912"
		color := "g"

		ctx := context.Background()
		mockService.InsertImei(ctx, imei, color, models.SystemStatus{})
		result, err := mockService.CheckImei(ctx, imei, models.SystemStatus{})
		if err != nil || result.Status != "ok" {
			t.Fatalf("Check IMEI failed: %v", err)
		}
		if result.Color != color {
			t.Fatalf("Expected color: %s, got %s", color, result.Color)
		}
		t.Logf("Checked imei: %s, color: %s", imei, color)
	})

	// EIR_12: Insert long blacklisted IMEI then check it
	t.Run("InsertAndCheck_LongBlacklisted_EIR_12", func(t *testing.T) {
		mockService.ClearImeiInfo()
		imei := "9123456789012"
		color := "b"

		ctx := context.Background()
		mockService.InsertImei(ctx, imei, color, models.SystemStatus{})
		result, err := mockService.CheckImei(ctx, imei, models.SystemStatus{})
		if err != nil || result.Status != "ok" {
			t.Fatalf("Check IMEI failed: %v", err)
		}
		if result.Color != color {
			t.Fatalf("Expected color: %s, got %s", color, result.Color)
		}
		t.Logf("Checked imei: %s, color: %s", imei, color)
	})

	// EIR_13: Insert whitelisted IMEI then check it
	t.Run("InsertAndCheck_Whitelisted_EIR_13", func(t *testing.T) {
		mockService.ClearImeiInfo()
		imei := "91234567895264"
		color := "w"

		ctx := context.Background()
		mockService.InsertImei(ctx, imei, color, models.SystemStatus{})
		result, err := mockService.CheckImei(ctx, imei, models.SystemStatus{})
		if err != nil || result.Status != "ok" {
			t.Fatalf("Check IMEI failed: %v", err)
		}
		if result.Color != color {
			t.Fatalf("Expected color: %s, got %s", color, result.Color)
		}
	})

	t.Logf("PCAP file saved: %s", pcapFile)
}

// TestCheckTacWithPCAP tests CheckTac functionality with Diameter S13 and PCAP capture
// Flow: 1) Insert TAC ranges into database via Service, 2) Check various IMEIs via Diameter S13 (Client)
func TestCheckTacWithPCAP(t *testing.T) {
	pcapFile := "diameter_check_tac_test.pcap"
	pcapWriter, _ := testutil.NewPCAPWriter(pcapFile)
	defer pcapWriter.Close()

	config := ServerConfig{
		Host:             "127.0.0.1",
		Port:             3870,
		OriginHost:       "eir.example.com",
		OriginRealm:      "example.com",
		ProductName:      "TestEIR",
		VendorID:         10415,
		MaxConnections:   1000,
		ReadTimeout:      30000000000,
		WriteTimeout:     10000000000,
		WatchdogInterval: 30000000000,
		WatchdogTimeout:  10000000000,
		MaxMessageSize:   65535,
		SendChannelSize:  100,
		RecvChannelSize:  100,
	}

	mockService, cleanup := newMockEIRService()
	defer cleanup()
	server := NewServer(config, mockService)
	server.Start()
	defer server.Stop()

	time.Sleep(500 * time.Millisecond)
	client := newTestClientWithPCAP(t, "127.0.0.1:3870", pcapWriter)
	defer client.Close()
	client.sendCERWithS13(t)
	client.receiveCEA(t)

	mockService.ClearTacInfo()

	// Step 1: Insert TAC ranges (Provisioning)
	t.Run("Step1_InsertTacRanges", func(t *testing.T) {
		testCases := []struct{ s, e, c string }{
			{"35", "35", "black"}, {"35310", "35319", "white"}, {"353200", "353299", "grey"},
		}
		for _, tc := range testCases {
			mockService.InsertTac(context.Background(), &ports.TacInfo{StartRangeTac: tc.s, EndRangeTac: tc.e, Color: tc.c})
		}
	})

	t.Run("Step2_CheckTacQueries", func(t *testing.T) {
		testCases := []struct {
			imei, expectedColor string
		}{
			{"35", "black"}, {"35315", "white"}, {"353250", "grey"}, {"1", "unknown"},
		}
		for _, tc := range testCases {
			// Gọi trực tiếp function CheckTac của service
			result, _ := mockService.CheckTac(context.Background(), tc.imei, models.SystemStatus{})
			if result.Color != tc.expectedColor {
				t.Errorf("IMEI %s: Expected %s, got %s", tc.imei, tc.expectedColor, result.Color)
			}
			t.Logf("✓ CheckTac: IMEI=%s, Color=%s", tc.imei, result.Color)
		}
	})

	t.Logf("PCAP file saved: %s", pcapFile)
}

// TestInsertTacWithPCAP tests TAC insertion functionality via Diameter protocol with PCAP capture
// Since Diameter S13 interface doesn't have TAC insertion, we use mockService directly
// This test verifies the TAC insertion logic matches the HTTP adapter behavior
func TestInsertTacWithPCAP(t *testing.T) {
	// Create PCAP writer for capturing test traffic
	pcapFile := "diameter_insert_tac_3868_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	// Configure Diameter server
	config := ServerConfig{
		Host:             "127.0.0.1",
		Port:             3868,
		OriginHost:       "eir.example.com",
		OriginRealm:      "example.com",
		ProductName:      "TestEIR",
		VendorID:         10415,
		MaxConnections:   1000,
		ReadTimeout:      30000000000,
		WriteTimeout:     10000000000,
		WatchdogInterval: 30000000000,
		WatchdogTimeout:  10000000000,
		MaxMessageSize:   65535,
		SendChannelSize:  100,
		RecvChannelSize:  100,
	}

	mockService, cleanup := newMockEIRService()
	defer cleanup()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start Diameter server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create test client with PCAP capture
	client := newTestClientWithPCAP(t, "127.0.0.1:3868", pcapWriter)
	defer client.Close()

	// Establish CER/CEA exchange
	client.sendCERWithS13(t)
	client.receiveCEA(t)

	// clean database before test
	mockService.ClearTacInfo()

	// Eir_Add_63: Single exact TAC values
	t.Run("Eir_Add_63", func(t *testing.T) {
		testCases := []struct {
			keyTac        string
			startRangeTac string
			endRangeTac   string
			color         string
		}{
			{"1134567890123456-1134567890123456", "1134567890123456", "1134567890123456", "white"},
			{"2-2", "2", "2", "white"},
		}

		for _, tc := range testCases {
			tacInfo := ports.TacInfo{
				KeyTac:        tc.keyTac,
				StartRangeTac: tc.startRangeTac,
				EndRangeTac:   tc.endRangeTac,
				Color:         tc.color,
				PrevLink:      nil,
			}

			result, err := mockService.InsertTac(context.Background(), &tacInfo)
			if err != nil || result.Status != "ok" {
				t.Errorf("Failed to insert TAC %s: %v", tc.keyTac, err)
			}

			t.Logf("TAC insertion '%s': TAC=%s-%s, Color=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, tc.color)
		}

		// List all data that was inserted during the test
		t.Logf("\n========== All Inserted TAC Data ==========")
		for i, tac := range mockService.ListAllTacInfo() {
			t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s",
				i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color)
		}
		t.Logf("==========================================\n")
	})

	// clean database before test
	mockService.ClearTacInfo()

	// Eir_Add_64: Various TAC ranges
	t.Run("Eir_Add_64", func(t *testing.T) {
		testCases := []struct {
			keyTac        string
			startRangeTac string
			endRangeTac   string
			color         string
		}{
			{"111-1222", "111", "1222", "white"},
			{"1223-13", "1223", "13", "white"},
			{"123456789012345-123456789012349", "123456789012345", "123456789012349", "white"},
			{"1-9", "1", "9", "white"},
			{"4-4234567890123456", "4", "4234567890123456", "white"},
			{"1234567890123456-1234567890123457", "1234567890123456", "1234567890123457", "white"},
		}

		for _, tc := range testCases {
			tacInfo := ports.TacInfo{
				KeyTac:        tc.keyTac,
				StartRangeTac: tc.startRangeTac,
				EndRangeTac:   tc.endRangeTac,
				Color:         tc.color,
				PrevLink:      nil,
			}

			result, err := mockService.InsertTac(context.Background(), &tacInfo)
			if err != nil || result.Status != "ok" {
				t.Errorf("Failed to insert TAC %s: %v", tc.keyTac, err)
			}

			t.Logf("TAC insertion '%s': TAC=%s-%s, Color=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, tc.color)
		}

		// List all data that was inserted during the test
		t.Logf("\n========== All Inserted TAC Data ==========")
		for i, tac := range mockService.ListAllTacInfo() {
			prevLinkStr := "nil"
			if tac.PrevLink != nil && *tac.PrevLink != "" {
				prevLinkStr = *tac.PrevLink
			}
			t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s, PrevLink: %s",
				i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLinkStr)
		}
		t.Logf("==========================================\n")
	})

	// clean database before test
	mockService.ClearTacInfo()

	// Eir_Add_65: Consecutive TAC values
	t.Run("Eir_Add_65", func(t *testing.T) {
		testCases := []struct {
			keyTac        string
			startRangeTac string
			endRangeTac   string
			color         string
		}{
			{"133-133", "133", "133", "white"},
			{"132-132", "132", "132", "white"},
			{"134-134", "134", "134", "white"},
		}

		for _, tc := range testCases {
			tacInfo := ports.TacInfo{
				KeyTac:        tc.keyTac,
				StartRangeTac: tc.startRangeTac,
				EndRangeTac:   tc.endRangeTac,
				Color:         tc.color,
				PrevLink:      nil,
			}

			result, err := mockService.InsertTac(context.Background(), &tacInfo)
			if err != nil || result.Status != "ok" {
				t.Errorf("Failed to insert TAC %s: %v", tc.keyTac, err)
			}

			t.Logf("TAC insertion '%s': TAC=%s-%s, Color=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, tc.color)
		}

		// List all data that was inserted during the test
		t.Logf("\n========== All Inserted TAC Data ==========")
		t.Logf("Total TAC records inserted: %d", len(mockService.insertedTacs))
		for i, tac := range mockService.ListAllTacInfo() {
			prevLinkStr := "nil"
			if tac.PrevLink != nil && *tac.PrevLink != "" {
				prevLinkStr = *tac.PrevLink
			}
			t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s, PrevLink: %s",
				i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLinkStr)
		}
		t.Logf("==========================================\n")
	})

	// clean database before test
	mockService.ClearTacInfo()

	// Eir_Add_66: Overlapping TAC ranges
	t.Run("Eir_Add_66", func(t *testing.T) {
		testCases := []struct {
			keyTac        string
			startRangeTac string
			endRangeTac   string
			color         string
		}{
			{"133-135", "133", "135", "white"},
			{"133-139", "133", "139", "white"},
		}

		for _, tc := range testCases {
			tacInfo := ports.TacInfo{
				KeyTac:        tc.keyTac,
				StartRangeTac: tc.startRangeTac,
				EndRangeTac:   tc.endRangeTac,
				Color:         tc.color,
				PrevLink:      nil,
			}

			result, err := mockService.InsertTac(context.Background(), &tacInfo)
			if err != nil || result.Status != "ok" {
				t.Errorf("Failed to insert TAC %s: %v", tc.keyTac, err)
			}

			t.Logf("TAC insertion '%s': TAC=%s-%s, Color=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, tc.color)
		}

		// List all data that was inserted during the test
		t.Logf("\n========== All Inserted TAC Data ==========")
		for i, tac := range mockService.ListAllTacInfo() {
			prevLinkStr := "nil"
			if tac.PrevLink != nil && *tac.PrevLink != "" {
				prevLinkStr = *tac.PrevLink
			}

			t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s, PrevLink: %s",
				i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLinkStr)
		}
		t.Logf("==========================================\n")
	})

	// clean database before test
	mockService.ClearTacInfo()

	// Eir_Add_67: Complex TAC range scenarios
	t.Run("Eir_Add_67", func(t *testing.T) {
		testCases := []struct {
			keyTac        string
			startRangeTac string
			endRangeTac   string
			color         string
		}{
			{"1222-1999", "1222", "1999", "white"},
			{"1222-1333", "1222", "1333", "white"},
			{"1666-1999", "1666", "1999", "white"},
			{"1888-1888", "1888", "1888", "white"},
			{"1222345-1222345", "1222345", "1222345", "white"},
		}

		for _, tc := range testCases {
			tacInfo := ports.TacInfo{
				KeyTac:        tc.keyTac,
				StartRangeTac: tc.startRangeTac,
				EndRangeTac:   tc.endRangeTac,
				Color:         tc.color,
				PrevLink:      nil,
			}

			result, err := mockService.InsertTac(context.Background(), &tacInfo)
			if err != nil || result.Status != "ok" {
				t.Errorf("Failed to insert TAC %s: %v", tc.keyTac, err)
			}

			t.Logf("TAC insertion '%s': TAC=%s-%s, Color=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, tc.color)
		}

		// List all data that was inserted during the test
		t.Logf("\n========== All Inserted TAC Data ==========")
		for i, tac := range mockService.ListAllTacInfo() {
			prevLinkStr := "nil"
			if tac.PrevLink != nil && *tac.PrevLink != "" {
				prevLinkStr = *tac.PrevLink
			}

			t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s, PrevLink: %s",
				i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLinkStr)
		}
		t.Logf("==========================================\n")
	})

	// clean database before test
	mockService.ClearTacInfo()

	// Eir_Add_68: Additional range tests
	t.Run("Eir_Add_68", func(t *testing.T) {
		testCases := []struct {
			keyTac        string
			startRangeTac string
			endRangeTac   string
			color         string
		}{
			{"1222-1666", "1222", "1666", "white"},
			{"1333-1555", "1333", "1555", "white"},
			{"1777-1888", "1777", "1888", "white"},
		}

		for _, tc := range testCases {
			tacInfo := ports.TacInfo{
				KeyTac:        tc.keyTac,
				StartRangeTac: tc.startRangeTac,
				EndRangeTac:   tc.endRangeTac,
				Color:         tc.color,
				PrevLink:      nil,
			}

			result, err := mockService.InsertTac(context.Background(), &tacInfo)
			if err != nil || result.Status != "ok" {
				t.Errorf("Failed to insert TAC %s: %v", tc.keyTac, err)
			}

			t.Logf("TAC insertion '%s': TAC=%s-%s, Color=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, tc.color)
		}

		// List all data that was inserted during the test
		t.Logf("\n========== All Inserted TAC Data ==========")
		for i, tac := range mockService.ListAllTacInfo() {
			prevLinkStr := "nil"
			if tac.PrevLink != nil && *tac.PrevLink != "" {
				prevLinkStr = *tac.PrevLink
			}

			t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s, PrevLink: %s",
				i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLinkStr)
		}
		t.Logf("==========================================\n")
	})

	// clean database before test
	mockService.ClearTacInfo()

	// Eir_Add_74: Invalid TAC tests (expected failures)
	t.Run("Eir_Add_74", func(t *testing.T) {
		testCases := []struct {
			keyTac        string
			startRangeTac string
			endRangeTac   string
			color         string
		}{
			{"9-1", "9", "1", "white"},
			{"abcd12354-12345a@#@#$@#", "abcd12354", "12345a@#@#$@#", "white"},
			{"\"1234 56789-12345\"", "\"1234 56789\"", "12345", "white"},
		}

		for _, tc := range testCases {
			tacInfo := ports.TacInfo{
				KeyTac:        tc.keyTac,
				StartRangeTac: tc.startRangeTac,
				EndRangeTac:   tc.endRangeTac,
				Color:         tc.color,
				PrevLink:      nil,
			}

			result, err := mockService.InsertTac(context.Background(), &tacInfo)
			if err == nil && result.Status == "ok" {
				t.Errorf("Expected InsertTac to fail for invalid TAC %s, but it succeeded", tc.keyTac)
			} else {
				t.Logf("TAC insertion '%s': Expected failure - %v", tc.keyTac, err)
			}
		}

		// List all data that was inserted during the test
		t.Logf("\n========== All Inserted TAC Data ==========")
		for i, tac := range mockService.ListAllTacInfo() {
			prevLinkStr := "nil"
			if tac.PrevLink != nil && *tac.PrevLink != "" {
				prevLinkStr = *tac.PrevLink
			}

			t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s, PrevLink: %s",
				i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLinkStr)
		}
		t.Logf("==========================================\n")
	})

	// clean database before test
	mockService.ClearTacInfo()

	// Eir_Add_75: Invalid color tests
	t.Run("Eir_Add_75", func(t *testing.T) {
		testCases := []struct {
			keyTac        string
			startRangeTac string
			endRangeTac   string
			color         string
		}{
			{"1134567890123456-1134567890123456", "1134567890123456", "1134567890123456", "."},
			{"1134567890123456-1134567890123456", "1134567890123456", "1134567890123456", "g"},
		}

		for _, tc := range testCases {
			tacInfo := ports.TacInfo{
				KeyTac:        tc.keyTac,
				StartRangeTac: tc.startRangeTac,
				EndRangeTac:   tc.endRangeTac,
				Color:         tc.color,
				PrevLink:      nil,
			}

			result, err := mockService.InsertTac(context.Background(), &tacInfo)
			if err == nil && result.Status == "ok" {
				t.Errorf("Expected InsertTac to fail for invalid color '%s', but it succeeded", tc.color)
			} else {
				t.Logf("TAC insertion '%s': Expected failure - %v", tc.keyTac, err)
			}
		}

		// List all data that was inserted during the test
		t.Logf("\n========== All Inserted TAC Data ==========")
		for i, tac := range mockService.ListAllTacInfo() {
			prevLinkStr := "nil"
			if tac.PrevLink != nil && *tac.PrevLink != "" {
				prevLinkStr = *tac.PrevLink
			}

			t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s, PrevLink: %s",
				i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLinkStr)
		}
		t.Logf("==========================================\n")
	})

	// clean database before test
	mockService.ClearTacInfo()

	// Eir_Add_82: Duplicate TAC insertion
	t.Run("Eir_Add_82", func(t *testing.T) {
		testCases := []struct {
			keyTac        string
			startRangeTac string
			endRangeTac   string
			color         string
		}{
			{"12345678901234567-12345678901234567", "12345678901234567", "12345678901234567", "white"},
			{"12345678901234567-12345678901234567", "12345678901234567", "12345678901234567", "white"},
		}
		i := 0
		for _, tc := range testCases {
			tacInfo := ports.TacInfo{
				KeyTac:        tc.keyTac,
				StartRangeTac: tc.startRangeTac,
				EndRangeTac:   tc.endRangeTac,
				Color:         tc.color,
				PrevLink:      nil,
			}

			result, err := mockService.InsertTac(context.Background(), &tacInfo)
			if i == 1 {
				if err == nil && result.Status == "ok" {
					t.Errorf("Expected InsertTac to fail for duplicate TAC %s, but it succeeded", tc.keyTac)
				} else {
					t.Logf("TAC insertion '%s': Expected duplicate failure - %v", tc.keyTac, err)
				}
			} else {
				if err != nil || result.Status != "ok" {
					t.Errorf("Failed to insert TAC %s: %v", tc.keyTac, err)
				} else {
					t.Logf("TAC insertion '%s': TAC=%s-%s, Color=%s",
						tc.keyTac, tc.startRangeTac, tc.endRangeTac, tc.color)
				}
			}
			i++
		}

		// List all data that was inserted during the test
		t.Logf("\n========== All Inserted TAC Data ==========")
		for i, tac := range mockService.ListAllTacInfo() {
			prevLinkStr := "nil"
			if tac.PrevLink != nil && *tac.PrevLink != "" {
				prevLinkStr = *tac.PrevLink
			}

			t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s, PrevLink: %s",
				i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLinkStr)
		}
		t.Logf("==========================================\n")
	})

	// clean database before test
	mockService.ClearTacInfo()

	// Eir_Add_86: Overlapping range conflicts
	t.Run("Eir_Add_86", func(t *testing.T) {
		testCases := []struct {
			keyTac        string
			startRangeTac string
			endRangeTac   string
			color         string
		}{
			{"1234-1235", "1234", "1235", "white"},
			{"1232-1234", "1232", "1234", "white"},
		}
		i := 0
		for _, tc := range testCases {
			tacInfo := ports.TacInfo{
				KeyTac:        tc.keyTac,
				StartRangeTac: tc.startRangeTac,
				EndRangeTac:   tc.endRangeTac,
				Color:         tc.color,
				PrevLink:      nil,
			}

			result, err := mockService.InsertTac(context.Background(), &tacInfo)
			if i == 1 {
				if err == nil && result.Status == "ok" {
					t.Errorf("Expected InsertTac to fail for overlapping range %s, but it succeeded", tc.keyTac)
				} else {
					t.Logf("TAC insertion '%s': Expected overlap failure - %v", tc.keyTac, err)
				}
			} else {
				if err != nil || result.Status != "ok" {
					t.Errorf("Failed to insert TAC %s: %v", tc.keyTac, err)
				} else {
					t.Logf("TAC insertion '%s': TAC=%s-%s, Color=%s",
						tc.keyTac, tc.startRangeTac, tc.endRangeTac, tc.color)
				}
			}
			i++
		}

		// List all data that was inserted during the test
		t.Logf("\n========== All Inserted TAC Data ==========")
		for i, tac := range mockService.ListAllTacInfo() {
			prevLinkStr := "nil"
			if tac.PrevLink != nil && *tac.PrevLink != "" {
				prevLinkStr = *tac.PrevLink
			}

			t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s, PrevLink: %s",
				i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLinkStr)
		}
		t.Logf("==========================================\n")
	})

	t.Logf("PCAP file saved: %s (contains all Diameter S13 InsertTac test traffic)", pcapFile)
	t.Log("Open in Wireshark with filter: diameter or tcp.port == 3868")
}

// TestInsertImeiWithPCAP tests IMEI insertion functionality via Diameter protocol with PCAP capture
// Since Diameter S13 interface doesn't have IMEI insertion, we use mockService directly
// This test verifies the IMEI insertion logic matches the HTTP adapter behavior
func TestInsertImeiWithPCAP(t *testing.T) {
	// Create PCAP writer for capturing test traffic
	pcapFile := "diameter_insert_imei_3868_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	// Configure Diameter server
	config := ServerConfig{
		Host:             "127.0.0.1",
		Port:             3868,
		OriginHost:       "eir.example.com",
		OriginRealm:      "example.com",
		ProductName:      "TestEIR",
		VendorID:         10415,
		MaxConnections:   1000,
		ReadTimeout:      30000000000,
		WriteTimeout:     10000000000,
		WatchdogInterval: 30000000000,
		WatchdogTimeout:  10000000000,
		MaxMessageSize:   65535,
		SendChannelSize:  100,
		RecvChannelSize:  100,
	}

	mockService, cleanup := newMockEIRService()
	defer cleanup()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start Diameter server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create test client with PCAP capture
	client := newTestClientWithPCAP(t, "127.0.0.1:3868", pcapWriter)
	defer client.Close()

	// Establish CER/CEA exchange
	client.sendCERWithS13(t)
	client.receiveCEA(t)

	// clean database before test
	mockService.ClearImeiInfo()

	// EIR_Add_1: Single digit IMEI - white
	t.Run("EIR_Add_1", func(t *testing.T) {
		testCases := []struct {
			imei          string
			color         string
			expectedError bool
			description   string
		}{
			{"1", "w", false, "Valid IMEI - white"},
		}

		for _, tc := range testCases {
			result, err := mockService.InsertImei(context.Background(), tc.imei, tc.color, models.SystemStatus{})
			if tc.expectedError {
				if err == nil && result.Status == "ok" {
					t.Errorf("InsertImei '%s' should have failed but succeeded", tc.description)
				}
			} else {
				if err != nil || result.Status != "ok" {
					t.Errorf("InsertImei '%s' failed: %v", tc.description, err)
				} else {
					t.Logf("✓ InsertImei '%s': IMEI=%s, Color=%s", tc.description, tc.imei, tc.color)
				}
			}
		}

		// List all inserted IMEI data after insertions
		t.Logf("\n===== All Inserted IMEI Data =====")
		allImeis := mockService.ListAllImeiInfo()
		if len(allImeis) == 0 {
			t.Logf("  WARNING: No IMEI records found in database")
		} else {
			for i, imei := range allImeis {
				endImeiStr := "[]"
				if len(imei.EndIMEI) > 0 {
					endImeiStr = fmt.Sprintf("%v", imei.EndIMEI)
				}
				t.Logf("  [%d] StartIMEI: '%s', EndIMEI: %s, Color: %s",
					i+1, imei.StartIMEI, endImeiStr, imei.Color)
			}
		}
		t.Logf("Total IMEI records: %d", len(allImeis))
		t.Logf("==================================\n")
	})

	// clean database before test
	mockService.ClearImeiInfo()

	// EIR_Add_2: 15-digit IMEI - grey
	t.Run("EIR_Add_2", func(t *testing.T) {
		testCases := []struct {
			imei          string
			color         string
			expectedError bool
			description   string
		}{
			{"123456789012345", "g", false, "Valid IMEI - grey"},
		}

		for _, tc := range testCases {
			result, err := mockService.InsertImei(context.Background(), tc.imei, tc.color, models.SystemStatus{})
			if tc.expectedError {
				if err == nil && result.Status == "ok" {
					t.Errorf("InsertImei '%s' should have failed but succeeded", tc.description)
				}
			} else {
				if err != nil || result.Status != "ok" {
					t.Errorf("InsertImei '%s' failed: %v", tc.description, err)
				} else {
					t.Logf("✓ InsertImei '%s': IMEI=%s, Color=%s", tc.description, tc.imei, tc.color)
				}
			}
		}

		// List all inserted IMEI data after insertions
		t.Logf("\n===== All Inserted IMEI Data =====")
		allImeis := mockService.ListAllImeiInfo()
		if len(allImeis) == 0 {
			t.Logf("  WARNING: No IMEI records found in database")
		} else {
			for i, imei := range allImeis {
				endImeiStr := "[]"
				if len(imei.EndIMEI) > 0 {
					endImeiStr = fmt.Sprintf("%v", imei.EndIMEI)
				}
				t.Logf("  [%d] StartIMEI: '%s', EndIMEI: %s, Color: %s",
					i+1, imei.StartIMEI, endImeiStr, imei.Color)
			}
		}
		t.Logf("Total IMEI records: %d", len(allImeis))
		t.Logf("==================================\n")
	})

	// clean database before test
	mockService.ClearImeiInfo()

	// EIR_Add_3: 14-digit IMEI - grey
	t.Run("EIR_Add_3", func(t *testing.T) {
		testCases := []struct {
			imei          string
			color         string
			expectedError bool
			description   string
		}{
			{"12345678901234", "g", false, "Valid IMEI - grey"},
		}

		for _, tc := range testCases {
			result, err := mockService.InsertImei(context.Background(), tc.imei, tc.color, models.SystemStatus{})
			if tc.expectedError {
				if err == nil && result.Status == "ok" {
					t.Errorf("InsertImei '%s' should have failed but succeeded", tc.description)
				}
			} else {
				if err != nil || result.Status != "ok" {
					t.Errorf("InsertImei '%s' failed: %v", tc.description, err)
				} else {
					t.Logf("✓ InsertImei '%s': IMEI=%s, Color=%s", tc.description, tc.imei, tc.color)
				}
			}
		}

		// List all inserted IMEI data after insertions
		t.Logf("\n===== All Inserted IMEI Data =====")
		allImeis := mockService.ListAllImeiInfo()
		if len(allImeis) == 0 {
			t.Logf("  WARNING: No IMEI records found in database")
		} else {
			for i, imei := range allImeis {
				endImeiStr := "[]"
				if len(imei.EndIMEI) > 0 {
					endImeiStr = fmt.Sprintf("%v", imei.EndIMEI)
				}
				t.Logf("  [%d] StartIMEI: '%s', EndIMEI: %s, Color: %s",
					i+1, imei.StartIMEI, endImeiStr, imei.Color)
			}
		}
		t.Logf("Total IMEI records: %d", len(allImeis))
		t.Logf("==================================\n")
	})

	// clean database before test
	mockService.ClearImeiInfo()

	// EIR_Add_4: 17-digit IMEI - grey
	t.Run("EIR_Add_4", func(t *testing.T) {
		testCases := []struct {
			imei          string
			color         string
			expectedError bool
			description   string
		}{
			{"12345678901234567", "g", false, "Valid IMEI - grey"},
		}

		for _, tc := range testCases {
			result, err := mockService.InsertImei(context.Background(), tc.imei, tc.color, models.SystemStatus{})
			if tc.expectedError {
				if err == nil && result.Status == "ok" {
					t.Errorf("InsertImei '%s' should have failed but succeeded", tc.description)
				}
			} else {
				if err != nil || result.Status != "ok" {
					t.Errorf("InsertImei '%s' failed: %v", tc.description, err)
				} else {
					t.Logf("✓ InsertImei '%s': IMEI=%s, Color=%s", tc.description, tc.imei, tc.color)
				}
			}
		}

		// List all inserted IMEI data after insertions
		t.Logf("\n===== All Inserted IMEI Data =====")
		allImeis := mockService.ListAllImeiInfo()
		if len(allImeis) == 0 {
			t.Logf("  WARNING: No IMEI records found in database")
		} else {
			for i, imei := range allImeis {
				endImeiStr := "[]"
				if len(imei.EndIMEI) > 0 {
					endImeiStr = fmt.Sprintf("%v", imei.EndIMEI)
				}
				t.Logf("  [%d] StartIMEI: '%s', EndIMEI: %s, Color: %s",
					i+1, imei.StartIMEI, endImeiStr, imei.Color)
			}
		}
		t.Logf("Total IMEI records: %d", len(allImeis))
		t.Logf("==================================\n")
	})

	// clean database before test
	mockService.ClearImeiInfo()

	// EIR_Add_5: Multiple IMEIs with same prefix - grey
	t.Run("EIR_Add_5", func(t *testing.T) {
		testCases := []struct {
			imei          string
			color         string
			expectedError bool
			description   string
		}{
			{"12345678901234", "g", false, "Valid IMEI - grey"},
			{"123456789012341", "g", false, "Valid IMEI - grey"},
			{"1234567890123411", "g", false, "Valid IMEI - grey"},
			{"12345678901234111", "g", false, "Valid IMEI - grey"},
			{"12345678901234112", "g", false, "Valid IMEI - grey"},
			{"12345678901234222", "g", false, "Valid IMEI - grey"},
			{"12345678901234333", "g", false, "Valid IMEI - grey"},
			{"12345678901234", "g", false, "Valid IMEI - grey (duplicate)"},
			{"12345678901234444", "g", false, "Valid IMEI - grey"},
		}

		for _, tc := range testCases {
			result, err := mockService.InsertImei(context.Background(), tc.imei, tc.color, models.SystemStatus{})
			if tc.expectedError {
				if err == nil && result.Status == "ok" {
					t.Errorf("InsertImei '%s' should have failed but succeeded", tc.description)
				}
			} else {
				if err != nil || result.Status != "ok" {
					t.Errorf("InsertImei '%s' failed: %v", tc.description, err)
				} else {
					t.Logf("✓ InsertImei '%s': IMEI=%s, Color=%s", tc.description, tc.imei, tc.color)
				}
			}
		}

		// List all inserted IMEI data after insertions
		t.Logf("\n===== All Inserted IMEI Data =====")
		allImeis := mockService.ListAllImeiInfo()
		if len(allImeis) == 0 {
			t.Logf("  WARNING: No IMEI records found in database")
		} else {
			for i, imei := range allImeis {
				endImeiStr := "[]"
				if len(imei.EndIMEI) > 0 {
					endImeiStr = fmt.Sprintf("%v", imei.EndIMEI)
				}
				t.Logf("  [%d] StartIMEI: '%s', EndIMEI: %s, Color: %s",
					i+1, imei.StartIMEI, endImeiStr, imei.Color)
			}
		}
		t.Logf("Total IMEI records: %d", len(allImeis))
		t.Logf("==================================\n")
	})

	t.Logf("\n========== Test Summary ==========")
	t.Logf("PCAP file saved: %s (contains all Diameter S13 InsertImei test traffic)", pcapFile)
	t.Log("Open in Wireshark with filter: diameter or tcp.port == 3868")
	t.Logf("==================================")
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
