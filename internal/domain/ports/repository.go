package ports

import (
	"context"

	"github.com/hsdfat8/eir/internal/domain/models"
)

// IMEIRepository defines the interface for IMEI data access
// This is a port owned by the domain layer
type IMEIRepository interface {
	// GetByIMEI retrieves equipment information by IMEI
	GetByIMEI(ctx context.Context, imei string) (*models.Equipment, error)

	// GetByIMEISV retrieves equipment information by IMEISV
	GetByIMEISV(ctx context.Context, imeisv string) (*models.Equipment, error)

	// Create adds a new equipment record
	Create(ctx context.Context, equipment *models.Equipment) error

	// Update updates an existing equipment record
	Update(ctx context.Context, equipment *models.Equipment) error

	// Delete removes an equipment record by IMEI
	Delete(ctx context.Context, imei string) error

	// List retrieves equipment records with pagination
	List(ctx context.Context, offset, limit int) ([]*models.Equipment, error)

	// ListByStatus retrieves equipment records by status
	ListByStatus(ctx context.Context, status models.EquipmentStatus, offset, limit int) ([]*models.Equipment, error)

	// IncrementCheckCount atomically increments the check counter and updates last check time
	IncrementCheckCount(ctx context.Context, imei string) error
}

// AuditRepository defines the interface for audit logging
type AuditRepository interface {
	// LogCheck records an equipment check operation
	LogCheck(ctx context.Context, audit *models.AuditLog) error

	// GetAuditsByIMEI retrieves audit logs for a specific IMEI
	GetAuditsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.AuditLog, error)

	// GetAuditsByTimeRange retrieves audit logs within a time range
	GetAuditsByTimeRange(ctx context.Context, startTime, endTime string, offset, limit int) ([]*models.AuditLog, error)
}

// CacheRepository defines the interface for caching (optional)
type CacheRepository interface {
	// Get retrieves equipment data from cache
	Get(ctx context.Context, imei string) (*models.Equipment, error)

	// Set stores equipment data in cache
	Set(ctx context.Context, imei string, equipment *models.Equipment, ttlSeconds int) error

	// Delete removes equipment data from cache
	Delete(ctx context.Context, imei string) error

	// Exists checks if a key exists in cache
	Exists(ctx context.Context, imei string) (bool, error)
}
