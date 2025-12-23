package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
)

// extendedAuditRepository implements the ExtendedAuditRepository interface using PostgreSQL
type extendedAuditRepository struct {
	auditRepository
}

// NewExtendedAuditRepository creates a new PostgreSQL extended audit repository
func NewExtendedAuditRepository(db dbExecutor) ports.ExtendedAuditRepository {
	return &extendedAuditRepository{
		auditRepository: auditRepository{db: db},
	}
}

// LogCheckExtended records an extended equipment check with additional metadata
func (r *extendedAuditRepository) LogCheckExtended(ctx context.Context, audit *models.AuditLogExtended) error {
	// First, log the basic audit entry
	err := r.auditRepository.LogCheck(ctx, &audit.AuditLog)
	if err != nil {
		return err
	}

	// Then, log the extended metadata
	var additionalDataJSON []byte
	if audit.AdditionalData != nil {
		additionalDataJSON, err = json.Marshal(audit.AdditionalData)
		if err != nil {
			return fmt.Errorf("failed to marshal additional data: %w", err)
		}
	}

	query := `
		INSERT INTO audit_log_extended (
			audit_log_id, ip_address, user_agent, additional_data, processing_time_ms
		) VALUES (
			$1, $2, $3, $4, $5
		)
	`

	_, err = r.db.ExecContext(ctx, query,
		audit.ID,
		audit.IPAddress,
		audit.UserAgent,
		additionalDataJSON,
		audit.ProcessingTimeMs,
	)
	if err != nil {
		return fmt.Errorf("failed to log extended audit: %w", err)
	}

	// Record change history if provided
	if audit.ChangeHistory != nil {
		historyRepo := NewHistoryRepository(r.db)
		err = historyRepo.RecordChange(ctx, audit.ChangeHistory)
		if err != nil {
			return fmt.Errorf("failed to record change history: %w", err)
		}
	}

	return nil
}

// GetExtendedAuditsByIMEI retrieves extended audit logs for a specific IMEI
func (r *extendedAuditRepository) GetExtendedAuditsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.AuditLogExtended, error) {
	query := `
		SELECT
			al.id, al.imei, al.imeisv, al.status, al.check_time, al.origin_host, al.origin_realm,
			al.user_name, al.supi, al.gpsi, al.request_source, al.session_id, al.result_code,
			ale.ip_address, ale.user_agent, ale.additional_data, ale.processing_time_ms
		FROM audit_log al
		LEFT JOIN audit_log_extended ale ON al.id = ale.audit_log_id
		WHERE al.imei = $1
		ORDER BY al.check_time DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, imei, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get extended audits by IMEI: %w", err)
	}
	defer rows.Close()

	var audits []*models.AuditLogExtended
	for rows.Next() {
		var audit models.AuditLogExtended
		var additionalDataJSON []byte

		err := rows.Scan(
			&audit.ID, &audit.IMEI, &audit.IMEISV, &audit.Status, &audit.CheckTime,
			&audit.OriginHost, &audit.OriginRealm, &audit.UserName, &audit.SUPI, &audit.GPSI,
			&audit.RequestSource, &audit.SessionID, &audit.ResultCode,
			&audit.IPAddress, &audit.UserAgent, &additionalDataJSON, &audit.ProcessingTimeMs,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit row: %w", err)
		}

		if len(additionalDataJSON) > 0 {
			err = json.Unmarshal(additionalDataJSON, &audit.AdditionalData)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal additional data: %w", err)
			}
		}

		audits = append(audits, &audit)
	}

	return audits, nil
}

// GetAuditsByRequestSource retrieves audits filtered by request source
func (r *extendedAuditRepository) GetAuditsByRequestSource(ctx context.Context, requestSource string, offset, limit int) ([]*models.AuditLog, error) {
	query := `
		SELECT id, imei, imeisv, status, check_time, origin_host, origin_realm,
		       user_name, supi, gpsi, request_source, session_id, result_code
		FROM audit_log
		WHERE request_source = $1
		ORDER BY check_time DESC
		LIMIT $2 OFFSET $3
	`

	var audits []*models.AuditLog
	err := r.db.SelectContext(ctx, &audits, query, requestSource, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get audits by request source: %w", err)
	}

	return audits, nil
}

// GetAuditStatistics retrieves aggregated audit statistics
func (r *extendedAuditRepository) GetAuditStatistics(ctx context.Context, startTime, endTime time.Time) (map[string]interface{}, error) {
	query := `
		SELECT
			COUNT(*) as total_checks,
			COUNT(DISTINCT imei) as unique_imeis,
			COUNT(CASE WHEN status = 'WHITELISTED' THEN 1 END) as whitelisted_count,
			COUNT(CASE WHEN status = 'BLACKLISTED' THEN 1 END) as blacklisted_count,
			COUNT(CASE WHEN status = 'GREYLISTED' THEN 1 END) as greylisted_count,
			COUNT(CASE WHEN request_source = 'DIAMETER_S13' THEN 1 END) as diameter_checks,
			COUNT(CASE WHEN request_source = 'HTTP_5G' THEN 1 END) as http_checks,
			AVG(COALESCE(ale.processing_time_ms, 0)) as avg_processing_time_ms
		FROM audit_log al
		LEFT JOIN audit_log_extended ale ON al.id = ale.audit_log_id
		WHERE al.check_time >= $1 AND al.check_time <= $2
	`

	var stats struct {
		TotalChecks          int64   `db:"total_checks"`
		UniqueIMEIs          int64   `db:"unique_imeis"`
		WhitelistedCount     int64   `db:"whitelisted_count"`
		BlacklistedCount     int64   `db:"blacklisted_count"`
		GreylistedCount      int64   `db:"greylisted_count"`
		DiameterChecks       int64   `db:"diameter_checks"`
		HTTPChecks           int64   `db:"http_checks"`
		AvgProcessingTimeMs  float64 `db:"avg_processing_time_ms"`
	}

	err := r.db.GetContext(ctx, &stats, query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit statistics: %w", err)
	}

	result := map[string]interface{}{
		"total_checks":            stats.TotalChecks,
		"unique_imeis":            stats.UniqueIMEIs,
		"whitelisted_count":       stats.WhitelistedCount,
		"blacklisted_count":       stats.BlacklistedCount,
		"greylisted_count":        stats.GreylistedCount,
		"diameter_checks":         stats.DiameterChecks,
		"http_checks":             stats.HTTPChecks,
		"avg_processing_time_ms":  stats.AvgProcessingTimeMs,
	}

	return result, nil
}
