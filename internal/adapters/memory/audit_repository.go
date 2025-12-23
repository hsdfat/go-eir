package memory

import (
	"context"
	"sync"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
)

// InMemoryAuditRepository is an in-memory implementation for testing
type InMemoryAuditRepository struct {
	mu     sync.RWMutex
	audits []*models.AuditLog
	nextID int64
}

// NewInMemoryAuditRepository creates a new in-memory audit repository
func NewInMemoryAuditRepository() ports.AuditRepository {
	return &InMemoryAuditRepository{
		audits: make([]*models.AuditLog, 0),
		nextID: 1,
	}
}

func (r *InMemoryAuditRepository) LogCheck(ctx context.Context, audit *models.AuditLog) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	audit.ID = r.nextID
	r.nextID++
	r.audits = append(r.audits, audit)
	return nil
}

func (r *InMemoryAuditRepository) GetAuditsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.AuditLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*models.AuditLog, 0)
	count := 0
	for _, audit := range r.audits {
		if audit.IMEI == imei {
			if count >= offset {
				result = append(result, audit)
				if len(result) >= limit {
					break
				}
			}
			count++
		}
	}
	return result, nil
}

func (r *InMemoryAuditRepository) GetAuditsByTimeRange(ctx context.Context, startTime, endTime string, offset, limit int) ([]*models.AuditLog, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Simple implementation - return all for now
	result := make([]*models.AuditLog, 0)
	count := 0
	for _, audit := range r.audits {
		if count >= offset {
			result = append(result, audit)
			if len(result) >= limit {
				break
			}
		}
		count++
	}
	return result, nil
}
