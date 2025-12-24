package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
)

// InMemoryIMEIRepository is an in-memory implementation for testing
type InMemoryIMEIRepository struct {
	mu        sync.RWMutex
	equipment map[string]*models.Equipment
	nextID    int64
	// For IMEI/TAC logic operations
	imeiData map[string]*ports.ImeiInfo
	tacData  map[string]*ports.TacInfo
}

// NewInMemoryIMEIRepository creates a new in-memory IMEI repository
func NewInMemoryIMEIRepository() ports.IMEIRepository {
	return &InMemoryIMEIRepository{
		equipment: make(map[string]*models.Equipment),
		imeiData:  make(map[string]*ports.ImeiInfo),
		tacData:   make(map[string]*ports.TacInfo),
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

// IMEI logic operations
func (r *InMemoryIMEIRepository) LookupImeiInfo(ctx context.Context, startRange string) (*ports.ImeiInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.imeiData[startRange]
	return info, ok
}

func (r *InMemoryIMEIRepository) SaveImeiInfo(ctx context.Context, info *ports.ImeiInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.imeiData[info.StartIMEI] = info
	return nil
}

func (r *InMemoryIMEIRepository) ListAllImeiInfo(ctx context.Context) []ports.ImeiInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ports.ImeiInfo, 0, len(r.imeiData))
	for _, info := range r.imeiData {
		result = append(result, *info)
	}
	return result
}

func (r *InMemoryIMEIRepository) ClearImeiInfo() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.imeiData = make(map[string]*ports.ImeiInfo)
}

// TAC logic operations
func (r *InMemoryIMEIRepository) SaveTacInfo(ctx context.Context, info *ports.TacInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tacData[info.KeyTac] = info
	return nil
}

func (r *InMemoryIMEIRepository) LookupTacInfo(ctx context.Context, key string) (*ports.TacInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, ok := r.tacData[key]
	return info, ok
}

func (r *InMemoryIMEIRepository) PrevTacInfo(ctx context.Context, key string) (*ports.TacInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var prev *ports.TacInfo
	for k, v := range r.tacData {
		if k < key {
			if prev == nil || k > prev.KeyTac {
				prev = v
			}
		}
	}
	if prev == nil {
		return nil, false
	}
	return prev, true
}

func (r *InMemoryIMEIRepository) NextTacInfo(ctx context.Context, key string) (*ports.TacInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var next *ports.TacInfo
	for k, v := range r.tacData {
		logger.Log.Debugw("find next", "key", key, "data", k, "value", v)
		if k > key {
			if next == nil || k < next.KeyTac {
				next = v
			}
		}
	}
	if next == nil {
		return nil, false
	}
	return next, true
}

func (r *InMemoryIMEIRepository) ListAllTacInfo(ctx context.Context) []*ports.TacInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ports.TacInfo, 0, len(r.tacData))
	for _, info := range r.tacData {
		result = append(result, info)
	}
	return result
}

// ClearTacInfo implements ports.IMEIRepository.
func (r *InMemoryIMEIRepository) ClearTacInfo(ctx context.Context) {
	fmt.Println("unimplemented")
}