# Database Adapter Implementation - EIR Project

## Overview

This implementation provides a comprehensive database abstraction layer for the Equipment Identity Register (EIR) project, supporting both **PostgreSQL** and **MongoDB** with full feature parity, enhanced audit tracking, and change history management.

## Features Implemented ✅

### Core Database Support
- ✅ **Dual Database Support**: PostgreSQL and MongoDB with unified interface
- ✅ **Repository Pattern**: Clean separation of concerns using Hexagonal Architecture
- ✅ **Factory Pattern**: Easy creation and configuration of database adapters
- ✅ **Transaction Support**: Atomic operations for both databases
- ✅ **Connection Pooling**: Efficient resource management
- ✅ **Health Monitoring**: Connection stats and health checks

### Enhanced Audit & History Tracking
- ✅ **Basic Audit Logging**: All equipment checks are logged
- ✅ **Extended Audit Logs**: IP addresses, user agents, processing metrics
- ✅ **Equipment History**: Complete change tracking (CREATE, UPDATE, DELETE, CHECK)
- ✅ **Point-in-time Snapshots**: State capture for rollback and compliance
- ✅ **Automatic History Triggers**: PostgreSQL auto-records changes
- ✅ **Audit Statistics**: Aggregated analytics and reporting

### Data Management
- ✅ **CRUD Operations**: Full create, read, update, delete support
- ✅ **Pagination**: Efficient listing with offset/limit
- ✅ **Filtering**: Query by status, time range, request source
- ✅ **Atomic Counters**: Safe concurrent check count updates
- ✅ **Data Purging**: Cleanup of old audit and history data
- ✅ **Database Optimization**: VACUUM (PostgreSQL) and Compact (MongoDB)

### PostgreSQL Specific
- ✅ **Table Partitioning**: Quarterly partitions for audit logs
- ✅ **Triggers & Functions**: Automatic snapshot creation and history tracking
- ✅ **Views**: Pre-computed statistics and hot equipment tracking
- ✅ **JSONB Support**: Flexible metadata storage with GIN indexes
- ✅ **Stored Procedures**: Atomic check count increment

### MongoDB Specific
- ✅ **Document Validation**: Schema enforcement with validators
- ✅ **Compound Indexes**: Optimized for common query patterns
- ✅ **TTL Indexes**: Optional automatic data expiration
- ✅ **Change Streams**: Real-time change notifications (optional)
- ✅ **Aggregation Pipelines**: Complex statistics queries
- ✅ **Sharding Ready**: Hash-based sharding support

## Project Structure

```
go-eir/
├── internal/
│   ├── domain/
│   │   ├── models/
│   │   │   ├── equipment.go              # Core domain models
│   │   │   └── history.go                # NEW: History & snapshot models
│   │   └── ports/
│   │       ├── repository.go             # Base repository interfaces
│   │       ├── database_adapter.go       # NEW: Unified adapter interface
│   │       └── history_repository.go     # NEW: History & snapshot interfaces
│   │
│   └── adapters/
│       ├── postgres/
│       │   ├── postgres_adapter.go       # NEW: PostgreSQL adapter
│       │   ├── imei_repository.go        # EXISTING
│       │   ├── audit_repository.go       # EXISTING
│       │   ├── extended_audit_repository.go  # NEW
│       │   ├── history_repository.go     # NEW
│       │   ├── snapshot_repository.go    # NEW
│       │   ├── schema.sql                # EXISTING
│       │   └── schema_extended.sql       # NEW: Extended tables & triggers
│       │
│       ├── mongodb/
│       │   ├── mongodb_adapter.go        # NEW: MongoDB adapter
│       │   ├── imei_repository.go        # NEW
│       │   ├── audit_repository.go       # NEW
│       │   ├── extended_audit_repository.go  # NEW
│       │   ├── history_repository.go     # NEW
│       │   ├── snapshot_repository.go    # NEW
│       │   └── SCHEMA.md                 # NEW: Schema documentation
│       │
│       └── factory/
│           └── database_factory.go       # NEW: Adapter factory
│
├── config/
│   └── database.yaml                     # NEW: Database configuration
│
├── examples/
│   └── database_adapter_example.go       # NEW: Complete usage examples
│
├── DATABASE_ADAPTER_GUIDE.md             # NEW: Comprehensive guide
└── DATABASE_ADAPTER_README.md            # This file
```

## Files Created

### Domain Layer (Models & Ports)
1. **internal/domain/models/history.go** - New history and snapshot models
2. **internal/domain/ports/database_adapter.go** - Unified database adapter interface
3. **internal/domain/ports/history_repository.go** - History and snapshot repository interfaces

