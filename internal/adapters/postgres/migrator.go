package postgres

import (
	"context"
	"embed"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

//go:embed schema.sql
var schemaFS embed.FS

// Migrator handles database schema migrations
type Migrator struct {
	db *sqlx.DB
}

// NewMigrator creates a new database migrator
func NewMigrator(db *sqlx.DB) *Migrator {
	return &Migrator{db: db}
}

// Migrate runs all necessary database migrations
func (m *Migrator) Migrate(ctx context.Context) error {
	fmt.Println("Starting database migration...")

	// Create migration tracking table if it doesn't exist
	if err := m.createMigrationTable(ctx); err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	// Check if schema has already been applied
	applied, err := m.isMigrationApplied(ctx, "initial_schema")
	if err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	if applied {
		fmt.Println("Initial schema already applied, skipping...")
		return nil
	}

	// Read and execute the schema.sql file
	schemaSQL, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema.sql: %w", err)
	}

	fmt.Println("Applying initial schema...")

	// Execute the schema in a transaction
	tx, err := m.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute the schema SQL
	if _, err := tx.ExecContext(ctx, string(schemaSQL)); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	// Record the migration
	if err := m.recordMigration(ctx, tx, "initial_schema", "Applied initial database schema from schema.sql"); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	fmt.Println("Database migration completed successfully!")
	return nil
}

// createMigrationTable creates the migrations tracking table
func (m *Migrator) createMigrationTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id SERIAL PRIMARY KEY,
			migration_name VARCHAR(255) NOT NULL UNIQUE,
			description TEXT,
			applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			checksum VARCHAR(64)
		);
		CREATE INDEX IF NOT EXISTS idx_migrations_name ON schema_migrations(migration_name);
	`

	_, err := m.db.ExecContext(ctx, query)
	return err
}

// isMigrationApplied checks if a migration has already been applied
func (m *Migrator) isMigrationApplied(ctx context.Context, migrationName string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM schema_migrations WHERE migration_name = $1`
	err := m.db.GetContext(ctx, &count, query, migrationName)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// recordMigration records a migration in the tracking table
func (m *Migrator) recordMigration(ctx context.Context, tx *sqlx.Tx, migrationName, description string) error {
	query := `
		INSERT INTO schema_migrations (migration_name, description, applied_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (migration_name) DO NOTHING
	`
	_, err := tx.ExecContext(ctx, query, migrationName, description, time.Now())
	return err
}

// GetMigrationStatus returns the status of all applied migrations
func (m *Migrator) GetMigrationStatus(ctx context.Context) ([]MigrationRecord, error) {
	var migrations []MigrationRecord
	query := `
		SELECT migration_name, description, applied_at
		FROM schema_migrations
		ORDER BY applied_at DESC
	`
	err := m.db.SelectContext(ctx, &migrations, query)
	return migrations, err
}

// MigrationRecord represents a migration record
type MigrationRecord struct {
	MigrationName string    `db:"migration_name"`
	Description   string    `db:"description"`
	AppliedAt     time.Time `db:"applied_at"`
}

// CreatePartitionsForYear creates audit_log partitions for a specific year
func (m *Migrator) CreatePartitionsForYear(ctx context.Context, year int) error {
	quarters := []struct {
		name  string
		start string
		end   string
	}{
		{fmt.Sprintf("audit_log_%d_q1", year), fmt.Sprintf("%d-01-01", year), fmt.Sprintf("%d-04-01", year)},
		{fmt.Sprintf("audit_log_%d_q2", year), fmt.Sprintf("%d-04-01", year), fmt.Sprintf("%d-07-01", year)},
		{fmt.Sprintf("audit_log_%d_q3", year), fmt.Sprintf("%d-07-01", year), fmt.Sprintf("%d-10-01", year)},
		{fmt.Sprintf("audit_log_%d_q4", year), fmt.Sprintf("%d-10-01", year), fmt.Sprintf("%d-01-01", year+1)},
	}

	for _, q := range quarters {
		query := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s PARTITION OF audit_log
			FOR VALUES FROM ('%s') TO ('%s')
		`, q.name, q.start, q.end)

		fmt.Printf("Creating partition: %s...\n", q.name)
		if _, err := m.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to create partition %s: %w", q.name, err)
		}
	}

	// Record the migration
	migrationName := fmt.Sprintf("partitions_%d", year)
	description := fmt.Sprintf("Created audit_log partitions for year %d", year)

	applied, err := m.isMigrationApplied(ctx, migrationName)
	if err == nil && !applied {
		query := `
			INSERT INTO schema_migrations (migration_name, description, applied_at)
			VALUES ($1, $2, $3)
		`
		_, _ = m.db.ExecContext(ctx, query, migrationName, description, time.Now())
	}

	fmt.Printf("Successfully created partitions for year %d\n", year)
	return nil
}

// VerifySchema verifies that all required tables and extensions exist
func (m *Migrator) VerifySchema(ctx context.Context) error {
	fmt.Println("Verifying database schema...")

	// Check extensions
	extensions := []string{"uuid-ossp", "pg_trgm"}
	for _, ext := range extensions {
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = $1)`
		if err := m.db.GetContext(ctx, &exists, query, ext); err != nil {
			return fmt.Errorf("failed to check extension %s: %w", ext, err)
		}
		if !exists {
			return fmt.Errorf("extension %s is not installed", ext)
		}
		fmt.Printf("✓ Extension %s is installed\n", ext)
	}

	// Check tables
	tables := []string{
		"equipment",
		"audit_log",
		"schema_migrations",
	}

	for _, table := range tables {
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = $1)`
		if err := m.db.GetContext(ctx, &exists, query, table); err != nil {
			return fmt.Errorf("failed to check table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("table %s does not exist", table)
		}
		fmt.Printf("✓ Table %s exists\n", table)
	}

	// Check functions
	functions := []string{
		"update_last_updated_column",
		"increment_equipment_check_count",
	}

	for _, fn := range functions {
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM pg_proc WHERE proname = $1)`
		if err := m.db.GetContext(ctx, &exists, query, fn); err != nil {
			return fmt.Errorf("failed to check function %s: %w", fn, err)
		}
		if !exists {
			return fmt.Errorf("function %s does not exist", fn)
		}
		fmt.Printf("✓ Function %s exists\n", fn)
	}

	// Check views
	views := []string{
		"hot_equipment",
		"equipment_statistics",
	}

	for _, view := range views {
		var exists bool
		query := `SELECT EXISTS(SELECT 1 FROM information_schema.views WHERE table_name = $1)`
		if err := m.db.GetContext(ctx, &exists, query, view); err != nil {
			return fmt.Errorf("failed to check view %s: %w", view, err)
		}
		if !exists {
			return fmt.Errorf("view %s does not exist", view)
		}
		fmt.Printf("✓ View %s exists\n", view)
	}

	fmt.Println("Schema verification completed successfully!")
	return nil
}
