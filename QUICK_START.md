# Database Adapter - Quick Start Guide

Get started with the database adapter in 5 minutes!

## Prerequisites

- Go 1.21+
- PostgreSQL 15+ OR MongoDB 6.0+
- Basic understanding of the EIR project

## Option 1: PostgreSQL (Recommended for Getting Started)

### Step 1: Install PostgreSQL

```bash
# macOS
brew install postgresql@15
brew services start postgresql@15

# Ubuntu/Debian
sudo apt install postgresql-15
sudo systemctl start postgresql

# Docker
docker run --name eir-postgres -e POSTGRES_PASSWORD=eir_password -p 5432:5432 -d postgres:15
```

### Step 2: Create Database

```bash
# Create database and user
createdb eir
psql -d postgres -c "CREATE USER eir WITH PASSWORD 'eir_password';"
psql -d postgres -c "GRANT ALL PRIVILEGES ON DATABASE eir TO eir;"

# Run schema
psql -U eir -d eir -f internal/adapters/postgres/schema.sql
psql -U eir -d eir -f internal/adapters/postgres/schema_extended.sql
```

### Step 3: Install Go Dependencies

```bash
go get github.com/lib/pq
go get github.com/jmoiron/sqlx
```

### Step 4: Run Example

```bash
go run examples/database_adapter_example.go -db=postgres -action=demo
```

You should see output like:
```
✓ Connected to postgres database
✓ Health check passed
✓ Connection stats: 1/25 connections (healthy: true)

=== Running Demo Scenario ===

1. Creating equipment...
  ✓ Created equipment ID: 1

2. Performing equipment checks with audit logging...
  ✓ Logged check #1 (audit ID: 1)
  ✓ Logged check #2 (audit ID: 2)
  ✓ Logged check #3 (audit ID: 3)

...
```

## Option 2: MongoDB

### Step 1: Install MongoDB

```bash
# macOS
brew tap mongodb/brew
brew install mongodb-community@6.0
brew services start mongodb-community@6.0

# Ubuntu/Debian
wget -qO - https://www.mongodb.org/static/pgp/server-6.0.asc | sudo apt-key add -
echo "deb [ arch=amd64,arm64 ] https://repo.mongodb.org/apt/ubuntu focal/mongodb-org/6.0 multiverse" | sudo tee /etc/apt/sources.list.d/mongodb-org-6.0.list
sudo apt update
sudo apt install mongodb-org
sudo systemctl start mongod

# Docker
docker run --name eir-mongo -p 27017:27017 -d mongo:6.0
```

### Step 2: Initialize Database

```bash
# Connect to MongoDB
mongosh

# Switch to eir database
use eir;

# Copy and paste the initialization script from:
# internal/adapters/mongodb/SCHEMA.md
# (Look for the section: "Initialization Script")
```

### Step 3: Install Go Dependencies

```bash
go get go.mongodb.org/mongo-driver/mongo
```

### Step 4: Run Example

```bash
go run examples/database_adapter_example.go -db=mongodb -action=demo
```

## Basic Usage in Your Code

### Simple Example

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/hsdfat8/eir/internal/adapters/factory"
    "github.com/hsdfat8/eir/internal/domain/models"
    "github.com/hsdfat8/eir/internal/domain/ports"
)

