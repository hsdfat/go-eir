package http

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/hsdfat8/eir/internal/adapters/postgres"
	"github.com/hsdfat8/eir/internal/adapters/testutil"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
	legacyModels "github.com/hsdfat8/eir/models"
	"github.com/hsdfat8/eir/pkg/logic"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"golang.org/x/net/http2"
)

// mockEIRService is a mock implementation of EIRService for testing
type mockEIRService struct {
	imeiRepo      ports.IMEIRepository
	insertedTacs  []ports.TacInfo
	insertedImeis []string
}

// newMockEIRService creates a properly initialized mock service
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
	// Convert domain model to legacy model
	legacyStatus := legacyModels.SystemStatus{
		OverloadLevel: status.OverloadLevel,
		TPSOverload:   status.TPSOverload,
	}

	// Use pkg/logic for IMEI checking
	result := logic.CheckImei(imei, legacyStatus)

	return &ports.CheckImeiResult{
		Status: result.Status,
		IMEI:   result.IMEI,
		Color:  result.Color,
	}, nil
}

func (m *mockEIRService) CheckTac(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckTacResult, error) {
	// Convert domain model to legacy model
	legacyStatus := legacyModels.SystemStatus{
		OverloadLevel: status.OverloadLevel,
		TPSOverload:   status.TPSOverload,
	}

	// Use pkg/logic for TAC checking
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
	// Convert domain model to legacy model
	legacyStatus := legacyModels.SystemStatus{
		OverloadLevel: status.OverloadLevel,
		TPSOverload:   status.TPSOverload,
	}

	// Use pkg/logic for IMEI insertion with the imeiRepo
	result := logic.InsertImei(m.imeiRepo, imei, color, legacyStatus)

	// Track inserted IMEIs
	if m.insertedImeis == nil {
		m.insertedImeis = []string{}
	}
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
		errStr := "invalid_parameter"
		return &ports.InsertTacResult{
			Status: "error",
			Error:  &errStr,
		}, fmt.Errorf("tacInfo is required")
	}

	// Convert domain TAC info to legacy model
	legacyTacInfo := legacyModels.TacInfo{
		KeyTac:        tacInfo.KeyTac,
		StartRangeTac: tacInfo.StartRangeTac,
		EndRangeTac:   tacInfo.EndRangeTac,
		Color:         tacInfo.Color,
		PrevLink:      tacInfo.PrevLink,
	}

	// Use pkg/logic for TAC insertion with the imeiRepo
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

	return &ports.InsertTacResult{
		Status:  result.Status,
		Error:   errorPtr,
		TacInfo: resultTacInfo,
	}, nil
}

func (m *mockEIRService) ListAllTacInfo() []*ports.TacInfo {
	ctx := context.Background()
	return m.imeiRepo.ListAllTacInfo(ctx)
}

func (m *mockEIRService) ClearTacInfo() {
	ctx := context.Background()
	m.imeiRepo.ClearTacInfo(ctx)
}

func (m *mockEIRService) ClearImeiInfo() {
	ctx := context.Background()
	m.imeiRepo.ClearImeiInfo(ctx)
}

func (m *mockEIRService) ListAllImeiInfo() []*ports.ImeiInfo {
	ctx := context.Background()
	return m.imeiRepo.ListAllImeiInfo(ctx)
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

// TestServerHTTP1Basic tests basic HTTP/1.1 server
func TestServerHTTP1Basic(t *testing.T) {
	config := ServerConfig{
		ListenAddr: "127.0.0.1:8080", // Standard HTTP port
	}

	mockService, _ := newMockEIRService()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	t.Logf("HTTP/1.1 server started on %s", server.GetAddr())

	if !server.IsRunning() {
		t.Fatal("Server should be running")
	}

	t.Log("HTTP/1.1 server test passed")
}

// TestServerHTTP1WithPCAP tests HTTP/1.1 server with PCAP capture (easier Wireshark decoding)
func TestServerHTTP1WithPCAP(t *testing.T) {
	// Create PCAP writer in the same directory as the test file
	pcapFile := "http1_8080_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	config := ServerConfig{
		ListenAddr: "127.0.0.1:8080", // Standard HTTP port
		EnableH2C:  false,            // Use HTTP/1.1 for easier Wireshark decoding
	}

	mockService, _ := newMockEIRService()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Get actual listening address
	addr := "127.0.0.1:8080"
	t.Logf("HTTP/1.1 server started on %s", addr)

	// Create HTTP/1.1 client with PCAP capture
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

	// Test health check endpoint
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := client.Get(fmt.Sprintf("http://%s/health", addr))
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Logf("Health check passed: HTTP/%d.%d", resp.ProtoMajor, resp.ProtoMinor)
	})

	// Test equipment status check (5G N5g-eir API)
	t.Run("GetEquipmentStatus", func(t *testing.T) {
		imei := "123456789012345"
		url := fmt.Sprintf("http://%s/n5g-eir-eic/v1/equipment-status?pei=%s", addr, imei)

		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("Equipment status check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result EirResponseData
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result.Status != models.EquipmentStatusWhitelisted {
			t.Errorf("Expected status WHITELISTED, got %s", result.Status)
		}

		t.Logf("Equipment status check passed: %s", result.Status)
	})

	// Test equipment provisioning
	t.Run("ProvisionEquipment", func(t *testing.T) {
		provision := ProvisionRequest{
			IMEI:   "123456789012345",
			Status: models.EquipmentStatusWhitelisted,
		}

		body, _ := json.Marshal(provision)
		url := fmt.Sprintf("http://%s/api/v1/equipment", addr)

		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Provision equipment failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(bodyBytes))
		}

		t.Log("Equipment provisioning passed")
	})

	t.Logf("PCAP file saved: %s", pcapFile)
}

