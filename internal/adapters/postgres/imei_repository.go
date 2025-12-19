package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/jmoiron/sqlx"
)

var (
	ErrNotFound      = errors.New("equipment not found")
	ErrAlreadyExists = errors.New("equipment already exists")
)

// imeiRepository implements the IMEIRepository interface using PostgreSQL
type imeiRepository struct {
	db *sqlx.DB
}

// NewIMEIRepository creates a new PostgreSQL IMEI repository
func NewIMEIRepository(db *sqlx.DB) ports.IMEIRepository {
	return &imeiRepository{db: db}
}

// GetByIMEI retrieves equipment by IMEI
func (r *imeiRepository) GetByIMEI(ctx context.Context, imei string) (*models.Equipment, error) {
	query := `
		SELECT id, imei, imeisv, status, reason, last_updated, last_check_time,
		       check_count, added_by, metadata, manufacturer_tac, manufacturer_name
		FROM equipment
		WHERE imei = $1
	`

	var equipment models.Equipment
	err := r.db.GetContext(ctx, &equipment, query, imei)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get equipment: %w", err)
	}

	return &equipment, nil
}

// GetByIMEISV retrieves equipment by IMEISV
func (r *imeiRepository) GetByIMEISV(ctx context.Context, imeisv string) (*models.Equipment, error) {
	query := `
		SELECT id, imei, imeisv, status, reason, last_updated, last_check_time,
		       check_count, added_by, metadata, manufacturer_tac, manufacturer_name
		FROM equipment
		WHERE imeisv = $1
	`

	var equipment models.Equipment
	err := r.db.GetContext(ctx, &equipment, query, imeisv)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get equipment by IMEISV: %w", err)
	}

	return &equipment, nil
}

// Create adds a new equipment record
func (r *imeiRepository) Create(ctx context.Context, equipment *models.Equipment) error {
	query := `
		INSERT INTO equipment (
			imei, imeisv, status, reason, last_updated, last_check_time,
			check_count, added_by, metadata, manufacturer_tac, manufacturer_name
		) VALUES (
			:imei, :imeisv, :status, :reason, :last_updated, :last_check_time,
			:check_count, :added_by, :metadata, :manufacturer_tac, :manufacturer_name
		) RETURNING id
	`

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	err = stmt.GetContext(ctx, &equipment.ID, equipment)
	if err != nil {
		return fmt.Errorf("failed to create equipment: %w", err)
	}

	return nil
}

// Update updates an existing equipment record
func (r *imeiRepository) Update(ctx context.Context, equipment *models.Equipment) error {
	query := `
		UPDATE equipment
		SET imeisv = :imeisv,
		    status = :status,
		    reason = :reason,
		    last_updated = :last_updated,
		    metadata = :metadata,
		    manufacturer_tac = :manufacturer_tac,
		    manufacturer_name = :manufacturer_name
		WHERE imei = :imei
	`

	result, err := r.db.NamedExecContext(ctx, query, equipment)
	if err != nil {
		return fmt.Errorf("failed to update equipment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete removes an equipment record
func (r *imeiRepository) Delete(ctx context.Context, imei string) error {
	query := `DELETE FROM equipment WHERE imei = $1`

	result, err := r.db.ExecContext(ctx, query, imei)
	if err != nil {
		return fmt.Errorf("failed to delete equipment: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// List retrieves equipment with pagination
func (r *imeiRepository) List(ctx context.Context, offset, limit int) ([]*models.Equipment, error) {
	query := `
		SELECT id, imei, imeisv, status, reason, last_updated, last_check_time,
		       check_count, added_by, metadata, manufacturer_tac, manufacturer_name
		FROM equipment
		ORDER BY last_updated DESC
		LIMIT $1 OFFSET $2
	`

	var equipments []*models.Equipment
	err := r.db.SelectContext(ctx, &equipments, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list equipment: %w", err)
	}

	return equipments, nil
}

// ListByStatus retrieves equipment by status with pagination
func (r *imeiRepository) ListByStatus(ctx context.Context, status models.EquipmentStatus, offset, limit int) ([]*models.Equipment, error) {
	query := `
		SELECT id, imei, imeisv, status, reason, last_updated, last_check_time,
		       check_count, added_by, metadata, manufacturer_tac, manufacturer_name
		FROM equipment
		WHERE status = $1
		ORDER BY last_updated DESC
		LIMIT $2 OFFSET $3
	`

	var equipments []*models.Equipment
	err := r.db.SelectContext(ctx, &equipments, query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list equipment by status: %w", err)
	}

	return equipments, nil
}

// IncrementCheckCount atomically increments check counter and updates last check time
func (r *imeiRepository) IncrementCheckCount(ctx context.Context, imei string) error {
	query := `SELECT increment_equipment_check_count($1)`

	_, err := r.db.ExecContext(ctx, query, imei)
	if err != nil {
		return fmt.Errorf("failed to increment check count: %w", err)
	}

	return nil
}
