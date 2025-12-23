package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// PostgresAdapter implements the DatabaseAdapter interface for PostgreSQL
type PostgresAdapter struct {
	db                      *sqlx.DB
	config                  *ports.PostgresConfig
	imeiRepo                ports.IMEIRepository
	auditRepo               ports.AuditRepository
	extendedAuditRepo       ports.ExtendedAuditRepository
	historyRepo             ports.HistoryRepository
	snapshotRepo            ports.SnapshotRepository
}

// NewPostgresAdapter creates a new PostgreSQL database adapter
func NewPostgresAdapter(config *ports.PostgresConfig) *PostgresAdapter {
	return &PostgresAdapter{
		config: config,
	}
}

// Connect establishes a connection to the PostgreSQL database
func (a *PostgresAdapter) Connect(ctx context.Context) error {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		a.config.Host,
		a.config.Port,
		a.config.User,
		a.config.Password,
		a.config.Database,
		a.config.SSLMode,
	)

	db, err := sqlx.ConnectContext(ctx, "postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to postgres: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(a.config.MaxOpenConns)
	db.SetMaxIdleConns(a.config.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(a.config.ConnMaxLifetime) * time.Second)
	db.SetConnMaxIdleTime(time.Duration(a.config.ConnMaxIdleTime) * time.Second)

	a.db = db

	// Initialize repositories
	a.imeiRepo = NewIMEIRepository(db)
	a.auditRepo = NewAuditRepository(db)
	a.extendedAuditRepo = NewExtendedAuditRepository(db)
	a.historyRepo = NewHistoryRepository(db)
	a.snapshotRepo = NewSnapshotRepository(db)

	return nil
}

// Disconnect closes the database connection
func (a *PostgresAdapter) Disconnect(ctx context.Context) error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

// Ping checks if the database connection is alive
func (a *PostgresAdapter) Ping(ctx context.Context) error {
	if a.db == nil {
		return fmt.Errorf("database not connected")
	}
	return a.db.PingContext(ctx)
}

// GetType returns the database type
func (a *PostgresAdapter) GetType() ports.DatabaseType {
	return ports.DatabaseTypePostgreSQL
}

// BeginTransaction starts a new database transaction
func (a *PostgresAdapter) BeginTransaction(ctx context.Context) (ports.Transaction, error) {
	tx, err := a.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &postgresTransaction{
		tx:        tx,
		imeiRepo:  NewIMEIRepository(tx),
		auditRepo: NewAuditRepository(tx),
	}, nil
}

// GetIMEIRepository returns the IMEI repository
func (a *PostgresAdapter) GetIMEIRepository() ports.IMEIRepository {
	return a.imeiRepo
}

// GetAuditRepository returns the audit repository
func (a *PostgresAdapter) GetAuditRepository() ports.AuditRepository {
	return a.auditRepo
}

// GetExtendedAuditRepository returns the extended audit repository
func (a *PostgresAdapter) GetExtendedAuditRepository() ports.ExtendedAuditRepository {
	return a.extendedAuditRepo
}

// GetHistoryRepository returns the history repository
func (a *PostgresAdapter) GetHistoryRepository() ports.HistoryRepository {
	return a.historyRepo
}

// GetSnapshotRepository returns the snapshot repository
func (a *PostgresAdapter) GetSnapshotRepository() ports.SnapshotRepository {
	return a.snapshotRepo
}

// HealthCheck performs a health check on the database
func (a *PostgresAdapter) HealthCheck(ctx context.Context) error {
	if err := a.Ping(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// Test a simple query
	var result int
	err := a.db.GetContext(ctx, &result, "SELECT 1")
	if err != nil {
		return fmt.Errorf("health check query failed: %w", err)
	}

	return nil
}

// GetConnectionStats returns database connection statistics
func (a *PostgresAdapter) GetConnectionStats() ports.ConnectionStats {
	stats := a.db.Stats()

	return ports.ConnectionStats{
		OpenConnections: stats.OpenConnections,
		IdleConnections: stats.Idle,
		MaxConnections:  a.config.MaxOpenConns,
		DatabaseType:    string(ports.DatabaseTypePostgreSQL),
		ConnectionString: fmt.Sprintf("%s:%d/%s", a.config.Host, a.config.Port, a.config.Database),
		Healthy:         a.Ping(context.Background()) == nil,
	}
}

// PurgeOldAudits removes audit logs older than the specified date
func (a *PostgresAdapter) PurgeOldAudits(ctx context.Context, beforeDate string) (int64, error) {
	query := `DELETE FROM audit_log WHERE check_time < $1::timestamp`

	result, err := a.db.ExecContext(ctx, query, beforeDate)
	if err != nil {
		return 0, fmt.Errorf("failed to purge old audits: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// PurgeOldHistory removes history records older than the specified date
func (a *PostgresAdapter) PurgeOldHistory(ctx context.Context, beforeDate string) (int64, error) {
	query := `DELETE FROM equipment_history WHERE changed_at < $1::timestamp`

	result, err := a.db.ExecContext(ctx, query, beforeDate)
	if err != nil {
		return 0, fmt.Errorf("failed to purge old history: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return rowsAffected, nil
}

// OptimizeDatabase performs database optimization operations
func (a *PostgresAdapter) OptimizeDatabase(ctx context.Context) error {
	// Run VACUUM ANALYZE on main tables
	tables := []string{"equipment", "audit_log", "equipment_history", "equipment_snapshots"}

	for _, table := range tables {
		_, err := a.db.ExecContext(ctx, fmt.Sprintf("VACUUM ANALYZE %s", table))
		if err != nil {
			return fmt.Errorf("failed to optimize table %s: %w", table, err)
		}
	}

	return nil
}

// postgresTransaction implements the Transaction interface
type postgresTransaction struct {
	tx        *sqlx.Tx
	imeiRepo  ports.IMEIRepository
	auditRepo ports.AuditRepository
}

// Commit commits the transaction
func (t *postgresTransaction) Commit(ctx context.Context) error {
	return t.tx.Commit()
}

// Rollback rolls back the transaction
func (t *postgresTransaction) Rollback(ctx context.Context) error {
	return t.tx.Rollback()
}

// GetIMEIRepository returns a transactional IMEI repository
func (t *postgresTransaction) GetIMEIRepository() ports.IMEIRepository {
	return t.imeiRepo
}

// GetAuditRepository returns a transactional audit repository
func (t *postgresTransaction) GetAuditRepository() ports.AuditRepository {
	return t.auditRepo
}