// TestServerH2C tests HTTP/2 cleartext server with PCAP capture
func TestServerH2C(t *testing.T) {
	// Create PCAP writer in the same directory as the test file
	pcapFile := "http2_h2c_8080_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	config := ServerConfig{
		ListenAddr: "127.0.0.1:8080", // Standard HTTP port
		EnableH2C:  true,
	}

	mockService, _ := newMockEIRService()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Get actual listening address
	addr := "127.0.0.1:8080"
	t.Logf("HTTP/2 (H2C) server started on %s", addr)

	// Create H2C client
	client := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				conn, err := net.Dial(network, addr)
				if err != nil {
					return nil, err
				}
				// Wrap connection with PCAP capture
				return testutil.NewCaptureConnection(conn, pcapWriter), nil
			},
		},
		Timeout: 5 * time.Second,
	}

	// Test health check endpoint
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := client.Get(fmt.Sprintf("http://%s/health", addr))
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}

		if resp.ProtoMajor != 2 {
			t.Errorf("Expected HTTP/2, got HTTP/%d", resp.ProtoMajor)
		}

		t.Logf("Health check passed: HTTP/%d.%d", resp.ProtoMajor, resp.ProtoMinor)
	})

	// Test equipment status check (5G N5g-eir API)
	t.Run("GetEquipmentStatus", func(t *testing.T) {
		imei := "123456789012345"
		url := fmt.Sprintf("http://%s/n5g-eir-eic/v1/equipment-status?pei=%s", addr, imei)

		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("Equipment status check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result EirResponseData
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result.Status != models.EquipmentStatusWhitelisted {
			t.Errorf("Expected status WHITELISTED, got %s", result.Status)
		}

		t.Logf("Equipment status check passed: %s", result.Status)
	})

	// Test equipment provisioning
	t.Run("ProvisionEquipment", func(t *testing.T) {
		provision := ProvisionRequest{
			IMEI:   "123456789012345",
			Status: models.EquipmentStatusWhitelisted,
		}

		body, _ := json.Marshal(provision)
		url := fmt.Sprintf("http://%s/api/v1/equipment", addr)

		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Provision equipment failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(bodyBytes))
		}

		t.Log("Equipment provisioning passed")
	})

	t.Logf("PCAP file saved: %s", pcapFile)
}

// TestServerH2CMultipleRequests tests concurrent HTTP/2 requests with PCAP
func TestServerH2CMultipleRequests(t *testing.T) {
	// Create PCAP writer in the same directory as the test file
	pcapFile := "http2_concurrent_8080_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	config := ServerConfig{
		ListenAddr: "127.0.0.1:8080", // Standard HTTP port
		EnableH2C:  true,
	}

	mockService, _ := newMockEIRService()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	addr := "127.0.0.1:8080"

	// Create H2C client
	client := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				conn, err := net.Dial(network, addr)
				if err != nil {
					return nil, err
				}
				return testutil.NewCaptureConnection(conn, pcapWriter), nil
			},
		},
		Timeout: 10 * time.Second,
	}

	// Send multiple concurrent requests
	numRequests := 20
	startTime := time.Now()

	done := make(chan bool, numRequests)
	for i := 0; i < numRequests; i++ {
		go func(index int) {
			imei := fmt.Sprintf("12345678901234%d", index)
			url := fmt.Sprintf("http://%s/n5g-eir-eic/v1/equipment-status?pei=%s", addr, imei)

			resp, err := client.Get(url)
			if err != nil {
				t.Errorf("Request %d failed: %v", index, err)
				done <- false
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Errorf("Request %d: Expected status 200, got %d", index, resp.StatusCode)
				done <- false
				return
			}

			done <- true
		}(i)
	}

	// Wait for all requests
	successCount := 0
	for i := 0; i < numRequests; i++ {
		if <-done {
			successCount++
		}
	}

	duration := time.Since(startTime)
	throughput := float64(numRequests) / duration.Seconds()

	t.Logf("Processed %d/%d requests in %v (%.2f req/sec)",
		successCount, numRequests, duration, throughput)

	if successCount != numRequests {
		t.Errorf("Expected %d successful requests, got %d", numRequests, successCount)
	}

	t.Logf("PCAP file saved: %s", pcapFile)
}

