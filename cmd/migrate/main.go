package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hsdfat8/eir/internal/adapters/postgres"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

func main() {
	var (
		databaseURL     = flag.String("database-url", "", "PostgreSQL connection string (overrides individual flags)")
		host            = flag.String("host", "localhost", "Database host")
		port            = flag.Int("port", 5432, "Database port")
		user            = flag.String("user", "eir", "Database user")
		password        = flag.String("password", "eir_password", "Database password")
		dbname          = flag.String("dbname", "eir", "Database name")
		sslmode         = flag.String("sslmode", "disable", "SSL mode (disable, require, verify-ca, verify-full)")
		verify          = flag.Bool("verify", false, "Verify schema after migration")
		createPartition = flag.Int("create-partition", 0, "Create audit_log partitions for a specific year (e.g., 2025)")
		status          = flag.Bool("status", false, "Show migration status")
	)

	flag.Parse()

	// Build connection string
	var dsn string
	if *databaseURL != "" {
		dsn = *databaseURL
	} else {
		dsn = fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			*host, *port, *user, *password, *dbname, *sslmode,
		)
	}

	// Connect to database
	fmt.Println("Connecting to database...")
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to ping database: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully connected to database!")

	// Create migrator
	migrator := postgres.NewMigrator(db)

	// Handle different commands
	switch {
	case *status:
		if err := showMigrationStatus(ctx, migrator); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get migration status: %v\n", err)
			os.Exit(1)
		}

	case *createPartition > 0:
		if err := migrator.CreatePartitionsForYear(ctx, *createPartition); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create partitions: %v\n", err)
			os.Exit(1)
		}

	default:
		// Run migration
		if err := migrator.Migrate(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
			os.Exit(1)
		}

		// Verify schema if requested
		if *verify {
			if err := migrator.VerifySchema(ctx); err != nil {
				fmt.Fprintf(os.Stderr, "Schema verification failed: %v\n", err)
				os.Exit(1)
			}
		}
	}

	fmt.Println("\n✓ All operations completed successfully!")
}

func showMigrationStatus(ctx context.Context, migrator *postgres.Migrator) error {
	fmt.Println("\nMigration Status:")
	fmt.Println("================")

	migrations, err := migrator.GetMigrationStatus(ctx)
	if err != nil {
		return err
	}

	if len(migrations) == 0 {
		fmt.Println("No migrations have been applied yet.")
		return nil
	}

	for _, m := range migrations {
		fmt.Printf("\n✓ %s\n", m.MigrationName)
		fmt.Printf("  Description: %s\n", m.Description)
		fmt.Printf("  Applied at:  %s\n", m.AppliedAt.Format(time.RFC3339))
	}

	return nil
}
