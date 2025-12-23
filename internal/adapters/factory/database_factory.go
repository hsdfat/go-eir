package factory

import (
	"context"
	"fmt"

	"github.com/hsdfat8/eir/internal/adapters/mongodb"
	"github.com/hsdfat8/eir/internal/adapters/postgres"
	"github.com/hsdfat8/eir/internal/domain/ports"
)

// DatabaseAdapterFactory creates database adapters based on configuration
type DatabaseAdapterFactory struct{}

// NewDatabaseAdapterFactory creates a new database adapter factory
func NewDatabaseAdapterFactory() *DatabaseAdapterFactory {
	return &DatabaseAdapterFactory{}
}

// CreateAdapter creates a database adapter based on the provided configuration
func (f *DatabaseAdapterFactory) CreateAdapter(config *ports.DatabaseConfig) (ports.DatabaseAdapter, error) {
	switch config.Type {
	case ports.DatabaseTypePostgreSQL:
		if config.PostgresConfig == nil {
			return nil, fmt.Errorf("postgres configuration is required for PostgreSQL adapter")
		}
		return postgres.NewPostgresAdapter(config.PostgresConfig), nil

	case ports.DatabaseTypeMongoDB:
		if config.MongoDBConfig == nil {
			return nil, fmt.Errorf("mongodb configuration is required for MongoDB adapter")
		}
		return mongodb.NewMongoDBAdapter(config.MongoDBConfig), nil

	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

// CreateAndConnectAdapter creates and connects a database adapter
func (f *DatabaseAdapterFactory) CreateAndConnectAdapter(ctx context.Context, config *ports.DatabaseConfig) (ports.DatabaseAdapter, error) {
	adapter, err := f.CreateAdapter(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create adapter: %w", err)
	}

	err = adapter.Connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return adapter, nil
}

// ValidateConfig validates the database configuration
func (f *DatabaseAdapterFactory) ValidateConfig(config *ports.DatabaseConfig) error {
	if config == nil {
		return fmt.Errorf("database configuration is nil")
	}

	switch config.Type {
	case ports.DatabaseTypePostgreSQL:
		return f.validatePostgresConfig(config.PostgresConfig)
	case ports.DatabaseTypeMongoDB:
		return f.validateMongoDBConfig(config.MongoDBConfig)
	default:
		return fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

func (f *DatabaseAdapterFactory) validatePostgresConfig(config *ports.PostgresConfig) error {
	if config == nil {
		return fmt.Errorf("postgres configuration is nil")
	}

	if config.Host == "" {
		return fmt.Errorf("postgres host is required")
	}

	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("postgres port must be between 1 and 65535")
	}

	if config.User == "" {
		return fmt.Errorf("postgres user is required")
	}

	if config.Database == "" {
		return fmt.Errorf("postgres database name is required")
	}

	if config.MaxOpenConns <= 0 {
		return fmt.Errorf("max_open_conns must be greater than 0")
	}

	if config.MaxIdleConns <= 0 {
		return fmt.Errorf("max_idle_conns must be greater than 0")
	}

	if config.MaxIdleConns > config.MaxOpenConns {
		return fmt.Errorf("max_idle_conns cannot be greater than max_open_conns")
	}

	return nil
}

func (f *DatabaseAdapterFactory) validateMongoDBConfig(config *ports.MongoDBConfig) error {
	if config == nil {
		return fmt.Errorf("mongodb configuration is nil")
	}

	if config.URI == "" {
		return fmt.Errorf("mongodb URI is required")
	}

	if config.Database == "" {
		return fmt.Errorf("mongodb database name is required")
	}

	if config.MaxPoolSize <= 0 {
		return fmt.Errorf("max_pool_size must be greater than 0")
	}

	if config.MinPoolSize < 0 {
		return fmt.Errorf("min_pool_size cannot be negative")
	}

	if config.MinPoolSize > config.MaxPoolSize {
		return fmt.Errorf("min_pool_size cannot be greater than max_pool_size")
	}

	return nil
}

// GetDefaultPostgresConfig returns a default PostgreSQL configuration
func GetDefaultPostgresConfig() *ports.PostgresConfig {
	return &ports.PostgresConfig{
		Host:            "localhost",
		Port:            5432,
		User:            "eir",
		Password:        "",
		Database:        "eir",
		SSLMode:         "disable",
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 300,  // 5 minutes
		ConnMaxIdleTime: 600,  // 10 minutes
		QueryTimeout:    30,   // 30 seconds
	}
}

// GetDefaultMongoDBConfig returns a default MongoDB configuration
func GetDefaultMongoDBConfig() *ports.MongoDBConfig {
	return &ports.MongoDBConfig{
		URI:                "mongodb://localhost:27017",
		Database:           "eir",
		MaxPoolSize:        100,
		MinPoolSize:        10,
		MaxConnIdleTime:    600,  // 10 minutes
		ServerTimeout:      30,   // 30 seconds
		SocketTimeout:      30,   // 30 seconds
		ReplicaSet:         "",
		ReadPreference:     "primary",
		WriteConcern:       "majority",
		EnableChangeStream: false,
	}
}

// CreateDefaultConfig creates a default database configuration for the specified type
func CreateDefaultConfig(dbType ports.DatabaseType) *ports.DatabaseConfig {
	config := &ports.DatabaseConfig{
		Type: dbType,
	}

	switch dbType {
	case ports.DatabaseTypePostgreSQL:
		config.PostgresConfig = GetDefaultPostgresConfig()
	case ports.DatabaseTypeMongoDB:
		config.MongoDBConfig = GetDefaultMongoDBConfig()
	}

	return config
}
