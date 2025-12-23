package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
)

// InMemoryIMEIRepository is an in-memory implementation for testing
type InMemoryIMEIRepository struct {
	mu         sync.RWMutex
	equipment  map[string]*models.Equipment
	nextID     int64
}

// NewInMemoryIMEIRepository creates a new in-memory IMEI repository
func NewInMemoryIMEIRepository() ports.IMEIRepository {
	return &InMemoryIMEIRepository{
		equipment: make(map[string]*models.Equipment),
		nextID:    1,
	}
}

func (r *InMemoryIMEIRepository) GetByIMEI(ctx context.Context, imei string) (*models.Equipment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if equip, ok := r.equipment[imei]; ok {
		return equip, nil
	}
	return nil, fmt.Errorf("equipment not found")
}

func (r *InMemoryIMEIRepository) GetByIMEISV(ctx context.Context, imeisv string) (*models.Equipment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, equip := range r.equipment {
		if equip.IMEISV != nil && *equip.IMEISV == imeisv {
			return equip, nil
		}
	}
	return nil, fmt.Errorf("equipment not found")
}

func (r *InMemoryIMEIRepository) Create(ctx context.Context, equipment *models.Equipment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.equipment[equipment.IMEI]; exists {
		return fmt.Errorf("equipment already exists")
	}

	equipment.ID = r.nextID
	r.nextID++
	r.equipment[equipment.IMEI] = equipment
	return nil
}

func (r *InMemoryIMEIRepository) Update(ctx context.Context, equipment *models.Equipment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.equipment[equipment.IMEI]; !exists {
		return fmt.Errorf("equipment not found")
	}

	r.equipment[equipment.IMEI] = equipment
	return nil
}

func (r *InMemoryIMEIRepository) Delete(ctx context.Context, imei string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.equipment[imei]; !exists {
		return fmt.Errorf("equipment not found")
	}

	delete(r.equipment, imei)
	return nil
}

func (r *InMemoryIMEIRepository) List(ctx context.Context, offset, limit int) ([]*models.Equipment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*models.Equipment, 0)
	count := 0
	for _, equip := range r.equipment {
		if count >= offset {
			result = append(result, equip)
			if len(result) >= limit {
				break
			}
		}
		count++
	}
	return result, nil
}

func (r *InMemoryIMEIRepository) ListByStatus(ctx context.Context, status models.EquipmentStatus, offset, limit int) ([]*models.Equipment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*models.Equipment, 0)
	count := 0
	for _, equip := range r.equipment {
		if equip.Status == status {
			if count >= offset {
				result = append(result, equip)
				if len(result) >= limit {
					break
				}
			}
			count++
		}
	}
	return result, nil
}

func (r *InMemoryIMEIRepository) IncrementCheckCount(ctx context.Context, imei string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if equip, exists := r.equipment[imei]; exists {
		equip.CheckCount++
		return nil
	}
	return fmt.Errorf("equipment not found")
}
