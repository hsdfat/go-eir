package mocks

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hsdfat8/eir/internal/domain/models"
)

// MockIMEIRepository is a mock implementation of IMEIRepository for testing
type MockIMEIRepository struct {
	mu            sync.RWMutex
	equipment     map[string]*models.Equipment // keyed by IMEI
	nextID        int64

	// Function overrides for testing
	GetByIMEIFunc          func(ctx context.Context, imei string) (*models.Equipment, error)
	GetByIMEISVFunc        func(ctx context.Context, imeisv string) (*models.Equipment, error)
	CreateFunc             func(ctx context.Context, equipment *models.Equipment) error
	UpdateFunc             func(ctx context.Context, equipment *models.Equipment) error
	DeleteFunc             func(ctx context.Context, imei string) error
	ListFunc               func(ctx context.Context, offset, limit int) ([]*models.Equipment, error)
	ListByStatusFunc       func(ctx context.Context, status models.EquipmentStatus, offset, limit int) ([]*models.Equipment, error)
	IncrementCheckCountFunc func(ctx context.Context, imei string) error
}

// NewMockIMEIRepository creates a new mock IMEI repository
func NewMockIMEIRepository() *MockIMEIRepository {
	return &MockIMEIRepository{
		equipment: make(map[string]*models.Equipment),
		nextID:    1,
	}
}

// GetByIMEI retrieves equipment by IMEI
func (m *MockIMEIRepository) GetByIMEI(ctx context.Context, imei string) (*models.Equipment, error) {
	if m.GetByIMEIFunc != nil {
		return m.GetByIMEIFunc(ctx, imei)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	equipment, exists := m.equipment[imei]
	if !exists {
		return nil, errors.New("equipment not found")
	}

	// Return a copy to prevent external modification
	return m.copyEquipment(equipment), nil
}

// GetByIMEISV retrieves equipment by IMEISV
func (m *MockIMEIRepository) GetByIMEISV(ctx context.Context, imeisv string) (*models.Equipment, error) {
	if m.GetByIMEISVFunc != nil {
		return m.GetByIMEISVFunc(ctx, imeisv)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, equipment := range m.equipment {
		if equipment.IMEISV != nil && *equipment.IMEISV == imeisv {
			return m.copyEquipment(equipment), nil
		}
	}

	return nil, errors.New("equipment not found")
}

// Create adds a new equipment record
func (m *MockIMEIRepository) Create(ctx context.Context, equipment *models.Equipment) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, equipment)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already exists
	if _, exists := m.equipment[equipment.IMEI]; exists {
		return errors.New("equipment already exists")
	}

	// Assign ID
	equipment.ID = m.nextID
	m.nextID++

	// Set timestamps
	equipment.LastUpdated = time.Now()

	// Store copy
	m.equipment[equipment.IMEI] = m.copyEquipment(equipment)

	return nil
}

// Update updates an existing equipment record
func (m *MockIMEIRepository) Update(ctx context.Context, equipment *models.Equipment) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, equipment)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.equipment[equipment.IMEI]
	if !exists {
		return errors.New("equipment not found")
	}

	// Preserve ID
	equipment.ID = existing.ID

	// Update timestamp
	equipment.LastUpdated = time.Now()

	// Store updated copy
	m.equipment[equipment.IMEI] = m.copyEquipment(equipment)

	return nil
}

// Delete removes an equipment record
func (m *MockIMEIRepository) Delete(ctx context.Context, imei string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, imei)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.equipment[imei]; !exists {
		return errors.New("equipment not found")
	}

	delete(m.equipment, imei)
	return nil
}

// List retrieves equipment with pagination
func (m *MockIMEIRepository) List(ctx context.Context, offset, limit int) ([]*models.Equipment, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, offset, limit)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models.Equipment
	for _, equipment := range m.equipment {
		result = append(result, m.copyEquipment(equipment))
	}

	// Apply pagination
	if offset >= len(result) {
		return []*models.Equipment{}, nil
	}

	end := offset + limit
	if end > len(result) {
		end = len(result)
	}

	return result[offset:end], nil
}

// ListByStatus retrieves equipment by status
func (m *MockIMEIRepository) ListByStatus(ctx context.Context, status models.EquipmentStatus, offset, limit int) ([]*models.Equipment, error) {
	if m.ListByStatusFunc != nil {
		return m.ListByStatusFunc(ctx, status, offset, limit)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models.Equipment
	for _, equipment := range m.equipment {
		if equipment.Status == status {
			result = append(result, m.copyEquipment(equipment))
		}
	}

	// Apply pagination
	if offset >= len(result) {
		return []*models.Equipment{}, nil
	}

	end := offset + limit
	if end > len(result) {
		end = len(result)
	}

	return result[offset:end], nil
}

// IncrementCheckCount increments the check counter
func (m *MockIMEIRepository) IncrementCheckCount(ctx context.Context, imei string) error {
	if m.IncrementCheckCountFunc != nil {
		return m.IncrementCheckCountFunc(ctx, imei)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	equipment, exists := m.equipment[imei]
	if !exists {
		return errors.New("equipment not found")
	}

	equipment.CheckCount++
	now := time.Now()
	equipment.LastCheckTime = &now

	return nil
}

// Helper methods

// AddEquipment adds equipment directly (for test setup)
func (m *MockIMEIRepository) AddEquipment(equipment *models.Equipment) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if equipment.ID == 0 {
		equipment.ID = m.nextID
		m.nextID++
	}

	m.equipment[equipment.IMEI] = m.copyEquipment(equipment)
}

// Clear removes all equipment (for test cleanup)
func (m *MockIMEIRepository) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.equipment = make(map[string]*models.Equipment)
	m.nextID = 1
}

// Count returns the number of equipment records
func (m *MockIMEIRepository) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.equipment)
}

// copyEquipment creates a deep copy of equipment
func (m *MockIMEIRepository) copyEquipment(e *models.Equipment) *models.Equipment {
	if e == nil {
		return nil
	}

	copy := &models.Equipment{
		ID:               e.ID,
		IMEI:             e.IMEI,
		Status:           e.Status,
		LastUpdated:      e.LastUpdated,
		CheckCount:       e.CheckCount,
		AddedBy:          e.AddedBy,
	}

	if e.IMEISV != nil {
		val := *e.IMEISV
		copy.IMEISV = &val
	}

	if e.Reason != nil {
		val := *e.Reason
		copy.Reason = &val
	}

	if e.LastCheckTime != nil {
		val := *e.LastCheckTime
		copy.LastCheckTime = &val
	}

	if e.Metadata != nil {
		val := *e.Metadata
		copy.Metadata = &val
	}

	if e.ManufacturerTAC != nil {
		val := *e.ManufacturerTAC
		copy.ManufacturerTAC = &val
	}

	if e.ManufacturerName != nil {
		val := *e.ManufacturerName
		copy.ManufacturerName = &val
	}

	return copy
}
