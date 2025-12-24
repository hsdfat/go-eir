# Quick Migration Guide

## Auto-Migration System for EIR Database

This project includes an auto-migration system that automatically creates and manages the PostgreSQL database schema.

### Quick Start

#### Option 1: Using Make (Recommended)

```bash
# Run migration
make migrate

# Run with verification
make migrate-verify

# Check migration status
make migrate-status
```

#### Option 2: Using the Shell Script

```bash
# Run migration
./scripts/migrate.sh

# Run with verification
./scripts/migrate.sh --verify

# Check status
./scripts/migrate.sh --status
```

#### Option 3: Using the CLI Tool Directly

```bash
# Build and run
go build -o bin/migrate ./cmd/migrate
./bin/migrate -database-url="host=14.225.198.206 user=adong password=adong123 dbname=adongfoodv4 port=5432 sslmode=disable"
```

### Database Configuration

The default database URL is configured in the [Makefile](Makefile):

```makefile
DATABASE_URL = "host=14.225.198.206 user=adong password=adong123 dbname=adongfoodv4 port=5432 sslmode=disable"
```

To use a different database, either:
1. Edit the `DATABASE_URL` in the Makefile
2. Set it as an environment variable: `export DATABASE_URL="..."`
3. Pass it as a flag: `./bin/migrate -database-url="..."`

### What Gets Created

The migration creates:

**Tables:**
- `equipment` - Main IMEI/equipment records
- `audit_log` - Partitioned audit logs (quarterly)
- `schema_migrations` - Migration tracking

**Indexes:**
- Optimized indexes on IMEI, status, timestamps
- GIN index on JSONB metadata

**Functions & Triggers:**
- Auto-update timestamps
- Atomic check counter increment

**Views:**
- `hot_equipment` - Recently accessed equipment
- `equipment_statistics` - Status metrics

**Extensions:**
- `uuid-ossp` - UUID functions
- `pg_trgm` - Text search optimization

### Advanced Usage

#### Create Partitions for Future Years

```bash
# Using Make
make migrate-create-partition YEAR=2028

# Using script
./scripts/migrate.sh --create-partition 2028

# Using CLI
./bin/migrate -database-url="..." -create-partition=2028
```

#### Verify Schema

```bash
make migrate-verify
```

Checks for:
- Required PostgreSQL extensions
- All tables, functions, and views
- Proper schema structure

#### Check Migration Status

```bash
make migrate-status
```

Shows:
- All applied migrations
- Timestamps
- Descriptions

### Features

- **Idempotent**: Safe to run multiple times
- **Tracking**: Records all migrations in `schema_migrations` table
- **Verification**: Optional schema validation
- **Partition Management**: Automatic quarterly partitions for audit logs
- **Zero Downtime**: Uses `CREATE IF NOT EXISTS` patterns

### Example Output

```bash
$ make migrate
Building migration tool...
Migration tool built: bin/migrate
Running database migration with auto-migrate...
Connecting to database...
Successfully connected to database!
Starting database migration...
Applying initial schema...
Database migration completed successfully!

✓ All operations completed successfully!
```

### Testing

The migration was successfully tested with:
- **Host**: 14.225.198.206
- **Database**: adongfoodv4
- **User**: adong
- **Status**: ✓ All tables, functions, and views created successfully

### Troubleshooting

**Connection failed?**
- Check database is running and accessible
- Verify credentials
- Check firewall rules

**Permission denied?**
- Ensure user has `CREATE` privileges
- Grant necessary permissions:
  ```sql
  GRANT CREATE ON DATABASE dbname TO username;
  GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO username;
  ```

**Migration already applied?**
- The system is idempotent - it will skip already applied migrations
- Check status: `make migrate-status`

### Integration

To run migrations programmatically:

```go
import (
    "github.com/hsdfat8/eir/internal/adapters/postgres"
    "github.com/jmoiron/sqlx"
)

// In your app initialization
db, _ := sqlx.Connect("postgres", databaseURL)
migrator := postgres.NewMigrator(db)
if err := migrator.Migrate(ctx); err != nil {
    log.Fatal(err)
}
```

### Full Documentation

See [docs/MIGRATION.md](docs/MIGRATION.md) for complete documentation.

### Files Created

- [internal/adapters/postgres/migrator.go](internal/adapters/postgres/migrator.go) - Migration logic
- [cmd/migrate/main.go](cmd/migrate/main.go) - CLI tool
- [scripts/migrate.sh](scripts/migrate.sh) - Shell script wrapper
- [Makefile](Makefile) - Make targets
- [docs/MIGRATION.md](docs/MIGRATION.md) - Full documentation