### PostgreSQL Adapter
4. **internal/adapters/postgres/postgres_adapter.go** - Main PostgreSQL adapter
5. **internal/adapters/postgres/extended_audit_repository.go** - Extended audit implementation
6. **internal/adapters/postgres/history_repository.go** - Change history tracking
7. **internal/adapters/postgres/snapshot_repository.go** - Snapshot management
8. **internal/adapters/postgres/schema_extended.sql** - Extended database schema

### MongoDB Adapter
9. **internal/adapters/mongodb/mongodb_adapter.go** - Main MongoDB adapter
10. **internal/adapters/mongodb/imei_repository.go** - Equipment repository
11. **internal/adapters/mongodb/audit_repository.go** - Audit logging
12. **internal/adapters/mongodb/extended_audit_repository.go** - Extended audit
13. **internal/adapters/mongodb/history_repository.go** - Change history
14. **internal/adapters/mongodb/snapshot_repository.go** - Snapshot management
15. **internal/adapters/mongodb/SCHEMA.md** - MongoDB schema documentation

### Factory & Configuration
16. **internal/adapters/factory/database_factory.go** - Adapter factory
17. **config/database.yaml** - Database configuration examples

### Documentation & Examples
18. **DATABASE_ADAPTER_GUIDE.md** - Comprehensive usage guide
19. **examples/database_adapter_example.go** - Complete working examples
20. **DATABASE_ADAPTER_README.md** - This file

## Quick Start

### 1. Install Dependencies

```bash
# PostgreSQL
go get github.com/lib/pq
go get github.com/jmoiron/sqlx

# MongoDB
go get go.mongodb.org/mongo-driver/mongo
```

### 2. Setup Database

#### PostgreSQL
```bash
# Create database
createdb eir
psql -d eir -f internal/adapters/postgres/schema.sql
psql -d eir -f internal/adapters/postgres/schema_extended.sql
```

#### MongoDB
```bash
# Run initialization script from SCHEMA.md
mongosh eir < init_script.js
```

### 3. Configure Application

Edit `config/database.yaml`:

```yaml
database:
  type: postgres  # or "mongodb"
  postgres:
    host: localhost
    port: 5432
    user: eir
    password: eir_password
    database: eir
```

### 4. Run Example

```bash
# Run demo with PostgreSQL
go run examples/database_adapter_example.go -db=postgres -action=demo

# Run demo with MongoDB
go run examples/database_adapter_example.go -db=mongodb -action=demo

# Run statistics
go run examples/database_adapter_example.go -db=postgres -action=stats

# Run cleanup
go run examples/database_adapter_example.go -db=postgres -action=cleanup

# Run migration
go run examples/database_adapter_example.go -action=migrate
```

## Usage Example

```go
package main

import (
    "context"
    "github.com/hsdfat8/eir/internal/adapters/factory"
    "github.com/hsdfat8/eir/internal/domain/ports"
)

func main() {
    ctx := context.Background()

    // Create configuration
    config := &ports.DatabaseConfig{
        Type: ports.DatabaseTypePostgreSQL,
        PostgresConfig: factory.GetDefaultPostgresConfig(),
    }

    // Create adapter
    dbFactory := factory.NewDatabaseAdapterFactory()
    adapter, _ := dbFactory.CreateAndConnectAdapter(ctx, config)
    defer adapter.Disconnect(ctx)

    // Use repositories
    imeiRepo := adapter.GetIMEIRepository()
    auditRepo := adapter.GetAuditRepository()
    historyRepo := adapter.GetHistoryRepository()
    snapshotRepo := adapter.GetSnapshotRepository()

    // Your application logic...
}
```

## Database Schema Highlights

### PostgreSQL

**New Tables:**
- `equipment_history` - Change tracking (partitioned by quarter)
- `equipment_snapshots` - Point-in-time snapshots
- `audit_log_extended` - Extended audit metadata

**New Triggers:**
- `trigger_equipment_change_history` - Auto-records changes
- `trigger_equipment_snapshot_before_update` - Auto-creates snapshots

**New Functions:**
- `record_equipment_change()` - Change tracking logic
- `create_equipment_snapshot_before_update()` - Snapshot creation
- `get_equipment_timeline(imei)` - Complete event timeline
- `cleanup_old_data(days)` - Data cleanup utility

### MongoDB

**Collections:**
- `equipment` - Equipment records with unique IMEI index
- `audit_log` - Audit logs with compound indexes
- `equipment_history` - Change history
- `equipment_snapshots` - Snapshots

**Indexes:**
- Unique index on IMEI
- Compound indexes for time-range queries
- Hash indexes for sharding
- Optional TTL indexes for auto-cleanup

## Key Interfaces

