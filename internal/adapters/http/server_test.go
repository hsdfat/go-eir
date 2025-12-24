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
	"testing"
	"time"

	"github.com/hsdfat8/eir/internal/adapters/memory"
	"github.com/hsdfat8/eir/internal/adapters/testutil"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
	legacyModels "github.com/hsdfat8/eir/models"
	"github.com/hsdfat8/eir/pkg/logic"
	"golang.org/x/net/http2"
)

// mockEIRService is a mock implementation of EIRService for testing
type mockEIRService struct {
	imeiRepo      ports.IMEIRepository
	insertedTacs  []ports.TacInfo
	insertedImeis []string
}

// newMockEIRService creates a properly initialized mock service
func newMockEIRService() *mockEIRService {
	return &mockEIRService{
		imeiRepo: memory.NewInMemoryIMEIRepository(),
	}
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

	// Track inserted TACs
	if m.insertedTacs == nil {
		m.insertedTacs = []ports.TacInfo{}
	}
	if result.TacInfo.KeyTac != "" {
		logger.Log.Debugw("insert new tac to memory", "tac", result.TacInfo)
		m.insertedTacs = append(m.insertedTacs, ports.TacInfo{
			KeyTac:        result.TacInfo.KeyTac,
			StartRangeTac: result.TacInfo.StartRangeTac,
			EndRangeTac:   result.TacInfo.EndRangeTac,
			Color:         result.TacInfo.Color,
			PrevLink:      result.TacInfo.PrevLink,
		})
	}

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
	return m.imeiRepo.ListAllTacInfo()
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

	mockService := newMockEIRService()
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

	mockService := newMockEIRService()
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

	mockService := newMockEIRService()
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

	mockService := newMockEIRService()
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

	mockService := newMockEIRService()
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

	mockService := newMockEIRService()
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

	// EIR_10
	t.Run("ValidIMEICheck_EIR_10", func(t *testing.T) {
		imei := "9"
		url := fmt.Sprintf("http://%s/api/v1/check-imei/%s", addr, imei)

		resp, err := client.Get(url)
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

		t.Logf("Valid IMEI check passed: IMEI=%s, Status=%s", imei, result.Status)
	})

	// EIR_11
	t.Run("ValidIMEICheck_EIR_11", func(t *testing.T) {
		imei := "912"
		url := fmt.Sprintf("http://%s/api/v1/check-imei/%s", addr, imei)

		resp, err := client.Get(url)
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

		t.Logf("Valid IMEI check passed: IMEI=%s, Status=%s", imei, result.Status)
	})

	// EIR_12
	t.Run("ValidIMEICheck_EIR_12", func(t *testing.T) {
		imei := "9123456789012"
		url := fmt.Sprintf("http://%s/api/v1/check-imei/%s", addr, imei)

		resp, err := client.Get(url)
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

		t.Logf("Valid IMEI check passed: IMEI=%s, Status=%s", imei, result.Status)
	})

	// EIR_13
	t.Run("ValidIMEICheck_EIR_13", func(t *testing.T) {
		imei := "91234567895264"
		url := fmt.Sprintf("http://%s/api/v1/check-imei/%s", addr, imei)

		resp, err := client.Get(url)
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
			t.Errorf("Expected status ok, got %s", result.Status)
		}

		t.Logf("Valid IMEI check passed: IMEI=%s, Status=%s", imei, result.Status)
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

	mockService := newMockEIRService()
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

	// // Eir_Add_63
	// t.Run("Eir_Add_63", func(t *testing.T) {
	// 	testCases := []struct {
	// 		keyTac        string
	// 		startRangeTac string
	// 		endRangeTac   string
	// 		color         string
	// 	}{
	// 		{"1134567890123456-1134567890123456", "1134567890123456", "1134567890123456", "white"},
	// 		{"2-2", "2", "2", "2"},
	// 	}

	// 	for _, tc := range testCases {
	// 		tacInfo := ports.TacInfo{
	// 			KeyTac:        tc.keyTac,
	// 			StartRangeTac: tc.startRangeTac,
	// 			EndRangeTac:   tc.endRangeTac,
	// 			Color:         tc.color,
	// 			PrevLink:      nil,
	// 		}

	// 		body, _ := json.Marshal(tacInfo)
	// 		url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

	// 		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	// 		if err != nil {
	// 			t.Errorf("InsertTac request failed for %s: %v", tc.keyTac, err)
	// 			continue
	// 		}

	// 		bodyBytes, _ := io.ReadAll(resp.Body)
	// 		resp.Body.Close()

	// 		t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
	// 			tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
	// 	}

	// 	// List all data that was inserted during the test
	// 	t.Logf("\n========== All Inserted TAC Data ==========")
	// 	t.Logf("Total TAC records inserted: %d", len(mockService.insertedTacs))
	// 	for i, tac := range mockService.insertedTacs {
	// 		t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s",
	// 			i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color)
	// 	}
	// 	t.Logf("==========================================\n")
	// })

	// // Eir_Add_64
	// t.Run("Eir_Add_64", func(t *testing.T) {
	// 	testCases := []struct {
	// 		keyTac        string
	// 		startRangeTac string
	// 		endRangeTac   string
	// 		color         string
	// 	}{
	// 		{"111-1222", "111", "1222", "white"},
	// 		{"1223-13", "1223", "13", "white"},
	// 		{"123456789012345-123456789012349", "123456789012345", "123456789012349", "white"},
	// 		{"1-9", "1", "9", "white"},
	// 		{"4-4234567890123456", "4", "4234567890123456", "white"},
	// 		{"1234567890123456-1234567890123457", "1234567890123456", "1234567890123457", "white"},
	// 	}

	// 	for _, tc := range testCases {
	// 		tacInfo := ports.TacInfo{
	// 			KeyTac:        tc.keyTac,
	// 			StartRangeTac: tc.startRangeTac,
	// 			EndRangeTac:   tc.endRangeTac,
	// 			Color:         tc.color,
	// 			PrevLink:      nil,
	// 		}

	// 		body, _ := json.Marshal(tacInfo)
	// 		url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

	// 		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	// 		if err != nil {
	// 			t.Errorf("InsertTac request failed for %s: %v", tc.keyTac, err)
	// 			continue
	// 		}

	// 		bodyBytes, _ := io.ReadAll(resp.Body)
	// 		resp.Body.Close()

	// 		t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
	// 			tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
	// 	}

	// 	// List all data that was inserted during the test
	// 	t.Logf("\n========== All Inserted TAC Data ==========")
	// 	t.Logf("Total TAC records inserted: %d", len(mockService.insertedTacs))
	// 	for i, tac := range mockService.insertedTacs {
	// 		t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s, PrevLink: %s",
	// 			i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, *tac.PrevLink)
	// 	}
	// 	t.Logf("==========================================\n")
	// })

	// // Eir_Add_65
	// t.Run("Eir_Add_65", func(t *testing.T) {
	// 	testCases := []struct {
	// 		keyTac        string
	// 		startRangeTac string
	// 		endRangeTac   string
	// 		color         string
	// 	}{
	// 		{"133-133", "133", "133", "white"},
	// 		{"132-132", "132", "132", "white"},
	// 		{"134-134", "134", "134", "white"},
	// 	}

	// 	for _, tc := range testCases {
	// 		tacInfo := ports.TacInfo{
	// 			KeyTac:        tc.keyTac,
	// 			StartRangeTac: tc.startRangeTac,
	// 			EndRangeTac:   tc.endRangeTac,
	// 			Color:         tc.color,
	// 			PrevLink:      nil,
	// 		}

	// 		body, _ := json.Marshal(tacInfo)
	// 		url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

	// 		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	// 		if err != nil {
	// 			t.Errorf("InsertTac request failed for %s: %v", tc.keyTac, err)
	// 			continue
	// 		}

	// 		bodyBytes, _ := io.ReadAll(resp.Body)
	// 		resp.Body.Close()

	// 		t.Logf("TAC insertion '%s': TAC=%s-%s, Status=%d, Response=%s",
	// 			tc.keyTac, tc.startRangeTac, tc.endRangeTac, resp.StatusCode, string(bodyBytes))
	// 	}

	// 	// List all data that was inserted during the test
	// 	t.Logf("\n========== All Inserted TAC Data ==========")
	// 	t.Logf("Total TAC records inserted: %d", len(mockService.insertedTacs))
	// 	for i, tac := range mockService.insertedTacs {
	// 		t.Logf("  [%d] KeyTac: %s, Range: %s-%s, Color: %s, PrevLink: %s",
	// 			i+1, tac.KeyTac, tac.StartRangeTac, tac.EndRangeTac, tac.Color, *tac.PrevLink)
	// 	}
	// 	t.Logf("==========================================\n")
	// })

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
				t.Errorf("InsertTac request failed for %s: %v", tc.keyTac, err)
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

	// Test Case 6: Invalid JSON payload
	// t.Run("InvalidJSONPayload", func(t *testing.T) {
	// 	invalidJSON := []byte(`{"KeyTac": "invalid json structure"`)
	// 	url := fmt.Sprintf("http://%s/api/v1/insert-tac", addr)

	// 	resp, err := client.Post(url, "application/json", bytes.NewReader(invalidJSON))
	// 	if err != nil {
	// 		t.Fatalf("Request failed: %v", err)
	// 	}
	// 	defer resp.Body.Close()

	// 	if resp.StatusCode != http.StatusBadRequest {
	// 		t.Errorf("Expected status 400 for invalid JSON, got %d", resp.StatusCode)
	// 	}

	// 	bodyBytes, _ := io.ReadAll(resp.Body)
	// 	t.Logf("Invalid JSON test: Status=%d, Response=%s", resp.StatusCode, string(bodyBytes))
	// })

	t.Logf("PCAP file saved: %s (contains all InsertTac HTTP/2 test traffic)", pcapFile)
	t.Log("Open in Wireshark with filter: http2 or tcp.port == 8080")
}