func main() {
    ctx := context.Background()

    // 1. Create configuration
    config := &ports.DatabaseConfig{
        Type: ports.DatabaseTypePostgreSQL, // or DatabaseTypeMongoDB
        PostgresConfig: &ports.PostgresConfig{
            Host:     "localhost",
            Port:     5432,
            User:     "eir",
            Password: "eir_password",
            Database: "eir",
            SSLMode:  "disable",
            MaxOpenConns: 25,
            MaxIdleConns: 5,
            ConnMaxLifetime: 300,
            ConnMaxIdleTime: 600,
            QueryTimeout: 30,
        },
    }

    // 2. Create adapter
    dbFactory := factory.NewDatabaseAdapterFactory()
    adapter, err := dbFactory.CreateAndConnectAdapter(ctx, config)
    if err != nil {
        log.Fatal(err)
    }
    defer adapter.Disconnect(ctx)

    // 3. Use repositories
    imeiRepo := adapter.GetIMEIRepository()
    auditRepo := adapter.GetAuditRepository()

    // 4. Create equipment
    equipment := &models.Equipment{
        IMEI:    "123456789012345",
        Status:  models.EquipmentStatusWhitelisted,
        AddedBy: "admin",
        LastUpdated: time.Now(),
    }
    imeiRepo.Create(ctx, equipment)

    // 5. Check equipment
    found, _ := imeiRepo.GetByIMEI(ctx, "123456789012345")
    log.Printf("Equipment status: %s", found.Status)

    // 6. Log audit
    audit := &models.AuditLog{
        IMEI:          equipment.IMEI,
        Status:        equipment.Status,
        CheckTime:     time.Now(),
        RequestSource: "HTTP_5G",
    }
    auditRepo.LogCheck(ctx, audit)

    log.Println("Success!")
}
```

## Common Operations

### Check Equipment with Audit

```go
func checkEquipment(ctx context.Context, adapter ports.DatabaseAdapter, imei string) error {
    // Get equipment
    equipment, err := adapter.GetIMEIRepository().GetByIMEI(ctx, imei)
    if err != nil {
        return err
    }

    // Log audit
    audit := &models.AuditLog{
        IMEI:          imei,
        Status:        equipment.Status,
        CheckTime:     time.Now(),
        RequestSource: "HTTP_5G",
    }

    err = adapter.GetAuditRepository().LogCheck(ctx, audit)
    if err != nil {
        return err
    }

    // Increment check count
    return adapter.GetIMEIRepository().IncrementCheckCount(ctx, imei)
}
```

### Update Equipment with History

```go
func updateEquipment(ctx context.Context, adapter ports.DatabaseAdapter, imei string, newStatus models.EquipmentStatus) error {
    tx, _ := adapter.BeginTransaction(ctx)
    defer tx.Rollback(ctx)

    // Get equipment
    equipment, _ := tx.GetIMEIRepository().GetByIMEI(ctx, imei)

    // Update
    equipment.Status = newStatus
    equipment.LastUpdated = time.Now()
    tx.GetIMEIRepository().Update(ctx, equipment)

    // Log
    audit := &models.AuditLog{
        IMEI:          imei,
        Status:        newStatus,
        CheckTime:     time.Now(),
        RequestSource: "ADMIN_UPDATE",
    }
    tx.GetAuditRepository().LogCheck(ctx, audit)

    return tx.Commit(ctx)
}
```

### Query History

```go
func getEquipmentHistory(ctx context.Context, adapter ports.DatabaseAdapter, imei string) {
    // Get change history
    history, _ := adapter.GetHistoryRepository().GetHistoryByIMEI(ctx, imei, 0, 10)

    for _, h := range history {
        log.Printf("%s: %s -> %s (by %s)",
            h.ChangedAt.Format("2006-01-02 15:04:05"),
            *h.PreviousStatus, h.NewStatus, h.ChangedBy)
    }

    // Get audit logs
    audits, _ := adapter.GetAuditRepository().GetAuditsByIMEI(ctx, imei, 0, 10)

    for _, a := range audits {
        log.Printf("%s: Check from %s - %s",
            a.CheckTime.Format("2006-01-02 15:04:05"),
            a.RequestSource, a.Status)
    }
}
```

## Switch Between Databases

Just change the configuration:

```go
// PostgreSQL
config := &ports.DatabaseConfig{
    Type: ports.DatabaseTypePostgreSQL,
    PostgresConfig: factory.GetDefaultPostgresConfig(),
}

// MongoDB
config := &ports.DatabaseConfig{
    Type: ports.DatabaseTypeMongoDB,
    MongoDBConfig: factory.GetDefaultMongoDBConfig(),
}
```

The rest of your code stays the same!

## Environment Variables

```bash
# PostgreSQL
export EIR_DB_TYPE=postgres
export EIR_POSTGRES_HOST=localhost
export EIR_POSTGRES_PORT=5432
export EIR_POSTGRES_USER=eir
export EIR_POSTGRES_PASSWORD=eir_password
export EIR_POSTGRES_DATABASE=eir

