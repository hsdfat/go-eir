# Database Migration Guide

This guide explains how to use the auto-migration system for the EIR (Equipment Identity Register) database.

## Overview

The migration system provides:
- **Automatic schema deployment** from `schema.sql`
- **Migration tracking** to avoid re-running migrations
- **Schema verification** to ensure all objects are created correctly
- **Partition management** for the audit_log table
- **Migration status reporting**

## Quick Start

### Using Make (Recommended)

```bash
# Run migration with the default database
make migrate

# Run migration with verification
make migrate-verify

# Check migration status
make migrate-status

# Create partitions for a specific year
make migrate-create-partition YEAR=2027
```

### Using the Shell Script

```bash
# Run migration
./scripts/migrate.sh

# Run migration with verification
./scripts/migrate.sh --verify

# Check status
./scripts/migrate.sh --status

# Create partitions
./scripts/migrate.sh --create-partition 2027

# Use custom database URL
DATABASE_URL="host=localhost port=5432 user=myuser password=mypass dbname=mydb sslmode=disable" ./scripts/migrate.sh
```

### Using the CLI Tool Directly

First, build the tool:
```bash
go build -o bin/migrate ./cmd/migrate
```

Then run it:
```bash
# Basic migration
./bin/migrate -database-url="host=localhost port=5432 user=eir password=eir_password dbname=eir sslmode=disable"

# With verification
./bin/migrate -database-url="..." -verify

# Check status
./bin/migrate -database-url="..." -status

# Create partitions
./bin/migrate -database-url="..." -create-partition=2027
```

## Database Configuration

### Using DATABASE_URL Environment Variable

Set the `DATABASE_URL` in your Makefile or as an environment variable:

```bash
export DATABASE_URL="host=14.225.198.206 user=adong password=adong123 dbname=adongfoodv4 port=5432 sslmode=disable"
make migrate
```

### Using Individual Flags

```bash
./bin/migrate \
  -host=14.225.198.206 \
  -port=5432 \
  -user=adong \
  -password=adong123 \
  -dbname=adongfoodv4 \
  -sslmode=disable
```

## What Gets Created

The migration creates the following database objects:

### Extensions
- `uuid-ossp` - UUID generation functions
- `pg_trgm` - Trigram-based text search optimization

### Tables
- `equipment` - Main IMEI/equipment storage with validation constraints
- `audit_log` - Partitioned table for check operation logs (quarterly partitions)
- `schema_migrations` - Migration tracking table

### Indexes
- Efficient indexes on IMEI, status, timestamps, and metadata fields
- GIN index on JSONB metadata for fast JSON queries

### Functions
- `update_last_updated_column()` - Auto-updates timestamps on changes
- `increment_equipment_check_count()` - Atomic counter increment

### Triggers
- `update_equipment_last_updated` - Automatically updates timestamps

### Views
- `hot_equipment` - Frequently accessed equipment (last 7 days)
- `equipment_statistics` - Status distribution and activity metrics

## Migration Status

The system tracks all applied migrations in the `schema_migrations` table:

```bash
make migrate-status
```

Output:
```
Migration Status:
================

✓ initial_schema
  Description: Applied initial database schema from schema.sql
  Applied at:  2024-01-15T10:30:45Z

✓ partitions_2025
  Description: Created audit_log partitions for year 2025
  Applied at:  2024-01-15T10:31:12Z
```

## Partition Management

The `audit_log` table uses **range partitioning by quarter** for optimal performance:

### Pre-created Partitions
- 2024 Q1-Q4
- 2025 Q1-Q4
- 2026 Q1

### Creating Additional Partitions

For new years, create partitions in advance:

```bash
# Using Make
make migrate-create-partition YEAR=2027

# Using script
./scripts/migrate.sh --create-partition 2027

# Using CLI
./bin/migrate -database-url="..." -create-partition=2027
```

This creates four quarterly partitions:
- `audit_log_2027_q1` (Jan-Mar)
- `audit_log_2027_q2` (Apr-Jun)
- `audit_log_2027_q3` (Jul-Sep)
- `audit_log_2027_q4` (Oct-Dec)

## Schema Verification

Verify that all required objects exist:

```bash
make migrate-verify
```

This checks for:
- Required PostgreSQL extensions
- All tables (equipment, audit_log, etc.)
- Database functions
- Views

Output:
```
Verifying database schema...
✓ Extension uuid-ossp is installed
✓ Extension pg_trgm is installed
✓ Table equipment exists
✓ Table audit_log exists
✓ Table schema_migrations exists
✓ Function update_last_updated_column exists
✓ Function increment_equipment_check_count exists
✓ View hot_equipment exists
✓ View equipment_statistics exists
Schema verification completed successfully!
```

## Idempotency

The migration system is **idempotent** - you can run it multiple times safely:

1. First run: Creates all objects and records the migration
2. Subsequent runs: Checks migration tracking table and skips if already applied
3. All SQL uses `CREATE ... IF NOT EXISTS` for safety

## Troubleshooting

### Connection Issues

If you get connection errors:

```bash
# Test connection manually
psql -h 14.225.198.206 -U adong -d adongfoodv4 -p 5432

# Check if PostgreSQL is running
pg_isready -h 14.225.198.206 -p 5432
```

### Permission Issues

Ensure your database user has these permissions:
```sql
GRANT CREATE ON DATABASE adongfoodv4 TO adong;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO adong;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO adong;
```

### Migration Already Applied

If migration is already applied and you want to re-run:

```sql
-- Check current migrations
SELECT * FROM schema_migrations;

-- Delete migration record (use with caution!)
DELETE FROM schema_migrations WHERE migration_name = 'initial_schema';
```

## Integration with Application

To integrate auto-migration into your application startup:

```go
import (
    "github.com/hsdfat8/eir/internal/adapters/postgres"
    "github.com/jmoiron/sqlx"
)

func initDatabase() error {
    // Connect to database
    db, err := sqlx.Connect("postgres", databaseURL)
    if err != nil {
        return err
    }

    // Run migrations
    migrator := postgres.NewMigrator(db)
    if err := migrator.Migrate(context.Background()); err != nil {
        return fmt.Errorf("migration failed: %w", err)
    }

    return nil
}
```

## CI/CD Integration

### Docker Compose Example

```yaml
version: '3.8'
services:
  migrate:
    build: .
    command: ./bin/migrate -database-url="${DATABASE_URL}"
    depends_on:
      - postgres
    environment:
      - DATABASE_URL=host=postgres port=5432 user=eir password=eir dbname=eir sslmode=disable
```

### GitHub Actions Example

```yaml
- name: Run Database Migration
  run: make migrate
  env:
    DATABASE_URL: "host=${{ secrets.DB_HOST }} user=${{ secrets.DB_USER }} password=${{ secrets.DB_PASSWORD }} dbname=${{ secrets.DB_NAME }} port=5432 sslmode=require"
```

## Best Practices

1. **Always backup before migration** in production
2. **Test migrations** in staging environment first
3. **Create partitions** in advance (at least 1 year ahead)
4. **Monitor partition usage** and create new ones proactively
5. **Use verification** to ensure successful deployment
6. **Track migration status** for audit purposes

## Support

For issues or questions:
- Check logs in the migration output
- Verify database connectivity
- Ensure PostgreSQL version >= 12
- Check user permissions
