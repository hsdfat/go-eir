package ports

import (
	"context"
	"time"

	"github.com/hsdfat8/eir/internal/domain/models"
)

// HistoryRepository defines the interface for equipment change history tracking
type HistoryRepository interface {
	// RecordChange records a change to equipment status or metadata
	RecordChange(ctx context.Context, history *models.EquipmentHistory) error

	// GetHistoryByIMEI retrieves change history for a specific IMEI
	GetHistoryByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.EquipmentHistory, error)

	// GetHistoryByTimeRange retrieves change history within a time range
	GetHistoryByTimeRange(ctx context.Context, startTime, endTime time.Time, offset, limit int) ([]*models.EquipmentHistory, error)

	// GetHistoryByChangeType retrieves history filtered by change type
	GetHistoryByChangeType(ctx context.Context, changeType models.ChangeType, offset, limit int) ([]*models.EquipmentHistory, error)
}

// SnapshotRepository defines the interface for equipment snapshots
type SnapshotRepository interface {
	// CreateSnapshot creates a point-in-time snapshot of equipment
	CreateSnapshot(ctx context.Context, snapshot *models.EquipmentSnapshot) error

	// GetSnapshotsByIMEI retrieves snapshots for a specific IMEI
	GetSnapshotsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.EquipmentSnapshot, error)

	// GetSnapshotByID retrieves a specific snapshot
	GetSnapshotByID(ctx context.Context, id int64) (*models.EquipmentSnapshot, error)

	// DeleteOldSnapshots removes snapshots older than the specified date
	DeleteOldSnapshots(ctx context.Context, before time.Time) (int64, error)
}

// ExtendedAuditRepository extends AuditRepository with additional capabilities
type ExtendedAuditRepository interface {
	AuditRepository

	// LogCheckExtended records an extended equipment check with additional metadata
	LogCheckExtended(ctx context.Context, audit *models.AuditLogExtended) error

	// GetExtendedAuditsByIMEI retrieves extended audit logs for a specific IMEI
	GetExtendedAuditsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.AuditLogExtended, error)

	// GetAuditsByRequestSource retrieves audits filtered by request source
	GetAuditsByRequestSource(ctx context.Context, requestSource string, offset, limit int) ([]*models.AuditLog, error)

	// GetAuditStatistics retrieves aggregated audit statistics
	GetAuditStatistics(ctx context.Context, startTime, endTime time.Time) (map[string]interface{}, error)
}