# MongoDB
export EIR_DB_TYPE=mongodb
export EIR_MONGODB_URI=mongodb://localhost:27017
export EIR_MONGODB_DATABASE=eir
```

## Configuration File

Create `config/database.yaml`:

```yaml
database:
  type: postgres

  postgres:
    host: localhost
    port: 5432
    user: eir
    password: eir_password
    database: eir
    ssl_mode: disable
    max_open_conns: 25
    max_idle_conns: 5
    conn_max_lifetime: 300
    conn_max_idle_time: 600
    query_timeout: 30
```

## Testing the Setup

### Test PostgreSQL Connection

```bash
psql -U eir -d eir -c "SELECT COUNT(*) FROM equipment;"
```

### Test MongoDB Connection

```bash
mongosh eir --eval "db.equipment.countDocuments()"
```

### Run All Example Actions

```bash
# Demo scenario
go run examples/database_adapter_example.go -db=postgres -action=demo

# Get statistics
go run examples/database_adapter_example.go -db=postgres -action=stats

# Cleanup old data
go run examples/database_adapter_example.go -db=postgres -action=cleanup

# Migrate data
go run examples/database_adapter_example.go -action=migrate
```

## Troubleshooting

### PostgreSQL Connection Issues

```bash
# Check if PostgreSQL is running
psql -U postgres -c "SELECT version();"

# Check if eir database exists
psql -U postgres -c "\l" | grep eir

# Check if user has permissions
psql -U postgres -c "\du" | grep eir
```

### MongoDB Connection Issues

```bash
# Check if MongoDB is running
mongosh --eval "db.adminCommand('ping')"

# Check if eir database exists
mongosh --eval "show dbs" | grep eir

# Check collections
mongosh eir --eval "show collections"
```

### Common Errors

**Error: "pq: password authentication failed"**
```bash
# Reset password
psql -U postgres -c "ALTER USER eir WITH PASSWORD 'eir_password';"
```

**Error: "no reachable servers"**
```bash
# Check MongoDB is running
sudo systemctl status mongod
# or
brew services list | grep mongodb
```

**Error: "table does not exist"**
```bash
# Run schema scripts
psql -U eir -d eir -f internal/adapters/postgres/schema.sql
psql -U eir -d eir -f internal/adapters/postgres/schema_extended.sql
```

## Next Steps

1. **Read the comprehensive guide**: [DATABASE_ADAPTER_GUIDE.md](DATABASE_ADAPTER_GUIDE.md)
2. **Review the architecture**: [ARCHITECTURE_DIAGRAM.md](ARCHITECTURE_DIAGRAM.md)
3. **Check the implementation summary**: [IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md)
4. **Study the example code**: [examples/database_adapter_example.go](examples/database_adapter_example.go)
5. **Integrate into your application**: Use the adapter in your EIR service

## Helpful Commands

```bash
# PostgreSQL: View tables
psql -U eir -d eir -c "\dt"

# PostgreSQL: View indexes
psql -U eir -d eir -c "\di"

# PostgreSQL: View partitions
psql -U eir -d eir -c "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename;"

# MongoDB: View collections
mongosh eir --eval "show collections"

# MongoDB: View indexes
mongosh eir --eval "db.equipment.getIndexes()"

# MongoDB: Count documents
mongosh eir --eval "db.equipment.countDocuments()"
```

## Performance Tips

1. **PostgreSQL**: Run `VACUUM ANALYZE` regularly
2. **MongoDB**: Monitor index usage with `explain()`
3. **Both**: Configure connection pool sizes based on load
4. **Both**: Use transactions for multi-operation atomicity
5. **Both**: Regularly purge old audit logs

## Support

- Check the comprehensive documentation
- Review example code
- Test with the provided example program
- Verify database setup with test queries

---

**You're ready to go!** 🚀

Start with the demo, explore the examples, and integrate into your EIR service.
