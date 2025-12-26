package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hsdfat8/eir/internal/adapters/diameter"
	httpAdapter "github.com/hsdfat8/eir/internal/adapters/http"
	"github.com/hsdfat8/eir/internal/config"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/service"
)

func main() {
	// Load configuration with environment overrides
	cfg := loadTestConfig()

	// Initialize Mock Repositories (no database required)
	imeiRepo := NewMockIMEIRepository()
	auditRepo := NewMockAuditRepository()

	// Seed test data
	seedTestData(imeiRepo)

	log.Println("✓ Mock data repositories initialized with test data")

	// Initialize EIR service
	eirService := service.NewEIRService(cfg, imeiRepo, auditRepo, nil)

	log.Println("✓ EIR service initialized")

	// Initialize HTTP server
	router := httpAdapter.SetupRouter(eirService)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Start HTTP server
	go func() {
		log.Printf("✓ HTTP server listening on %s", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Initialize Diameter S13 server
	diameterConfig := diameter.ServerConfig{
		ListenAddr:  cfg.Diameter.ListenAddr,
		OriginHost:  cfg.Diameter.OriginHost,
		OriginRealm: cfg.Diameter.OriginRealm,
		ProductName: cfg.Diameter.ProductName,
		VendorID:    cfg.Diameter.VendorID,
	}

	diameterServer := diameter.NewServer(diameterConfig, eirService)

	// Start Diameter server
	if err := diameterServer.Start(); err != nil {
		log.Fatalf("Failed to start Diameter server: %v", err)
	}

	log.Println("✓ Diameter S13 server started")
	log.Println("✓ EIR Core Application fully operational")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Shutdown Diameter server
	if err := diameterServer.Stop(); err != nil {
		log.Printf("Diameter server shutdown error: %v", err)
	}

	log.Println("Servers stopped gracefully")
}

// loadTestConfig loads configuration for test environment
func loadTestConfig() *config.Config {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:         getEnv("SERVER_HOST", "0.0.0.0"),
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		Diameter: config.DiameterConfig{
			ListenAddr:  getEnv("DIAMETER_LISTEN_ADDR", "0.0.0.0:3868"),
			OriginHost:  getEnv("DIAMETER_ORIGIN_HOST", "eir.test.epc.mnc001.mcc001.3gppnetwork.org"),
			OriginRealm: getEnv("DIAMETER_ORIGIN_REALM", "test.epc.mnc001.mcc001.3gppnetwork.org"),
			ProductName: getEnv("DIAMETER_PRODUCT_NAME", "EIR-Core/1.0"),
			VendorID:    10415,
		},
	}

	return cfg
}

// seedTestData populates mock repository with test IMEIs
func seedTestData(repo *MockIMEIRepository) {
	testData := []struct {
		imei   string
		status models.EquipmentStatus
	}{
		// Whitelisted devices
		{"123456789012345", models.EquipmentStatusWhitelisted},
		{"111111111111111", models.EquipmentStatusWhitelisted},
		{"222222222222222", models.EquipmentStatusWhitelisted},
		{"333333333333333", models.EquipmentStatusWhitelisted},
		{"444444444444444", models.EquipmentStatusWhitelisted},

		// Greylisted devices
		{"555555555555555", models.EquipmentStatusGreylisted},
		{"666666666666666", models.EquipmentStatusGreylisted},
		{"777777777777777", models.EquipmentStatusGreylisted},

		// Blacklisted devices
		{"999999999999999", models.EquipmentStatusBlacklisted},
		{"888888888888888", models.EquipmentStatusBlacklisted},
		{"000000000000000", models.EquipmentStatusBlacklisted},
	}

	now := time.Now()
	for _, td := range testData {
		repo.data[td.imei] = &models.Equipment{
			IMEI:          td.imei,
			Status:        td.status,
			CheckCount:    0,
			LastCheckTime: nil,
			LastUpdated:   now,
		}
	}

	log.Printf("Seeded %d test IMEI records", len(testData))
}

// MockIMEIRepository implements ports.IMEIRepository using in-memory storage
type MockIMEIRepository struct {
	mu   sync.RWMutex
	data map[string]*models.Equipment
}

// NewMockIMEIRepository creates a new mock IMEI repository
func NewMockIMEIRepository() *MockIMEIRepository {
	return &MockIMEIRepository{
		data: make(map[string]*models.Equipment),
	}
}

// GetByIMEI retrieves equipment by IMEI
func (m *MockIMEIRepository) GetByIMEI(ctx context.Context, imei string) (*models.Equipment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	equipment, exists := m.data[imei]
	if !exists {
		return nil, service.ErrEquipmentNotFound
	}

	// Return a copy to prevent external modification
	equipmentCopy := *equipment
	return &equipmentCopy, nil
}

// GetByIMEISV retrieves equipment by IMEISV
func (m *MockIMEIRepository) GetByIMEISV(ctx context.Context, imeisv string) (*models.Equipment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// For mock implementation, we search through all equipment
	for _, equipment := range m.data {
		if equipment.IMEISV != nil && *equipment.IMEISV == imeisv {
			equipmentCopy := *equipment
			return &equipmentCopy, nil
		}
	}

	return nil, service.ErrEquipmentNotFound
}

// Create creates a new equipment record
func (m *MockIMEIRepository) Create(ctx context.Context, equipment *models.Equipment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.data[equipment.IMEI]; exists {
		return service.ErrInvalidRequest // Equipment already exists
	}

	equipment.LastUpdated = time.Now()
	m.data[equipment.IMEI] = equipment

	return nil
}

// Update updates an existing equipment record
func (m *MockIMEIRepository) Update(ctx context.Context, equipment *models.Equipment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.data[equipment.IMEI]; !exists {
		return service.ErrEquipmentNotFound
	}

	equipment.LastUpdated = time.Now()
	m.data[equipment.IMEI] = equipment

	return nil
}

// Delete deletes an equipment record
func (m *MockIMEIRepository) Delete(ctx context.Context, imei string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.data[imei]; !exists {
		return service.ErrEquipmentNotFound
	}

	delete(m.data, imei)

	return nil
}

// List lists equipment records with pagination
func (m *MockIMEIRepository) List(ctx context.Context, offset, limit int) ([]*models.Equipment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var equipments []*models.Equipment
	for _, equipment := range m.data {
		equipmentCopy := *equipment
		equipments = append(equipments, &equipmentCopy)
	}

	// Simple pagination
	start := offset
	if start > len(equipments) {
		start = len(equipments)
	}

	end := start + limit
	if end > len(equipments) {
		end = len(equipments)
	}

	return equipments[start:end], nil
}

// ListByStatus lists equipment records by status with pagination
func (m *MockIMEIRepository) ListByStatus(ctx context.Context, status models.EquipmentStatus, offset, limit int) ([]*models.Equipment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var equipments []*models.Equipment
	for _, equipment := range m.data {
		if equipment.Status == status {
			equipmentCopy := *equipment
			equipments = append(equipments, &equipmentCopy)
		}
	}

	// Simple pagination
	start := offset
	if start > len(equipments) {
		start = len(equipments)
	}

	end := start + limit
	if end > len(equipments) {
		end = len(equipments)
	}

	return equipments[start:end], nil
}

// IncrementCheckCount increments the check counter for an IMEI
func (m *MockIMEIRepository) IncrementCheckCount(ctx context.Context, imei string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	equipment, exists := m.data[imei]
	if !exists {
		return service.ErrEquipmentNotFound
	}

	equipment.CheckCount++
	now := time.Now()
	equipment.LastCheckTime = &now
	equipment.LastUpdated = now

	return nil
}

// MockAuditRepository implements ports.AuditRepository using in-memory storage
type MockAuditRepository struct {
	mu   sync.RWMutex
	logs []*models.AuditLog
}

// NewMockAuditRepository creates a new mock audit repository
func NewMockAuditRepository() *MockAuditRepository {
	return &MockAuditRepository{
		logs: make([]*models.AuditLog, 0),
	}
}

// LogCheck logs an equipment check
func (m *MockAuditRepository) LogCheck(ctx context.Context, audit *models.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	audit.CheckTime = time.Now()
	m.logs = append(m.logs, audit)

	log.Printf("Audit: IMEI=%s, Status=%s, Source=%s", audit.IMEI, audit.Status, audit.RequestSource)

	return nil
}

// GetAuditsByIMEI retrieves audit logs for a specific IMEI
func (m *MockAuditRepository) GetAuditsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.AuditLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filteredLogs []*models.AuditLog
	for _, log := range m.logs {
		if log.IMEI == imei {
			filteredLogs = append(filteredLogs, log)
		}
	}

	// Simple pagination
	start := offset
	if start > len(filteredLogs) {
		start = len(filteredLogs)
	}

	end := start + limit
	if end > len(filteredLogs) {
		end = len(filteredLogs)
	}

	return filteredLogs[start:end], nil
}

// GetAuditsByTimeRange retrieves audit logs within a time range
func (m *MockAuditRepository) GetAuditsByTimeRange(ctx context.Context, startTime, endTime string, offset, limit int) ([]*models.AuditLog, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Parse time strings
	start, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil, err
	}

	end, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil, err
	}

	var filteredLogs []*models.AuditLog
	for _, log := range m.logs {
		if log.CheckTime.After(start) && log.CheckTime.Before(end) {
			filteredLogs = append(filteredLogs, log)
		}
	}

	// Simple pagination
	startIdx := offset
	if startIdx > len(filteredLogs) {
		startIdx = len(filteredLogs)
	}

	endIdx := startIdx + limit
	if endIdx > len(filteredLogs) {
		endIdx = len(filteredLogs)
	}

	return filteredLogs[startIdx:endIdx], nil
}

// Count returns the total number of audit logs
func (m *MockAuditRepository) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.logs)
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