// TestServerGracefulShutdown tests graceful server shutdown
func TestServerGracefulShutdown(t *testing.T) {
	config := ServerConfig{
		ListenAddr:      "127.0.0.1:8080", // Standard HTTP port
		EnableH2C:       true,
		ShutdownTimeout: 5 * time.Second,
	}

	mockService, _ := newMockEIRService()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !server.IsRunning() {
		t.Fatal("Server should be running")
	}

	// Stop server
	if err := server.Stop(); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	t.Log("Graceful shutdown test passed")
}

func TestCheckImeiWithPCAP(t *testing.T) {
	pcapFile := "http2_check_imei_8080_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	config := ServerConfig{
		ListenAddr: "127.0.0.1:8080",
		EnableH2C:  true,
	}

	mockService, cleanup := newMockEIRService()
	defer cleanup()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	addr := "127.0.0.1:8080"
	t.Logf("Server started on %s for CheckImei testing", addr)

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

	// EIR_10: Insert blacklisted IMEI then check it
	t.Run("InsertAndCheck_Blacklisted_EIR_10", func(t *testing.T) {
		// Clear database before test
		mockService.ClearImeiInfo()

		imei := "9"
		color := "b"

		// Step 1: Insert IMEI with black color
		insertReq := ports.ImeiInfoInsert{
			Imei:  imei,
			Color: color,
		}
		body, _ := json.Marshal(insertReq)
		insertURL := fmt.Sprintf("http://%s/api/v1/insert-imei", addr)

		resp, err := client.Post(insertURL, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Insert IMEI request failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Insert IMEI failed with status %d", resp.StatusCode)
		}
		t.Logf("✓ Inserted IMEI: %s with color: %s", imei, color)

		// Step 2: Check IMEI
		checkURL := fmt.Sprintf("http://%s/api/v1/check-imei/%s", addr, imei)
		resp, err = client.Get(checkURL)
		if err != nil {
			t.Fatalf("CheckImei request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result EirResponseData
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result.Status != models.EquipmentStatusBlacklisted {
			t.Errorf("Expected status %s, got %s", models.EquipmentStatusBlacklisted, result.Status)
		}

		t.Logf("✓ Check IMEI passed: IMEI=%s, Status=%s", imei, result.Status)
	})

	// EIR_11: Insert greylisted IMEI then check it
	t.Run("InsertAndCheck_Greylisted_EIR_11", func(t *testing.T) {
		// Clear database before test
		mockService.ClearImeiInfo()

		imei := "912"
		color := "g"

		// Step 1: Insert IMEI with grey color
		insertReq := ports.ImeiInfoInsert{
			Imei:  imei,
			Color: color,
		}
		body, _ := json.Marshal(insertReq)
		insertURL := fmt.Sprintf("http://%s/api/v1/insert-imei", addr)

		resp, err := client.Post(insertURL, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Insert IMEI request failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Insert IMEI failed with status %d", resp.StatusCode)
		}
		t.Logf("✓ Inserted IMEI: %s with color: %s", imei, color)

		// Step 2: Check IMEI
		checkURL := fmt.Sprintf("http://%s/api/v1/check-imei/%s", addr, imei)
		resp, err = client.Get(checkURL)
		if err != nil {
			t.Fatalf("CheckImei request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result EirResponseData
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result.Status != models.EquipmentStatusGreylisted {
			t.Errorf("Expected status %s, got %s", models.EquipmentStatusGreylisted, result.Status)
		}

		t.Logf("✓ Check IMEI passed: IMEI=%s, Status=%s", imei, result.Status)
	})

	// EIR_12: Insert long blacklisted IMEI then check it
	t.Run("InsertAndCheck_LongBlacklisted_EIR_12", func(t *testing.T) {
		// Clear database before test
		mockService.ClearImeiInfo()

		imei := "9123456789012"
		color := "b"

		// Step 1: Insert IMEI with black color
		insertReq := ports.ImeiInfoInsert{
			Imei:  imei,
			Color: color,
		}
		body, _ := json.Marshal(insertReq)
		insertURL := fmt.Sprintf("http://%s/api/v1/insert-imei", addr)

		resp, err := client.Post(insertURL, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Insert IMEI request failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Insert IMEI failed with status %d", resp.StatusCode)
		}
		t.Logf("✓ Inserted IMEI: %s with color: %s", imei, color)

		// Step 2: Check IMEI
		checkURL := fmt.Sprintf("http://%s/api/v1/check-imei/%s", addr, imei)
		resp, err = client.Get(checkURL)
		if err != nil {
			t.Fatalf("CheckImei request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result EirResponseData
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result.Status != models.EquipmentStatusBlacklisted {
			t.Errorf("Expected status %s, got %s", models.EquipmentStatusBlacklisted, result.Status)
		}

		t.Logf("✓ Check IMEI passed: IMEI=%s, Status=%s", imei, result.Status)
	})

	// EIR_13: Insert whitelisted IMEI then check it
	t.Run("InsertAndCheck_Whitelisted_EIR_13", func(t *testing.T) {
		// Clear database before test
		mockService.ClearImeiInfo()

		imei := "91234567895264"
		color := "w"

		// Step 1: Insert IMEI with white color
		insertReq := ports.ImeiInfoInsert{
			Imei:  imei,
			Color: color,
		}
		body, _ := json.Marshal(insertReq)
		insertURL := fmt.Sprintf("http://%s/api/v1/insert-imei", addr)

		resp, err := client.Post(insertURL, "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Insert IMEI request failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Insert IMEI failed with status %d", resp.StatusCode)
		}
		t.Logf("✓ Inserted IMEI: %s with color: %s", imei, color)

		// Step 2: Check IMEI
		checkURL := fmt.Sprintf("http://%s/api/v1/check-imei/%s", addr, imei)
		resp, err = client.Get(checkURL)
		if err != nil {
			t.Fatalf("CheckImei request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(body))
		}

		var result EirResponseData
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result.Status != models.EquipmentStatusWhitelisted {
			t.Errorf("Expected status %s, got %s", models.EquipmentStatusWhitelisted, result.Status)
		}

		t.Logf("✓ Check IMEI passed: IMEI=%s, Status=%s", imei, result.Status)
	})

	// Missing IMEI parameter
	t.Run("MissingIMEIParameter", func(t *testing.T) {
		url := fmt.Sprintf("http://%s/n5g-eir-eic/v1/check-imei", addr)

		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 for missing IMEI, got %d", resp.StatusCode)
		}
	})

	// Invalid imei
	t.Run("IMEIVariations", func(t *testing.T) {
		testCases := []struct {
			name string
			imei string
		}{
			{"error", "11111111111111"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				url := fmt.Sprintf("http://%s/n5g-eir-eic/v1/check-imei?imei=%s", addr, tc.imei)

				resp, err := client.Get(url)
				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusNotFound {
					t.Errorf("Expected 404 invalid IMEI, got %d", resp.StatusCode)
				}
			})
		}
	})

	t.Logf("PCAP file saved: %s (contains all CheckImei test traffic)", pcapFile)
}

// TestCheckTacWithPCAP tests CheckTac functionality with HTTP/2 and PCAP capture
// Flow: 1) Insert TAC ranges into database, 2) Check various IMEIs against TAC ranges
func TestCheckTacWithPCAP(t *testing.T) {
	// Create PCAP writer for capturing test traffic
	pcapFile := "http2_check_tac_8080_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	// Configure server for HTTP/2 (H2C)
	config := ServerConfig{
		ListenAddr: "127.0.0.1:8080",
		EnableH2C:  true,
	}

	mockService, cleanup := newMockEIRService()
	defer cleanup()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	addr := "127.0.0.1:8080"
	t.Logf("HTTP/2 server started on %s for CheckTac testing", addr)

	// Create HTTP/2 client with PCAP capture
	client := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				conn, err := net.Dial(network, addr)
				if err != nil {
					return nil, err
				}
				return testutil.NewCaptureConnection(conn, pcapWriter), nil
			},
		},
		Timeout: 5 * time.Second,
	}

	// Clear database before test
	mockService.ClearTacInfo()

	// Step 1: Insert TAC ranges
	t.Run("Step1_InsertTacRanges", func(t *testing.T) {
		testCases := []struct {
			startRangeTac string
			endRangeTac   string
			color         string
			description   string
		}{
			{"35", "35", "black", "Single TAC - Blacklisted"},
			{"353", "353", "grey", "Single TAC - Greylisted"},
			{"3531", "3531", "white", "Single TAC - Whitelisted"},
			{"35310", "35319", "white", "TAC range (10 values) - Whitelisted"},
			{"353200", "353299", "grey", "TAC range (100 values) - Greylisted"},
			{"3533000", "3533999", "black", "TAC range (1000 values) - Blacklisted"},
			{"35340000", "35349999", "white", "Large TAC range (10000 values) - Whitelisted"},
			{"90", "99", "black", "TAC range for testing"},
			{"912", "912", "grey", "Exact match greylisted"},
			{"9123456789012", "9123456789012", "black", "Long TAC blacklisted"},
			{"91234567895264", "91234567895264", "white", "Exact IMEI whitelisted"},
		}

		for _, tc := range testCases {
			tacInfo := ports.TacInfo{
				StartRangeTac: tc.startRangeTac,
				EndRangeTac:   tc.endRangeTac,
				Color:         tc.color,
			}

			body, _ := json.Marshal(tacInfo)
			url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertTac request failed for %s: %v", tc.description, err)
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
				t.Errorf("TAC insertion '%s' failed: Status=%d, Response=%s",
					tc.description, resp.StatusCode, string(bodyBytes))
			} else {
				t.Logf("✓ TAC inserted '%s': Range=%s-%s, Color=%s",
					tc.description, tc.startRangeTac, tc.endRangeTac, tc.color)
			}
		}

		// List all inserted TAC data
		t.Logf("\n========== Inserted TAC Ranges ==========")
		allTacs := mockService.ListAllTacInfo()
		for i, tac := range allTacs {
			prevLinkStr := "nil"
			if tac.PrevLink != nil && *tac.PrevLink != "" {
				prevLinkStr = *tac.PrevLink
			}
			t.Logf("  [%d] Range: %s-%s, Color: %s, PrevLink: %s",
				i+1, tac.StartRangeTac, tac.EndRangeTac, tac.Color, prevLinkStr)
		}
		t.Logf("Total TAC ranges: %d", len(allTacs))
		t.Logf("==========================================\n")
	})

	// Step 2: Check IMEIs against TAC ranges
	t.Run("Step2_CheckTacQueries", func(t *testing.T) {
		testCases := []struct {
			imei           string
			expectedColor  string
			expectedStatus models.EquipmentStatus
			description    string
		}{
			// Test exact matches
			{"35", "black", models.EquipmentStatusBlacklisted, "Exact match - Blacklisted"},
			{"353", "grey", models.EquipmentStatusGreylisted, "Exact match - Greylisted"},
			{"3531", "white", models.EquipmentStatusWhitelisted, "Exact match - Whitelisted"},

			// Test range matches
			{"35310", "white", models.EquipmentStatusWhitelisted, "Start of range"},
			{"35315", "white", models.EquipmentStatusWhitelisted, "Middle of range"},
			{"35319", "white", models.EquipmentStatusWhitelisted, "End of range"},
			{"353200", "grey", models.EquipmentStatusGreylisted, "Start of 100-value range"},
			{"353250", "grey", models.EquipmentStatusGreylisted, "Middle of 100-value range"},
			{"353299", "grey", models.EquipmentStatusGreylisted, "End of 100-value range"},

			// Test large range
			{"3533000", "black", models.EquipmentStatusBlacklisted, "Start of large range"},
			{"3533500", "black", models.EquipmentStatusBlacklisted, "Middle of large range"},
			{"3533999", "black", models.EquipmentStatusBlacklisted, "End of large range"},

			// Test very large range
			{"35340000", "white", models.EquipmentStatusWhitelisted, "Start of very large range"},
			{"35345000", "white", models.EquipmentStatusWhitelisted, "Middle of very large range"},
			{"35349999", "white", models.EquipmentStatusWhitelisted, "End of very large range"},

			// Test specific IMEIs from CheckImei tests
			{"9", "black", models.EquipmentStatusBlacklisted, "Single digit in black range"},
			{"912", "grey", models.EquipmentStatusGreylisted, "Exact greylisted TAC"},
			{"9123456789012", "black", models.EquipmentStatusBlacklisted, "Long blacklisted TAC"},
			{"91234567895264", "white", models.EquipmentStatusWhitelisted, "Exact whitelisted IMEI"},

			// Test out of range (should return unknown color with error status)
			{"1", "unknown", models.EquipmentStatusWhitelisted, "Not in any range"},
			{"88", "unknown", models.EquipmentStatusWhitelisted, "Below 90-99 range"},
			{"100", "unknown", models.EquipmentStatusWhitelisted, "Above 90-99 range"},
		}

		for _, tc := range testCases {
			url := fmt.Sprintf("http://%s/api/v1/check-tac/%s", addr, tc.imei)

			resp, err := client.Get(url)
			if err != nil {
				t.Errorf("CheckTac request failed for IMEI %s: %v", tc.imei, err)
				continue
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Logf("  CheckTac '%s': IMEI=%s, Status=%d, Response=%s",
					tc.description, tc.imei, resp.StatusCode, string(bodyBytes))
				continue
			}

			var result struct {
				Status  string         `json:"status"`
				IMEI    string         `json:"imei"`
				Color   string         `json:"color"`
				TacInfo *ports.TacInfo `json:"tac_info,omitempty"`
			}

			if err := json.Unmarshal(bodyBytes, &result); err != nil {
				t.Errorf("Failed to decode response for IMEI %s: %v", tc.imei, err)
				continue
			}

			// Verify color match
			if result.Color != tc.expectedColor {
				t.Errorf("CheckTac '%s': Expected color %s, got %s (IMEI=%s)",
					tc.description, tc.expectedColor, result.Color, tc.imei)
			} else {
				tacInfoStr := "nil"
				if result.TacInfo != nil {
					tacInfoStr = fmt.Sprintf("%s-%s", result.TacInfo.StartRangeTac, result.TacInfo.EndRangeTac)
				}
				t.Logf("  ✓ CheckTac '%s': IMEI=%s, Color=%s, TacRange=%s",
					tc.description, tc.imei, result.Color, tacInfoStr)
			}
		}
	})

	t.Logf("\n========== Test Summary ==========")
	t.Logf("PCAP file saved: %s", pcapFile)
	t.Log("Open in Wireshark with filter: http2 or tcp.port == 8080")
	t.Logf("==================================")
}