### DatabaseAdapter
```go
type DatabaseAdapter interface {
    Connect(ctx) error
    Disconnect(ctx) error
    Ping(ctx) error
    GetType() DatabaseType
    BeginTransaction(ctx) (Transaction, error)
    GetIMEIRepository() IMEIRepository
    GetAuditRepository() AuditRepository
    GetExtendedAuditRepository() ExtendedAuditRepository
    GetHistoryRepository() HistoryRepository
    GetSnapshotRepository() SnapshotRepository
    HealthCheck(ctx) error
    GetConnectionStats() ConnectionStats
    PurgeOldAudits(ctx, beforeDate) (int64, error)
    PurgeOldHistory(ctx, beforeDate) (int64, error)
    OptimizeDatabase(ctx) error
}
```

### HistoryRepository
```go
type HistoryRepository interface {
    RecordChange(ctx, history) error
    GetHistoryByIMEI(ctx, imei, offset, limit) ([]*EquipmentHistory, error)
    GetHistoryByTimeRange(ctx, start, end, offset, limit) ([]*EquipmentHistory, error)
    GetHistoryByChangeType(ctx, changeType, offset, limit) ([]*EquipmentHistory, error)
}
```

### ExtendedAuditRepository
```go
type ExtendedAuditRepository interface {
    AuditRepository
    LogCheckExtended(ctx, audit) error
    GetExtendedAuditsByIMEI(ctx, imei, offset, limit) ([]*AuditLogExtended, error)
    GetAuditsByRequestSource(ctx, source, offset, limit) ([]*AuditLog, error)
    GetAuditStatistics(ctx, start, end) (map[string]interface{}, error)
}
```

## Testing

```bash
# Run unit tests
go test ./internal/adapters/postgres/...
go test ./internal/adapters/mongodb/...

# Run integration tests
go test -tags=integration ./...

# Run example
go run examples/database_adapter_example.go
```

## Performance Considerations

### PostgreSQL
- Uses table partitioning for audit logs (quarterly)
- Optimized indexes for common queries
- Connection pooling configured
- Regular VACUUM recommended

### MongoDB
- Compound indexes for time-range queries
- Optional sharding by hashed IMEI
- Change streams for real-time updates
- TTL indexes for automatic cleanup

## Migration Between Databases

The example includes a migration utility:

```bash
# Migrate from PostgreSQL to MongoDB
go run examples/database_adapter_example.go -action=migrate
```

## Configuration Options

### PostgreSQL
- Host, Port, User, Password, Database
- SSL Mode (disable, require, verify-ca, verify-full)
- Connection Pool (max open, max idle, lifetime)
- Query Timeout

### MongoDB
- URI (connection string with replica set)
- Database name
- Pool Size (max, min)
- Timeouts (server, socket, idle)
- Read Preference (primary, secondary, etc.)
- Write Concern (majority, w1, w2, etc.)
- Change Streams (enable/disable)

## Maintenance Operations

```go
// Cleanup old data (90 days)
cutoffDate := time.Now().Add(-90 * 24 * time.Hour).Format("2006-01-02")
adapter.PurgeOldAudits(ctx, cutoffDate)
adapter.PurgeOldHistory(ctx, cutoffDate)

// Optimize database
adapter.OptimizeDatabase(ctx)

// Health check
stats := adapter.GetConnectionStats()
err := adapter.HealthCheck(ctx)
```

## Benefits

1. **Flexibility**: Switch databases without code changes
2. **Scalability**: Both databases support horizontal scaling
3. **Audit Compliance**: Complete audit trail with change history
4. **Performance**: Optimized indexes and query patterns
5. **Maintainability**: Clean architecture with clear separation
6. **Extensibility**: Easy to add new repositories or adapters
7. **Reliability**: Transaction support for atomic operations
8. **Observability**: Health checks and connection monitoring

## Next Steps

1. **Integration**: Integrate adapters into existing EIR service
2. **Testing**: Add comprehensive unit and integration tests
3. **Monitoring**: Add Prometheus metrics for database operations
4. **Documentation**: Update API documentation
5. **CI/CD**: Add database migration scripts to deployment pipeline
6. **Benchmarking**: Performance testing with realistic load
7. **Backup**: Implement automated backup strategies

## Support & Documentation

- **Comprehensive Guide**: See [DATABASE_ADAPTER_GUIDE.md](DATABASE_ADAPTER_GUIDE.md)
- **PostgreSQL Schema**: See [internal/adapters/postgres/schema_extended.sql](internal/adapters/postgres/schema_extended.sql)
- **MongoDB Schema**: See [internal/adapters/mongodb/SCHEMA.md](internal/adapters/mongodb/SCHEMA.md)
- **Example Code**: See [examples/database_adapter_example.go](examples/database_adapter_example.go)
- **Configuration**: See [config/database.yaml](config/database.yaml)

## License

Same as the EIR project.

## Authors

- Database Adapter Implementation: Claude Sonnet 4.5
- EIR Project: hsdfat8

---

**Status**: ✅ Complete and ready for integration
**Date**: 2025-12-23
