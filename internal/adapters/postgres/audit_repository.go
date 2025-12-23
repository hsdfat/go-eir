package postgres

import (
	"context"
	"fmt"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
)

// auditRepository implements the AuditRepository interface using PostgreSQL
type auditRepository struct {
	db dbExecutor
}

// NewAuditRepository creates a new PostgreSQL audit repository
func NewAuditRepository(db dbExecutor) ports.AuditRepository {
	return &auditRepository{db: db}
}

// LogCheck records an equipment check operation
func (r *auditRepository) LogCheck(ctx context.Context, audit *models.AuditLog) error {
	query := `
		INSERT INTO audit_log (
			imei, imeisv, status, check_time, origin_host, origin_realm,
			user_name, supi, gpsi, request_source, session_id, result_code
		) VALUES (
			:imei, :imeisv, :status, :check_time, :origin_host, :origin_realm,
			:user_name, :supi, :gpsi, :request_source, :session_id, :result_code
		) RETURNING id
	`

	stmt, err := r.db.PrepareNamedContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	err = stmt.GetContext(ctx, &audit.ID, audit)
	if err != nil {
		return fmt.Errorf("failed to log check: %w", err)
	}

	return nil
}

// GetAuditsByIMEI retrieves audit logs for a specific IMEI
func (r *auditRepository) GetAuditsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, imei, imeisv, status, check_time, origin_host, origin_realm,
		       user_name, supi, gpsi, request_source, session_id, result_code
		FROM audit_log
		WHERE imei = $1
		ORDER BY check_time DESC
		LIMIT $2 OFFSET $3
	`

	var audits []*models.AuditLog
	err := r.db.SelectContext(ctx, &audits, query, imei, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get audits by IMEI: %w", err)
	}

	return audits, nil
}

// GetAuditsByTimeRange retrieves audit logs within a time range
func (r *auditRepository) GetAuditsByTimeRange(ctx context.Context, startTime, endTime string, offset, limit int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, imei, imeisv, status, check_time, origin_host, origin_realm,
		       user_name, supi, gpsi, request_source, session_id, result_code
		FROM audit_log
		WHERE check_time >= $1::timestamp AND check_time <= $2::timestamp
		ORDER BY check_time DESC
		LIMIT $3 OFFSET $4
	`

	var audits []*models.AuditLog
	err := r.db.SelectContext(ctx, &audits, query, startTime, endTime, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get audits by time range: %w", err)
	}

	return audits, nil
}