// TestInsertTacWithPCAP tests InsertTac functionality with HTTP/2 and PCAP capture
// This test verifies the TAC insertion logic through the HTTP API
func TestInsertTacWithPCAP(t *testing.T) {
	// Create PCAP writer for capturing test traffic
	pcapFile := "http2_insert_tac_8080_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	// Configure server for HTTP/2 (H2C)
	config := ServerConfig{
		ListenAddr: "127.0.0.1:8080",
		EnableH2C:  true, // Enable HTTP/2 Cleartext
	}

	mockService, cleanup := newMockEIRService()
	defer cleanup()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	addr := "127.0.0.1:8080"
	t.Logf("HTTP/2 server started on %s for InsertTac testing", addr)

	// Create HTTP/2 client with PCAP capture
	client := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				conn, err := net.Dial(network, addr)
				if err != nil {
					return nil, err
				}
				// Wrap connection with PCAP capture
				return testutil.NewCaptureConnection(conn, pcapWriter), nil
			},
		},
		Timeout: 5 * time.Second,
	}

	// clean database before test
	mockService.ClearTacInfo()

	// Eir_Add_63
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

			body, _ := json.Marshal(tacInfo)
			url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, err)
				continue
			}
			if resp.StatusCode == http.StatusBadRequest {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, resp.Status)
				continue
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
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
	// Eir_Add_64
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

			body, _ := json.Marshal(tacInfo)
			url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, err)
				continue
			}
			if resp.StatusCode == http.StatusBadRequest {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, resp.Status)
				continue
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
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
	// Eir_Add_65
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

			body, _ := json.Marshal(tacInfo)
			url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, err)
				continue
			}
			if resp.StatusCode == http.StatusBadRequest {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, resp.Status)
				continue
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
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
	// Eir_Add_66
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

			body, _ := json.Marshal(tacInfo)
			url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, err)
				continue
			}
			if resp.StatusCode == http.StatusBadRequest {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, resp.Status)
				continue
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
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
	// Eir_Add_67
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

			body, _ := json.Marshal(tacInfo)
			url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, err)
				continue
			}
			if resp.StatusCode == http.StatusBadRequest {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, resp.Status)
				continue
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
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
	// EIR_Add_68
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

			body, _ := json.Marshal(tacInfo)
			url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, err)
				continue
			}
			if resp.StatusCode == http.StatusBadRequest {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, resp.Status)
				continue
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
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
	// EIR_Add_74
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

			body, _ := json.Marshal(tacInfo)
			url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, err)
				continue
			}
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("InsertTac request expected failed for %s: %v", tc.keyTac, resp.Status)
				continue
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
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
	// EIR_Add_75
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

			body, _ := json.Marshal(tacInfo)
			url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, err)
				continue
			}
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("InsertTac request expected failed for %s: %v", tc.keyTac, resp.Status)
				continue
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
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
	// EIR_Add_82
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

			body, _ := json.Marshal(tacInfo)
			url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, err)
				continue
			}
			if i == 1 {
				if resp.StatusCode != http.StatusBadRequest {
					t.Fatalf("InsertTac request expected failed for %s: %v", tc.keyTac, resp.Status)
					continue
				}
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			i++
			t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
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
	// EIR_Add_86
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

			body, _ := json.Marshal(tacInfo)
			url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertTac request failed for %s: %v", tc.keyTac, err)
				continue
			}
			if i == 1 {
				if resp.StatusCode != http.StatusBadRequest {
					t.Fatalf("InsertTac request expected failed for %s: %v", tc.keyTac, resp.Status)
					continue
				}
			}
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			i++
			t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
				tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
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
	t.Logf("PCAP file saved: %s (contains all InsertTac HTTP/2 test traffic)", pcapFile)
	t.Log("Open in Wireshark with filter: http2 or tcp.port == 8080")
}

