package postgres

import (
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// Config holds PostgreSQL connection configuration
type Config struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// NewDB creates a new PostgreSQL database connection
func NewDB(cfg Config) (*sqlx.DB, error) {
	sslMode := cfg.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	// Use URL format for DSN (better compatibility with YugabyteDB than key=value format)
	passwordPart := ""
	if cfg.Password != "" {
		passwordPart = ":" + cfg.Password
	}

	// First, connect to the default database to check if target database exists
	defaultDB := "yugabyte" // YugabyteDB default database
	defaultDSN := fmt.Sprintf("postgres://%s%s@%s:%d/%s?sslmode=%s",
		cfg.User, passwordPart, cfg.Host, cfg.Port, defaultDB, sslMode)

	adminDB, err := sqlx.Connect("postgres", defaultDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open admin connection: %w", err)
	}
	defer adminDB.Close()

	// Test admin connection
	if err := adminDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping admin database: %w", err)
	}

	// Check if target database exists
	var exists bool
	checkQuery := "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)"
	if err := adminDB.QueryRow(checkQuery, cfg.Database).Scan(&exists); err != nil {
		return nil, fmt.Errorf("failed to check database existence: %w", err)
	}

	// Create database if it doesn't exist
	if !exists {
		createQuery := fmt.Sprintf("CREATE DATABASE %s", cfg.Database)
		if _, err := adminDB.Exec(createQuery); err != nil {
			return nil, fmt.Errorf("failed to create database %s: %w", cfg.Database, err)
		}
	}

	// Now connect to the target database
	dsn := fmt.Sprintf("postgres://%s%s@%s:%d/%s?sslmode=%s",
		cfg.User, passwordPart, cfg.Host, cfg.Port, cfg.Database, sslMode)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection and verify we're connected to the correct database
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Verify we're connected to the correct database (important for YugabyteDB)
	var actualDB string
	if err := db.QueryRow("SELECT current_database()").Scan(&actualDB); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to verify database: %w", err)
	}
	if actualDB != cfg.Database {
		db.Close()
		return nil, fmt.Errorf("connected to wrong database: expected=%s, actual=%s", cfg.Database, actualDB)
	}

	// Set connection pool settings
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25)
	}

	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5)
	}

	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(5 * time.Minute)
	}

	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	} else {
		db.SetConnMaxIdleTime(5 * time.Minute)
	}

	return db, nil
}
