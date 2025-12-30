package postgres

import (
	"context"
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
	// Use URL format for DSN (better compatibility with YugabyteDB than key=value format)
	passwordPart := ""
	if cfg.Password != "" {
		passwordPart = ":" + cfg.Password
	}
	dsn := fmt.Sprintf("postgres://%s%s@%s:%d/%s?sslmode=%s",
		cfg.User,
		passwordPart,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.SSLMode,
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
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

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