// TestInsertImeiWithPCAP tests InsertImei functionality with HTTP/2 and PCAP capture
func TestInsertImeiWithPCAP(t *testing.T) {
	// Create PCAP writer for capturing test traffic
	pcapFile := "http2_insert_imei_8080_test.pcap"
	pcapWriter, err := testutil.NewPCAPWriter(pcapFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}
	defer pcapWriter.Close()

	// Configure server for HTTP/2 (H2C)
	config := ServerConfig{
		ListenAddr: "127.0.0.1:8080",
		EnableH2C:  true,
	}

	mockService, cleanup := newMockEIRService()
	defer cleanup()
	server := NewServer(config, mockService)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	addr := "127.0.0.1:8080"
	t.Logf("HTTP/2 server started on %s for InsertImei testing", addr)

	// Create HTTP/2 client with PCAP capture
	client := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
				conn, err := net.Dial(network, addr)
				if err != nil {
					return nil, err
				}
				return testutil.NewCaptureConnection(conn, pcapWriter), nil
			},
		},
		Timeout: 5 * time.Second,
	}

	// Clear database before test
	mockService.ClearImeiInfo()
	// EIR_Add_1
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
			provision := ports.ImeiInfoInsert{
				Imei:  tc.imei,
				Color: tc.color,
			}

			body, _ := json.Marshal(provision)
			url := fmt.Sprintf("http://%s/api/v1/insert-imei", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertImei request failed for %s: %v", tc.description, err)
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if tc.expectedError {
				if resp.StatusCode == http.StatusCreated {
					t.Errorf("InsertImei '%s' should have failed but succeeded", tc.description)
				}
			} else {
				if resp.StatusCode != http.StatusCreated {
					t.Errorf("InsertImei '%s' failed: Status=%d, Response=%s",
						tc.description, resp.StatusCode, string(bodyBytes))
				} else {
					t.Logf("✓ InsertImei '%s': IMEI=%s, Color=%s, Status=%d",
						tc.description, tc.imei, tc.color, resp.StatusCode)
				}
			}
		}

		// List all inserted IMEI data after insertions
		t.Logf("\n===== All Inserted IMEI Data =====")
		allImeis := mockService.ListAllImeiInfo()
		if len(allImeis) == 0 {
			t.Logf("  WARNING: No IMEI records found in database")
			t.Logf("  Note: This may occur if data is not persisted or transactions are not committed")
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

	// Clear database before test
	mockService.ClearImeiInfo()
	// EIR_Add_2
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
			provision := ports.ImeiInfoInsert{
				Imei:  tc.imei,
				Color: tc.color,
			}

			body, _ := json.Marshal(provision)
			url := fmt.Sprintf("http://%s/api/v1/insert-imei", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertImei request failed for %s: %v", tc.description, err)
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if tc.expectedError {
				if resp.StatusCode == http.StatusCreated {
					t.Errorf("InsertImei '%s' should have failed but succeeded", tc.description)
				}
			} else {
				if resp.StatusCode != http.StatusCreated {
					t.Errorf("InsertImei '%s' failed: Status=%d, Response=%s",
						tc.description, resp.StatusCode, string(bodyBytes))
				} else {
					t.Logf("✓ InsertImei '%s': IMEI=%s, Color=%s, Status=%d",
						tc.description, tc.imei, tc.color, resp.StatusCode)
				}
			}
		}

		// List all inserted IMEI data after insertions
		t.Logf("\n===== All Inserted IMEI Data =====")
		allImeis := mockService.ListAllImeiInfo()
		if len(allImeis) == 0 {
			t.Logf("  WARNING: No IMEI records found in database")
			t.Logf("  Note: This may occur if data is not persisted or transactions are not committed")
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

	// Clear database before test
	mockService.ClearImeiInfo()
	// EIR_Add_3
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
			provision := ports.ImeiInfoInsert{
				Imei:  tc.imei,
				Color: tc.color,
			}

			body, _ := json.Marshal(provision)
			url := fmt.Sprintf("http://%s/api/v1/insert-imei", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertImei request failed for %s: %v", tc.description, err)
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if tc.expectedError {
				if resp.StatusCode == http.StatusCreated {
					t.Errorf("InsertImei '%s' should have failed but succeeded", tc.description)
				}
			} else {
				if resp.StatusCode != http.StatusCreated {
					t.Errorf("InsertImei '%s' failed: Status=%d, Response=%s",
						tc.description, resp.StatusCode, string(bodyBytes))
				} else {
					t.Logf("✓ InsertImei '%s': IMEI=%s, Color=%s, Status=%d",
						tc.description, tc.imei, tc.color, resp.StatusCode)
				}
			}
		}

		// List all inserted IMEI data after insertions
		t.Logf("\n===== All Inserted IMEI Data =====")
		allImeis := mockService.ListAllImeiInfo()
		if len(allImeis) == 0 {
			t.Logf("  WARNING: No IMEI records found in database")
			t.Logf("  Note: This may occur if data is not persisted or transactions are not committed")
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

	// Clear database before test
	mockService.ClearImeiInfo()
	// EIR_Add_4
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
			provision := ports.ImeiInfoInsert{
				Imei:  tc.imei,
				Color: tc.color,
			}

			body, _ := json.Marshal(provision)
			url := fmt.Sprintf("http://%s/api/v1/insert-imei", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertImei request failed for %s: %v", tc.description, err)
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if tc.expectedError {
				if resp.StatusCode == http.StatusCreated {
					t.Errorf("InsertImei '%s' should have failed but succeeded", tc.description)
				}
			} else {
				if resp.StatusCode != http.StatusCreated {
					t.Errorf("InsertImei '%s' failed: Status=%d, Response=%s",
						tc.description, resp.StatusCode, string(bodyBytes))
				} else {
					t.Logf("✓ InsertImei '%s': IMEI=%s, Color=%s, Status=%d",
						tc.description, tc.imei, tc.color, resp.StatusCode)
				}
			}
		}

		// List all inserted IMEI data after insertions
		t.Logf("\n===== All Inserted IMEI Data =====")
		allImeis := mockService.ListAllImeiInfo()
		if len(allImeis) == 0 {
			t.Logf("  WARNING: No IMEI records found in database")
			t.Logf("  Note: This may occur if data is not persisted or transactions are not committed")
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

	// Clear database before test
	mockService.ClearImeiInfo()
	// EIR_Add_5
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
			{"12345678901234", "g", false, "Valid IMEI - grey"},
			{"12345678901234444", "g", false, "Valid IMEI - grey"},
		}

		for _, tc := range testCases {
			provision := ports.ImeiInfoInsert{
				Imei:  tc.imei,
				Color: tc.color,
			}

			body, _ := json.Marshal(provision)
			url := fmt.Sprintf("http://%s/api/v1/insert-imei", addr)

			resp, err := client.Post(url, "application/json", bytes.NewReader(body))
			if err != nil {
				t.Fatalf("InsertImei request failed for %s: %v", tc.description, err)
			}

			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if tc.expectedError {
				if resp.StatusCode == http.StatusCreated {
					t.Errorf("InsertImei '%s' should have failed but succeeded", tc.description)
				}
			} else {
				if resp.StatusCode != http.StatusCreated {
					t.Errorf("InsertImei '%s' failed: Status=%d, Response=%s",
						tc.description, resp.StatusCode, string(bodyBytes))
				} else {
					t.Logf("✓ InsertImei '%s': IMEI=%s, Color=%s, Status=%d",
						tc.description, tc.imei, tc.color, resp.StatusCode)
				}
			}
		}

		// List all inserted IMEI data after insertions
		t.Logf("\n===== All Inserted IMEI Data =====")
		allImeis := mockService.ListAllImeiInfo()
		if len(allImeis) == 0 {
			t.Logf("  WARNING: No IMEI records found in database")
			t.Logf("  Note: This may occur if data is not persisted or transactions are not committed")
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
	t.Logf("PCAP file saved: %s", pcapFile)
	t.Log("Open in Wireshark with filter: http2 or tcp.port == 8080")
	t.Logf("==================================")
}
