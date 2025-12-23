package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
)

// historyRepository implements the HistoryRepository interface using PostgreSQL
type historyRepository struct {
	db dbExecutor
}

// NewHistoryRepository creates a new PostgreSQL history repository
func NewHistoryRepository(db dbExecutor) ports.HistoryRepository {
	return &historyRepository{db: db}
}

// RecordChange records a change to equipment status or metadata
func (r *historyRepository) RecordChange(ctx context.Context, history *models.EquipmentHistory) error {
	query := `
		INSERT INTO equipment_history (
			imei, change_type, changed_at, changed_by,
			previous_status, new_status, previous_reason, new_reason,
			change_details, session_id
		) VALUES (
			:imei, :change_type, :changed_at, :changed_by,
			:previous_status, :new_status, :previous_reason, :new_reason,
			:change_details, :session_id
		) RETURNING id
	`

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	err = stmt.GetContext(ctx, &history.ID, history)
	if err != nil {
		return fmt.Errorf("failed to record change: %w", err)
	}

	return nil
}

// GetHistoryByIMEI retrieves change history for a specific IMEI
func (r *historyRepository) GetHistoryByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.EquipmentHistory, error) {
	query := `
		SELECT id, imei, change_type, changed_at, changed_by,
		       previous_status, new_status, previous_reason, new_reason,
		       change_details, session_id
		FROM equipment_history
		WHERE imei = $1
		ORDER BY changed_at DESC
		LIMIT $2 OFFSET $3
	`

	var history []*models.EquipmentHistory
	err := r.db.SelectContext(ctx, &history, query, imei, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get history by IMEI: %w", err)
	}

	return history, nil
}

// GetHistoryByTimeRange retrieves change history within a time range
func (r *historyRepository) GetHistoryByTimeRange(ctx context.Context, startTime, endTime time.Time, offset, limit int) ([]*models.EquipmentHistory, error) {
	query := `
		SELECT id, imei, change_type, changed_at, changed_by,
		       previous_status, new_status, previous_reason, new_reason,
		       change_details, session_id
		FROM equipment_history
		WHERE changed_at >= $1 AND changed_at <= $2
		ORDER BY changed_at DESC
		LIMIT $3 OFFSET $4
	`

	var history []*models.EquipmentHistory
	err := r.db.SelectContext(ctx, &history, query, startTime, endTime, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get history by time range: %w", err)
	}

	return history, nil
}

// GetHistoryByChangeType retrieves history filtered by change type
func (r *historyRepository) GetHistoryByChangeType(ctx context.Context, changeType models.ChangeType, offset, limit int) ([]*models.EquipmentHistory, error) {
	query := `
		SELECT id, imei, change_type, changed_at, changed_by,
		       previous_status, new_status, previous_reason, new_reason,
		       change_details, session_id
		FROM equipment_history
		WHERE change_type = $1
		ORDER BY changed_at DESC
		LIMIT $2 OFFSET $3
	`

	var history []*models.EquipmentHistory
	err := r.db.SelectContext(ctx, &history, query, changeType, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get history by change type: %w", err)
	}

	return history, nil
}
