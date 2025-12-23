package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
)

// snapshotRepository implements the SnapshotRepository interface using PostgreSQL
type snapshotRepository struct {
	db dbExecutor
}

// NewSnapshotRepository creates a new PostgreSQL snapshot repository
func NewSnapshotRepository(db dbExecutor) ports.SnapshotRepository {
	return &snapshotRepository{db: db}
}

// CreateSnapshot creates a point-in-time snapshot of equipment
func (r *snapshotRepository) CreateSnapshot(ctx context.Context, snapshot *models.EquipmentSnapshot) error {
	query := `
		INSERT INTO equipment_snapshots (
			equipment_id, imei, snapshot_time, status, reason,
			check_count, metadata, created_by, snapshot_type
		) VALUES (
			:equipment_id, :imei, :snapshot_time, :status, :reason,
			:check_count, :metadata, :created_by, :snapshot_type
		) RETURNING id
	`

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	err = stmt.GetContext(ctx, &snapshot.ID, snapshot)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	return nil
}

// GetSnapshotsByIMEI retrieves snapshots for a specific IMEI
func (r *snapshotRepository) GetSnapshotsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.EquipmentSnapshot, error) {
	query := `
		SELECT id, equipment_id, imei, snapshot_time, status, reason,
		       check_count, metadata, created_by, snapshot_type
		FROM equipment_snapshots
		WHERE imei = $1
		ORDER BY snapshot_time DESC
		LIMIT $2 OFFSET $3
	`

	var snapshots []*models.EquipmentSnapshot
	err := r.db.SelectContext(ctx, &snapshots, query, imei, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots by IMEI: %w", err)
	}

	return snapshots, nil
}

// GetSnapshotByID retrieves a specific snapshot
func (r *snapshotRepository) GetSnapshotByID(ctx context.Context, id int64) (*models.EquipmentSnapshot, error) {
	query := `
		SELECT id, equipment_id, imei, snapshot_time, status, reason,
		       check_count, metadata, created_by, snapshot_type
		FROM equipment_snapshots
		WHERE id = $1
	`

	var snapshot models.EquipmentSnapshot
	err := r.db.GetContext(ctx, &snapshot, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	return &snapshot, nil
}

// DeleteOldSnapshots removes snapshots older than the specified date
func (r *snapshotRepository) DeleteOldSnapshots(ctx context.Context, before time.Time) (int64, error) {
	query := `DELETE FROM equipment_snapshots WHERE snapshot_time < $1`

	result, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old snapshots: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}
