package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/hsdfat8/eir/internal/domain/models"
)

// MockAuditRepository is a mock implementation of AuditRepository for testing
type MockAuditRepository struct {
	mu        sync.RWMutex
	auditLogs []*models.AuditLog
	nextID    int64

	// Function overrides for testing
	LogCheckFunc            func(ctx context.Context, audit *models.AuditLog) error
	GetAuditsByIMEIFunc     func(ctx context.Context, imei string, offset, limit int) ([]*models.AuditLog, error)
	GetAuditsByTimeRangeFunc func(ctx context.Context, startTime, endTime string, offset, limit int) ([]*models.AuditLog, error)
}

// NewMockAuditRepository creates a new mock audit repository
func NewMockAuditRepository() *MockAuditRepository {
	return &MockAuditRepository{
		auditLogs: make([]*models.AuditLog, 0),
		nextID:    1,
	}
}

// LogCheck records an equipment check
func (m *MockAuditRepository) LogCheck(ctx context.Context, audit *models.AuditLog) error {
	if m.LogCheckFunc != nil {
		return m.LogCheckFunc(ctx, audit)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Assign ID
	audit.ID = m.nextID
	m.nextID++

	// Set check time if not set
	if audit.CheckTime.IsZero() {
		audit.CheckTime = time.Now()
	}

	// Store copy
	m.auditLogs = append(m.auditLogs, m.copyAuditLog(audit))

	return nil
}

// GetAuditsByIMEI retrieves audit logs for a specific IMEI
func (m *MockAuditRepository) GetAuditsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.AuditLog, error) {
	if m.GetAuditsByIMEIFunc != nil {
		return m.GetAuditsByIMEIFunc(ctx, imei, offset, limit)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models.AuditLog
	for _, log := range m.auditLogs {
		if log.IMEI == imei {
			result = append(result, m.copyAuditLog(log))
		}
	}

	// Apply pagination
	if offset >= len(result) {
		return []*models.AuditLog{}, nil
	}

	end := offset + limit
	if end > len(result) {
		end = len(result)
	}

	return result[offset:end], nil
}

// GetAuditsByTimeRange retrieves audit logs within a time range
func (m *MockAuditRepository) GetAuditsByTimeRange(ctx context.Context, startTime, endTime string, offset, limit int) ([]*models.AuditLog, error) {
	if m.GetAuditsByTimeRangeFunc != nil {
		return m.GetAuditsByTimeRangeFunc(ctx, startTime, endTime, offset, limit)
	}

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

	var result []*models.AuditLog
	for _, log := range m.auditLogs {
		if log.CheckTime.After(start) && log.CheckTime.Before(end) {
			result = append(result, m.copyAuditLog(log))
		}
	}

	// Apply pagination
	if offset >= len(result) {
		return []*models.AuditLog{}, nil
	}

	endIndex := offset + limit
	if endIndex > len(result) {
		endIndex = len(result)
	}

	return result[offset:endIndex], nil
}

// Helper methods

// GetAllLogs returns all audit logs (for testing)
func (m *MockAuditRepository) GetAllLogs() []*models.AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.AuditLog, len(m.auditLogs))
	for i, log := range m.auditLogs {
		result[i] = m.copyAuditLog(log)
	}

	return result
}

// Clear removes all audit logs (for test cleanup)
func (m *MockAuditRepository) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.auditLogs = make([]*models.AuditLog, 0)
	m.nextID = 1
}

// Count returns the number of audit logs
func (m *MockAuditRepository) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.auditLogs)
}

// copyAuditLog creates a deep copy of an audit log
func (m *MockAuditRepository) copyAuditLog(a *models.AuditLog) *models.AuditLog {
	if a == nil {
		return nil
	}

	copy := &models.AuditLog{
		ID:            a.ID,
		IMEI:          a.IMEI,
		Status:        a.Status,
		CheckTime:     a.CheckTime,
		RequestSource: a.RequestSource,
	}

	if a.IMEISV != nil {
		val := *a.IMEISV
		copy.IMEISV = &val
	}

	if a.OriginHost != nil {
		val := *a.OriginHost
		copy.OriginHost = &val
	}

	if a.OriginRealm != nil {
		val := *a.OriginRealm
		copy.OriginRealm = &val
	}

	if a.UserName != nil {
		val := *a.UserName
		copy.UserName = &val
	}

	if a.SUPI != nil {
		val := *a.SUPI
		copy.SUPI = &val
	}

	if a.GPSI != nil {
		val := *a.GPSI
		copy.GPSI = &val
	}

	if a.SessionID != nil {
		val := *a.SessionID
		copy.SessionID = &val
	}

	if a.ResultCode != nil {
		val := *a.ResultCode
		copy.ResultCode = &val
	}

	return copy
}
