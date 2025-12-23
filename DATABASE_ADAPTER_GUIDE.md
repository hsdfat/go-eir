# Database Adapter Implementation Guide

This guide explains the database adapter architecture for the EIR (Equipment Identity Register) project, which supports both PostgreSQL and MongoDB with comprehensive audit and history tracking.

## Table of Contents

1. [Overview](#overview)
2. [Architecture](#architecture)
3. [Features](#features)
4. [Getting Started](#getting-started)
5. [Usage Examples](#usage-examples)
6. [Configuration](#configuration)
7. [Data Models](#data-models)
8. [Audit and History Tracking](#audit-and-history-tracking)
9. [Migration Guide](#migration-guide)
10. [Performance Considerations](#performance-considerations)
11. [Troubleshooting](#troubleshooting)

## Overview

The database adapter provides a unified interface for database operations, allowing seamless switching between PostgreSQL and MongoDB without changing application code. Both adapters support:

- **Full CRUD operations** for equipment management
- **Comprehensive audit logging** for all equipment checks
- **Change history tracking** for equipment modifications
- **Point-in-time snapshots** for audit and rollback
- **Extended audit capabilities** with performance metrics
- **Transaction support** for atomic operations
- **Connection pooling** and health monitoring

## Architecture

### Hexagonal Architecture (Ports and Adapters)

```
┌─────────────────────────────────────────────────────┐
│                  Domain Layer                       │
│  ┌──────────────────────────────────────────┐      │
│  │  Equipment, AuditLog, History Models     │      │
│  └──────────────────────────────────────────┘      │
│  ┌──────────────────────────────────────────┐      │
│  │  Ports (Interfaces)                      │      │
│  │  - DatabaseAdapter                       │      │
│  │  - IMEIRepository                        │      │
│  │  - AuditRepository                       │      │
│  │  - HistoryRepository                     │      │
│  │  - SnapshotRepository                    │      │
│  └──────────────────────────────────────────┘      │
└─────────────────────────────────────────────────────┘
                       ▲
                       │
         ┌─────────────┴─────────────┐
         │                           │
┌────────┴────────┐         ┌────────┴────────┐
│ PostgreSQL      │         │ MongoDB         │
│ Adapter         │         │ Adapter         │
├─────────────────┤         ├─────────────────┤
│ - IMEI Repo     │         │ - IMEI Repo     │
│ - Audit Repo    │         │ - Audit Repo    │
│ - History Repo  │         │ - History Repo  │
│ - Snapshot Repo │         │ - Snapshot Repo │
└─────────────────┘         └─────────────────┘
```

### Key Components

1. **DatabaseAdapter**: Main interface for database operations
2. **Repository Interfaces**: Define data access patterns
3. **Concrete Adapters**: PostgreSQL and MongoDB implementations
4. **Factory**: Creates and configures adapters
5. **Models**: Domain entities (Equipment, AuditLog, History, Snapshots)

## Features

### Core Features

✅ **Dual Database Support**: PostgreSQL and MongoDB with identical interfaces
✅ **IMEI Management**: Create, read, update, delete equipment records
✅ **Status Tracking**: WHITELISTED, BLACKLISTED, GREYLISTED
✅ **Audit Logging**: Complete trail of all equipment checks
✅ **Change History**: Track all modifications to equipment
✅ **Snapshots**: Point-in-time state capture
✅ **Transaction Support**: Atomic multi-operation support
✅ **Connection Pooling**: Efficient resource management
✅ **Health Monitoring**: Connection stats and health checks

### Extended Features

✅ **Extended Audit Logs**: IP addresses, user agents, processing metrics
✅ **Aggregated Statistics**: Pre-computed analytics
✅ **Automatic History**: Triggers for change tracking (PostgreSQL)
✅ **Partitioning**: Quarterly partitions for audit logs (PostgreSQL)
✅ **TTL Support**: Automatic cleanup of old data (MongoDB)
✅ **Change Streams**: Real-time notifications (MongoDB)
✅ **Sharding Ready**: Horizontal scaling support (MongoDB)

## Getting Started

### 1. Install Dependencies

```bash
# PostgreSQL driver
go get github.com/lib/pq
go get github.com/jmoiron/sqlx

# MongoDB driver
go get go.mongodb.org/mongo-driver/mongo
```

### 2. Initialize Database

#### PostgreSQL

```bash
# Connect to PostgreSQL
psql -U postgres

# Create database and user
CREATE DATABASE eir;
CREATE USER eir WITH PASSWORD 'eir_password';
GRANT ALL PRIVILEGES ON DATABASE eir TO eir;

# Run schema migrations
psql -U eir -d eir -f internal/adapters/postgres/schema.sql
psql -U eir -d eir -f internal/adapters/postgres/schema_extended.sql
```

#### MongoDB

```bash
# Connect to MongoDB
mongosh

# Run initialization script
use eir;
load('internal/adapters/mongodb/SCHEMA.md'); # Extract and run the JS code
```

### 3. Configure Application

Create `config/database.yaml`:

```yaml
database:
  type: postgres  # or "mongodb"

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

## Usage Examples

### Basic Setup

```go
package main

import (
    "context"
    "log"

    "github.com/hsdfat8/eir/internal/adapters/factory"
    "github.com/hsdfat8/eir/internal/domain/ports"
)

func main() {
    ctx := context.Background()

    // Create configuration
    config := &ports.DatabaseConfig{
        Type: ports.DatabaseTypePostgreSQL,
        PostgresConfig: &ports.PostgresConfig{
            Host:            "localhost",
            Port:            5432,
            User:            "eir",
            Password:        "eir_password",
            Database:        "eir",
            SSLMode:         "disable",
            MaxOpenConns:    25,
            MaxIdleConns:    5,
            ConnMaxLifetime: 300,
            ConnMaxIdleTime: 600,
            QueryTimeout:    30,
        },
    }

    // Create factory
    dbFactory := factory.NewDatabaseAdapterFactory()

    // Validate configuration
    if err := dbFactory.ValidateConfig(config); err != nil {
        log.Fatalf("Invalid config: %v", err)
    }

    // Create and connect adapter
    adapter, err := dbFactory.CreateAndConnectAdapter(ctx, config)
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer adapter.Disconnect(ctx)

    // Use repositories
    imeiRepo := adapter.GetIMEIRepository()
    auditRepo := adapter.GetAuditRepository()
    historyRepo := adapter.GetHistoryRepository()
    snapshotRepo := adapter.GetSnapshotRepository()

    log.Printf("Connected to %s database", adapter.GetType())
}
```

### Create Equipment

```go
func createEquipment(ctx context.Context, adapter ports.DatabaseAdapter) error {
    imeiRepo := adapter.GetIMEIRepository()

    equipment := &models.Equipment{
        IMEI:             "123456789012345",
        Status:           models.EquipmentStatusWhitelisted,
        AddedBy:          "admin",
        LastUpdated:      time.Now(),
        CheckCount:       0,
        ManufacturerTAC:  strPtr("12345678"),
        ManufacturerName: strPtr("Apple"),
    }

    err := imeiRepo.Create(ctx, equipment)
    if err != nil {
        return fmt.Errorf("failed to create equipment: %w", err)
    }

    log.Printf("Created equipment with ID: %d", equipment.ID)
    return nil
}
```

### Log Equipment Check with Audit

```go
func checkEquipment(ctx context.Context, adapter ports.DatabaseAdapter, imei string) error {
    imeiRepo := adapter.GetIMEIRepository()
    auditRepo := adapter.GetAuditRepository()

    // Get equipment
    equipment, err := imeiRepo.GetByIMEI(ctx, imei)
    if err != nil {
        return fmt.Errorf("equipment not found: %w", err)
    }

    // Log audit entry
    audit := &models.AuditLog{
        IMEI:          imei,
        Status:        equipment.Status,
        CheckTime:     time.Now(),
        RequestSource: "HTTP_5G",
        SUPI:          strPtr("imsi-123456789012345"),
    }

    err = auditRepo.LogCheck(ctx, audit)
    if err != nil {
        return fmt.Errorf("failed to log audit: %w", err)
    }

    // Increment check count
    err = imeiRepo.IncrementCheckCount(ctx, imei)
    if err != nil {
        return fmt.Errorf("failed to increment count: %w", err)
    }

    log.Printf("Equipment %s status: %s", imei, equipment.Status)
    return nil
}
```

### Extended Audit with Metrics

```go
func checkEquipmentExtended(ctx context.Context, adapter ports.DatabaseAdapter, imei string, ipAddr string) error {
    startTime := time.Now()

    extAuditRepo := adapter.GetExtendedAuditRepository()
    historyRepo := adapter.GetHistoryRepository()

    // Perform check...
    equipment, err := adapter.GetIMEIRepository().GetByIMEI(ctx, imei)
    if err != nil {
        return err
    }

    processingTime := time.Since(startTime).Milliseconds()

    // Log extended audit with metrics
    extAudit := &models.AuditLogExtended{
        AuditLog: models.AuditLog{
            IMEI:          imei,
            Status:        equipment.Status,
            CheckTime:     time.Now(),
            RequestSource: "HTTP_5G",
        },
        IPAddress:        &ipAddr,
        ProcessingTimeMs: &processingTime,
        AdditionalData: map[string]interface{}{
            "client_version": "1.0.0",
            "region":        "US-WEST",
        },
        ChangeHistory: &models.EquipmentHistory{
            IMEI:        imei,
            ChangeType:  models.ChangeTypeCheck,
            ChangedAt:   time.Now(),
            ChangedBy:   "system",
            NewStatus:   equipment.Status,
        },
    }

    err = extAuditRepo.LogCheckExtended(ctx, extAudit)
    if err != nil {
        return fmt.Errorf("failed to log extended audit: %w", err)
    }

    return nil
}
```

### Create Snapshot

```go
func createEquipmentSnapshot(ctx context.Context, adapter ports.DatabaseAdapter, imei string) error {
    imeiRepo := adapter.GetIMEIRepository()
    snapshotRepo := adapter.GetSnapshotRepository()

    // Get current equipment state
    equipment, err := imeiRepo.GetByIMEI(ctx, imei)
    if err != nil {
        return err
    }

    // Create snapshot
    snapshot := &models.EquipmentSnapshot{
        EquipmentID:  equipment.ID,
        IMEI:         equipment.IMEI,
        SnapshotTime: time.Now(),
        Status:       equipment.Status,
        Reason:       equipment.Reason,
        CheckCount:   equipment.CheckCount,
        Metadata:     equipment.Metadata,
        CreatedBy:    "admin",
        SnapshotType: "MANUAL",
    }

    err = snapshotRepo.CreateSnapshot(ctx, snapshot)
    if err != nil {
        return fmt.Errorf("failed to create snapshot: %w", err)
    }

    log.Printf("Created snapshot ID: %d", snapshot.ID)
    return nil
}
```

### Transaction Example

```go
func updateEquipmentWithAudit(ctx context.Context, adapter ports.DatabaseAdapter, imei string, newStatus models.EquipmentStatus) error {
    // Begin transaction
    tx, err := adapter.BeginTransaction(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx) // Rollback if not committed

    imeiRepo := tx.GetIMEIRepository()
    auditRepo := tx.GetAuditRepository()

    // Get current equipment
    equipment, err := imeiRepo.GetByIMEI(ctx, imei)
    if err != nil {
        return err
    }

    oldStatus := equipment.Status
    equipment.Status = newStatus
    equipment.LastUpdated = time.Now()

    // Update equipment
    err = imeiRepo.Update(ctx, equipment)
    if err != nil {
        return err
    }

    // Log audit
    audit := &models.AuditLog{
        IMEI:          imei,
        Status:        newStatus,
        CheckTime:     time.Now(),
        RequestSource: "ADMIN_UPDATE",
    }

    err = auditRepo.LogCheck(ctx, audit)
    if err != nil {
        return err
    }

    // Commit transaction
    err = tx.Commit(ctx)
    if err != nil {
        return err
    }

    log.Printf("Updated equipment %s: %s -> %s", imei, oldStatus, newStatus)
    return nil
}
```

### Get Audit Statistics

```go
func getAuditStats(ctx context.Context, adapter ports.DatabaseAdapter) error {
    extAuditRepo := adapter.GetExtendedAuditRepository()

    startTime := time.Now().Add(-24 * time.Hour)
    endTime := time.Now()

    stats, err := extAuditRepo.GetAuditStatistics(ctx, startTime, endTime)
    if err != nil {
        return err
    }

    log.Printf("Audit Statistics (last 24 hours):")
    log.Printf("  Total Checks: %v", stats["total_checks"])
    log.Printf("  Unique IMEIs: %v", stats["unique_imeis"])
    log.Printf("  Whitelisted: %v", stats["whitelisted_count"])
    log.Printf("  Blacklisted: %v", stats["blacklisted_count"])
    log.Printf("  Greylisted: %v", stats["greylisted_count"])
    log.Printf("  Avg Processing Time: %.2f ms", stats["avg_processing_time_ms"])

    return nil
}
```

### Cleanup Old Data

```go
func cleanupOldData(ctx context.Context, adapter ports.DatabaseAdapter) error {
    // Delete audits older than 90 days
    cutoffDate := time.Now().Add(-90 * 24 * time.Hour).Format("2006-01-02")

    auditCount, err := adapter.PurgeOldAudits(ctx, cutoffDate)
    if err != nil {
        return err
    }

    historyCount, err := adapter.PurgeOldHistory(ctx, cutoffDate)
    if err != nil {
        return err
    }

    log.Printf("Cleaned up %d old audits and %d old history records", auditCount, historyCount)

    // Optimize database
    err = adapter.OptimizeDatabase(ctx)
    if err != nil {
        return err
    }

    return nil
}
```

## Configuration

### Environment Variables

```bash
# Database type
export EIR_DB_TYPE=postgres  # or mongodb

# PostgreSQL
export EIR_POSTGRES_HOST=localhost
export EIR_POSTGRES_PORT=5432
export EIR_POSTGRES_USER=eir
export EIR_POSTGRES_PASSWORD=secret
export EIR_POSTGRES_DATABASE=eir
export EIR_POSTGRES_SSL_MODE=require

# MongoDB
export EIR_MONGODB_URI=mongodb://user:pass@localhost:27017
export EIR_MONGODB_DATABASE=eir
export EIR_MONGODB_MAX_POOL_SIZE=100
```

### YAML Configuration

See [`config/database.yaml`](config/database.yaml) for complete examples.

## Data Models

### Equipment

- **IMEI**: 14-16 digit identifier
- **Status**: WHITELISTED, BLACKLISTED, GREYLISTED
- **Metadata**: JSONB for extensibility
- **Timestamps**: Creation, last update, last check
- **Counters**: Check count

### AuditLog

- **Check details**: IMEI, status, timestamp
- **Source tracking**: Diameter S13, HTTP 5G
- **Subscriber info**: SUPI, GPSI, username
- **Session correlation**: Session ID, result code

### EquipmentHistory

- **Change tracking**: CREATE, UPDATE, DELETE, CHECK
- **State transitions**: Previous/new status and reason
- **Audit trail**: Who, when, why
- **Details**: Additional change metadata

### EquipmentSnapshot

- **Point-in-time state**: Full equipment state
- **Snapshot types**: MANUAL, SCHEDULED, PRE_UPDATE
- **Rollback support**: Restore previous states

## Audit and History Tracking

### Automatic History (PostgreSQL)

PostgreSQL uses database triggers to automatically record changes:

```sql
-- Automatically creates history entry on equipment change
CREATE TRIGGER trigger_equipment_change_history
    AFTER INSERT OR UPDATE OR DELETE ON equipment
    FOR EACH ROW
    EXECUTE FUNCTION record_equipment_change();
```

### Manual History (MongoDB)

MongoDB requires application-level history tracking:

```go
// Record change manually
history := &models.EquipmentHistory{
    IMEI:           equipment.IMEI,
    ChangeType:     models.ChangeTypeUpdate,
    ChangedAt:      time.Now(),
    ChangedBy:      "admin",
    PreviousStatus: &oldStatus,
    NewStatus:      equipment.Status,
}
historyRepo.RecordChange(ctx, history)
```

## Migration Guide

### PostgreSQL to MongoDB

```go
func migrateToMongoDB(ctx context.Context) error {
    // Connect to both databases
    pgConfig := factory.CreateDefaultConfig(ports.DatabaseTypePostgreSQL)
    mongoConfig := factory.CreateDefaultConfig(ports.DatabaseTypeMongoDB)

    dbFactory := factory.NewDatabaseAdapterFactory()

    pgAdapter, _ := dbFactory.CreateAndConnectAdapter(ctx, pgConfig)
    defer pgAdapter.Disconnect(ctx)

    mongoAdapter, _ := dbFactory.CreateAndConnectAdapter(ctx, mongoConfig)
    defer mongoAdapter.Disconnect(ctx)

    // Migrate equipment
    offset := 0
    limit := 1000

    for {
        equipments, err := pgAdapter.GetIMEIRepository().List(ctx, offset, limit)
        if err != nil || len(equipments) == 0 {
            break
        }

        for _, equipment := range equipments {
            mongoAdapter.GetIMEIRepository().Create(ctx, equipment)
        }

        offset += limit
        log.Printf("Migrated %d equipment records", offset)
    }

    return nil
}
```

## Performance Considerations

### PostgreSQL

- **Partitioning**: Audit logs partitioned quarterly
- **Indexes**: Optimized for IMEI, status, time-based queries
- **Connection pooling**: Configurable pool size
- **VACUUM**: Regular maintenance recommended

### MongoDB

- **Sharding**: Hash-based sharding on IMEI
- **Indexes**: Compound indexes for common queries
- **Change streams**: Optional real-time notifications
- **TTL indexes**: Automatic cleanup of old data

### Best Practices

1. **Use connection pooling**: Configure appropriate pool sizes
2. **Batch operations**: Use transactions for multiple operations
3. **Index optimization**: Monitor query performance
4. **Regular cleanup**: Purge old audit logs
5. **Monitor metrics**: Track connection stats and query times

## Troubleshooting

### Connection Issues

```go
// Check database health
err := adapter.HealthCheck(ctx)
if err != nil {
    log.Printf("Health check failed: %v", err)
}

// Get connection stats
stats := adapter.GetConnectionStats()
log.Printf("Open connections: %d/%d", stats.OpenConnections, stats.MaxConnections)
log.Printf("Healthy: %v", stats.Healthy)
```

### Performance Issues

```bash
# PostgreSQL: Check slow queries
SELECT * FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;

# MongoDB: Profile slow queries
db.setProfilingLevel(1, { slowms: 100 })
db.system.profile.find().limit(10).sort({ ts: -1 })
```

### Data Inconsistency

```go
// Create snapshot before risky operations
snapshotRepo.CreateSnapshot(ctx, snapshot)

// Use transactions for atomic operations
tx, _ := adapter.BeginTransaction(ctx)
defer tx.Rollback(ctx)
// ... perform operations ...
tx.Commit(ctx)
```

## Support

For issues or questions:
- Check logs for detailed error messages
- Review configuration settings
- Consult database-specific documentation
- Test with default configurations first

Helper function for string pointers:
```go
func strPtr(s string) *string {
    return &s
}
```
