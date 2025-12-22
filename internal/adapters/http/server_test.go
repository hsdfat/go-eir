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

	"github.com/hsdfat8/eir/internal/adapters/testutil"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"golang.org/x/net/http2"
)

// mockEIRService is a mock implementation of EIRService for testing
type mockEIRService struct{}

func (m *mockEIRService) CheckImei(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckImeiResult, error) {
	return &ports.CheckImeiResult{
		Status: "ok",
		IMEI:   imei,
		Color:  "w",
	}, nil
}

func (m *mockEIRService) CheckTac(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckTacResult, error) {
	return &ports.CheckTacResult{
		Status: "ok",
		IMEI:   imei,
		Color:  "white",
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

// TestServerHTTP1Basic tests basic HTTP/1.1 server
func TestServerHTTP1Basic(t *testing.T) {
	config := ServerConfig{
		ListenAddr: "127.0.0.1:8080", // Standard HTTP port
	}

	mockService := &mockEIRService{}
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
		EnableH2C:  false,             // Use HTTP/1.1 for easier Wireshark decoding
	}

	mockService := &mockEIRService{}
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

	mockService := &mockEIRService{}
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

	mockService := &mockEIRService{}
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

	mockService := &mockEIRService{}
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
