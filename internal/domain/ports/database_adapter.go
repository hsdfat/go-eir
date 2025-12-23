package ports

import (
	"context"
	"io"
)

// DatabaseType represents the type of database backend
type DatabaseType string

const (
	DatabaseTypePostgreSQL DatabaseType = "postgres"
	DatabaseTypeMongoDB    DatabaseType = "mongodb"
)

// DatabaseAdapter defines the unified interface for database operations
// This provides a common abstraction over PostgreSQL and MongoDB implementations
type DatabaseAdapter interface {
	// Connect establishes a connection to the database
	Connect(ctx context.Context) error

	// Disconnect closes the database connection
	Disconnect(ctx context.Context) error

	// Ping checks if the database connection is alive
	Ping(ctx context.Context) error

	// GetType returns the database type
	GetType() DatabaseType

	// Transaction management
	BeginTransaction(ctx context.Context) (Transaction, error)

	// Repository factory methods
	GetIMEIRepository() IMEIRepository
	GetAuditRepository() AuditRepository
	GetExtendedAuditRepository() ExtendedAuditRepository
	GetHistoryRepository() HistoryRepository
	GetSnapshotRepository() SnapshotRepository

	// Health and maintenance
	HealthCheck(ctx context.Context) error
	GetConnectionStats() ConnectionStats

	// Cleanup and maintenance operations
	PurgeOldAudits(ctx context.Context, beforeDate string) (int64, error)
	PurgeOldHistory(ctx context.Context, beforeDate string) (int64, error)
	OptimizeDatabase(ctx context.Context) error
}

// Transaction represents a database transaction
type Transaction interface {
	// Commit commits the transaction
	Commit(ctx context.Context) error

	// Rollback rolls back the transaction
	Rollback(ctx context.Context) error

	// GetIMEIRepository returns a transactional IMEI repository
	GetIMEIRepository() IMEIRepository

	// GetAuditRepository returns a transactional audit repository
	GetAuditRepository() AuditRepository
}

// ConnectionStats provides database connection statistics
type ConnectionStats struct {
	OpenConnections  int    `json:"open_connections"`
	IdleConnections  int    `json:"idle_connections"`
	MaxConnections   int    `json:"max_connections"`
	DatabaseType     string `json:"database_type"`
	ConnectionString string `json:"connection_string"` // Sanitized, without credentials
	Healthy          bool   `json:"healthy"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Type             DatabaseType          `yaml:"type" json:"type"`
	PostgresConfig   *PostgresConfig       `yaml:"postgres,omitempty" json:"postgres,omitempty"`
	MongoDBConfig    *MongoDBConfig        `yaml:"mongodb,omitempty" json:"mongodb,omitempty"`
}

// PostgresConfig holds PostgreSQL-specific configuration
type PostgresConfig struct {
	Host            string `yaml:"host" json:"host"`
	Port            int    `yaml:"port" json:"port"`
	User            string `yaml:"user" json:"user"`
	Password        string `yaml:"password" json:"password"`
	Database        string `yaml:"database" json:"database"`
	SSLMode         string `yaml:"ssl_mode" json:"ssl_mode"`
	MaxOpenConns    int    `yaml:"max_open_conns" json:"max_open_conns"`
	MaxIdleConns    int    `yaml:"max_idle_conns" json:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime" json:"conn_max_lifetime"` // in seconds
	ConnMaxIdleTime int    `yaml:"conn_max_idle_time" json:"conn_max_idle_time"` // in seconds
	QueryTimeout    int    `yaml:"query_timeout" json:"query_timeout"` // in seconds
}

// MongoDBConfig holds MongoDB-specific configuration
type MongoDBConfig struct {
	URI                string `yaml:"uri" json:"uri"`
	Database           string `yaml:"database" json:"database"`
	MaxPoolSize        int    `yaml:"max_pool_size" json:"max_pool_size"`
	MinPoolSize        int    `yaml:"min_pool_size" json:"min_pool_size"`
	MaxConnIdleTime    int    `yaml:"max_conn_idle_time" json:"max_conn_idle_time"` // in seconds
	ServerTimeout      int    `yaml:"server_timeout" json:"server_timeout"` // in seconds
	SocketTimeout      int    `yaml:"socket_timeout" json:"socket_timeout"` // in seconds
	ReplicaSet         string `yaml:"replica_set,omitempty" json:"replica_set,omitempty"`
	ReadPreference     string `yaml:"read_preference" json:"read_preference"` // primary, secondary, etc.
	WriteConcern       string `yaml:"write_concern" json:"write_concern"` // majority, etc.
	EnableChangeStream bool   `yaml:"enable_change_stream" json:"enable_change_stream"`
}

// DatabaseMigration defines interface for database migrations
type DatabaseMigration interface {
	// Up applies the migration
	Up(ctx context.Context) error

	// Down reverts the migration
	Down(ctx context.Context) error

	// Version returns the migration version
	Version() int64

	// Description returns the migration description
	Description() string
}

// MigrationManager manages database migrations
type MigrationManager interface {
	// Run runs all pending migrations
	Run(ctx context.Context) error

	// Rollback rolls back the last migration
	Rollback(ctx context.Context) error

	// Status returns the current migration status
	Status(ctx context.Context) ([]MigrationStatus, error)
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	Version     int64  `json:"version"`
	Description string `json:"description"`
	Applied     bool   `json:"applied"`
	AppliedAt   string `json:"applied_at,omitempty"`
}

// DataExporter exports data from the database
type DataExporter interface {
	io.Closer

	// ExportEquipment exports equipment data
	ExportEquipment(ctx context.Context, writer io.Writer, format string) error

	// ExportAudits exports audit logs
	ExportAudits(ctx context.Context, writer io.Writer, format string, startTime, endTime string) error

	// ExportHistory exports change history
	ExportHistory(ctx context.Context, writer io.Writer, format string, startTime, endTime string) error
}

// DataImporter imports data into the database
type DataImporter interface {
	io.Closer

	// ImportEquipment imports equipment data
	ImportEquipment(ctx context.Context, reader io.Reader, format string) (int64, error)

	// ValidateImport validates import data without applying it
	ValidateImport(ctx context.Context, reader io.Reader, format string) error
}
