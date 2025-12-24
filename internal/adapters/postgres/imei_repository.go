package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
)

var (
	ErrNotFound      = errors.New("equipment not found")
	ErrAlreadyExists = errors.New("equipment already exists")
)

// imeiRepository implements the IMEIRepository interface using PostgreSQL
type imeiRepository struct {
	db dbExecutor
}

// NewIMEIRepository creates a new PostgreSQL IMEI repository
func NewIMEIRepository(db dbExecutor) ports.IMEIRepository {
	return &imeiRepository{db: db}
}

func (r *imeiRepository) SetLogger(l logger.Logger) {
	// Mock implementation - no-op for testing
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

// IMEI logic operations (not implemented for PostgreSQL - use in-memory for testing)
func (r *imeiRepository) LookupImeiInfo(ctx context.Context, startRange string) (*ports.ImeiInfo, bool) {
	query := `SELECT startimei, endimei, color FROM imei_info WHERE startimei = $1`

	var info ports.ImeiInfo
	// Lưu ý: ports.ImeiInfo.EndIMEI nên là []string để tương thích với TEXT[]
	err := r.db.GetContext(ctx, &info, query, startRange)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false
		}
		return nil, false
	}
	return &info, true
}

func (r *imeiRepository) SaveImeiInfo(ctx context.Context, info *ports.ImeiInfo) error {
	query := `
		INSERT INTO imei_info (startimei, endimei, color)
		VALUES ($1, $2, $3)
		ON CONFLICT (startimei) 
		DO UPDATE SET endimei = EXCLUDED.endimei, color = EXCLUDED.color
	`
	_, err := r.db.ExecContext(ctx, query, info.StartIMEI, info.EndIMEI, info.Color)
	if err != nil {
		return fmt.Errorf("failed to save imei info: %w", err)
	}
	return nil
}

func (r *imeiRepository) ListAllImeiInfo(ctx context.Context) []ports.ImeiInfo {
	query := `SELECT startimei, endimei, color FROM imei_info`

	var result []ports.ImeiInfo
	err := r.db.SelectContext(ctx, &result, query)
	if err != nil {
		return []ports.ImeiInfo{}
	}
	return result
}

func (r *imeiRepository) ClearImeiInfo() {
	// No-op
}

// TAC logic operations (not implemented for PostgreSQL - use in-memory for testing)
func (r *imeiRepository) SaveTacInfo(ctx context.Context, info *ports.TacInfo) error {
	logger.Log.Debugw("Jump into SaveTacInfo in database")

	query := `
		INSERT INTO tac_info (keytac, startrangetac, endrangetac, color, prevlink)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (keytac) 
		DO UPDATE SET 
			startrangetac = EXCLUDED.startrangetac, 
			endrangetac = EXCLUDED.endrangetac, 
			color = EXCLUDED.color, 
			prevlink = EXCLUDED.prevlink
	`
	_, err := r.db.ExecContext(ctx, query,
		info.KeyTac, info.StartRangeTac, info.EndRangeTac, info.Color, info.PrevLink)
	if err != nil {
		logger.Log.Debugw("error executing SaveTacInfo: ", err)
		return fmt.Errorf("failed to save tac info: %w", err)
	}
	return nil
}

func (r *imeiRepository) LookupTacInfo(ctx context.Context, key string) (*ports.TacInfo, bool) {
	logger.Log.Debugw("Jump into LookupTacInfo in database")

	query := `SELECT keytac, startrangetac, endrangetac, color, prevlink FROM tac_info WHERE keytac = $1`

	var info ports.TacInfo
	err := r.db.GetContext(ctx, &info, query, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false
		}
		return nil, false
	}
	return &info, true
}

func (r *imeiRepository) PrevTacInfo(ctx context.Context, key string) (*ports.TacInfo, bool) {
	logger.Log.Debugw("Jump into PrevTacInfo in database")

	query := `
		SELECT keytac, startrangetac, endrangetac, color, prevlink 
		FROM tac_info 
		WHERE keytac < $1 
		ORDER BY keytac DESC 
		LIMIT 1
	`

	var info ports.TacInfo
	err := r.db.GetContext(ctx, &info, query, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false
		}
		return nil, false
	}
	return &info, true
}

func (r *imeiRepository) NextTacInfo(ctx context.Context, key string) (*ports.TacInfo, bool) {
	logger.Log.Debugw("Jump into NextTacInfo in database")

	query := `
		SELECT keytac, startrangetac, endrangetac, color, prevlink 
		FROM tac_info 
		WHERE keytac > $1 
		ORDER BY keytac ASC 
		LIMIT 1
	`

	var info ports.TacInfo
	err := r.db.GetContext(ctx, &info, query, key)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false
		}
		return nil, false
	}
	return &info, true
}

func (r *imeiRepository) ListAllTacInfo(ctx context.Context) []*ports.TacInfo {
	logger.Log.Debugw("Jump into ListAllTacInfo in database")
	query := `SELECT keytac, startrangetac, endrangetac, color, prevlink FROM tac_info ORDER BY keytac ASC`

	var result []*ports.TacInfo
	err := r.db.SelectContext(ctx, &result, query)
	if err != nil {
		return []*ports.TacInfo{}
	}
	return result
}

func (r *imeiRepository) ClearTacInfo(ctx context.Context) {
	logger.Log.Debug("Cleaning tac_info")
	query := `DELETE FROM tac_info`
	_, _ = r.db.ExecContext(ctx, query)
}
