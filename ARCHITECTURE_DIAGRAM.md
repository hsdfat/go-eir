# Database Adapter Architecture Diagram

## System Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          EIR Application Layer                          │
│                                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                 │
│  │ HTTP Handler │  │ Diameter S13 │  │ Admin API    │                 │
│  │  (5G N5g)    │  │   Handler    │  │  Handler     │                 │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘                 │
│         │                 │                  │                         │
│         └─────────────────┴──────────────────┘                         │
│                           │                                            │
│                  ┌────────▼────────┐                                   │
│                  │  EIR Service    │                                   │
│                  │   (Business     │                                   │
│                  │    Logic)       │                                   │
│                  └────────┬────────┘                                   │
└───────────────────────────┼────────────────────────────────────────────┘
                            │
                            │ Uses DatabaseAdapter Interface
                            │
┌───────────────────────────▼────────────────────────────────────────────┐
│                          Domain Layer (Ports)                          │
│                                                                        │
│  ┌──────────────────────────────────────────────────────────────┐    │
│  │                  DatabaseAdapter Interface                    │    │
│  │  + Connect() / Disconnect() / Ping()                         │    │
│  │  + BeginTransaction() → Transaction                          │    │
│  │  + GetIMEIRepository() → IMEIRepository                      │    │
│  │  + GetAuditRepository() → AuditRepository                    │    │
│  │  + GetExtendedAuditRepository() → ExtendedAuditRepository    │    │
│  │  + GetHistoryRepository() → HistoryRepository                │    │
│  │  + GetSnapshotRepository() → SnapshotRepository              │    │
│  │  + HealthCheck() / GetConnectionStats()                      │    │
│  │  + PurgeOldAudits() / PurgeOldHistory()                      │    │
│  └──────────────────────────────────────────────────────────────┘    │
│                                                                        │
│  ┌─────────────────┐  ┌──────────────────┐  ┌─────────────────┐     │
│  │ IMEIRepository  │  │ AuditRepository  │  │ HistoryRepo     │     │
│  ├─────────────────┤  ├──────────────────┤  ├─────────────────┤     │
│  │ GetByIMEI()     │  │ LogCheck()       │  │ RecordChange()  │     │
│  │ Create()        │  │ GetAuditsByIMEI()│  │ GetHistoryByIMEI│     │
│  │ Update()        │  │ GetByTimeRange() │  │ GetByTimeRange()│     │
│  │ Delete()        │  └──────────────────┘  │ GetByChangeType │     │
│  │ List()          │                        └─────────────────┘     │
│  │ ListByStatus()  │  ┌──────────────────┐  ┌─────────────────┐     │
│  │ IncrementCheck()│  │ExtendedAuditRepo │  │ SnapshotRepo    │     │
│  └─────────────────┘  ├──────────────────┤  ├─────────────────┤     │
│                       │LogCheckExtended()│  │CreateSnapshot() │     │
│  ┌─────────────────┐  │GetExtendedAudits│  │GetSnapshotsByID │     │
│  │ Domain Models   │  │GetByReqSource() │  │GetByIMEI()      │     │
│  ├─────────────────┤  │GetStatistics()  │  │DeleteOld()      │     │
│  │ Equipment       │  └──────────────────┘  └─────────────────┘     │
│  │ AuditLog        │                                                 │
│  │ AuditLogExt     │  ┌──────────────────┐                          │
│  │ History         │  │  Transaction     │                          │
│  │ Snapshot        │  ├──────────────────┤                          │
│  └─────────────────┘  │ Commit()         │                          │
│                       │ Rollback()       │                          │
│                       │ GetRepos()       │                          │
│                       └──────────────────┘                          │
└────────────────────────────────────────────────────────────────────┘
                            │
                            │ Implemented by
                            │
            ┌───────────────┴───────────────┐
            │                               │
┌───────────▼────────────┐      ┌───────────▼────────────┐
│   PostgreSQL Adapter   │      │    MongoDB Adapter     │
├────────────────────────┤      ├────────────────────────┤
│                        │      │                        │
│ ┌────────────────────┐ │      │ ┌────────────────────┐ │
│ │ postgres_adapter.go│ │      │ │mongodb_adapter.go  │ │
│ └────────────────────┘ │      │ └────────────────────┘ │
│                        │      │                        │
│ Repositories:          │      │ Repositories:          │
│ ┌──────────────────┐   │      │ ┌──────────────────┐   │
│ │ IMEI Repository  │   │      │ │ IMEI Repository  │   │
│ │ - GetByIMEI      │   │      │ │ - GetByIMEI      │   │
│ │ - Create/Update  │   │      │ │ - Create/Update  │   │
│ │ - List/Delete    │   │      │ │ - List/Delete    │   │
│ └──────────────────┘   │      │ └──────────────────┘   │
│                        │      │                        │
│ ┌──────────────────┐   │      │ ┌──────────────────┐   │
│ │ Audit Repository │   │      │ │ Audit Repository │   │
│ │ - LogCheck       │   │      │ │ - LogCheck       │   │
│ │ - GetAudits      │   │      │ │ - GetAudits      │   │
│ └──────────────────┘   │      │ └──────────────────┘   │
│                        │      │                        │
│ ┌──────────────────┐   │      │ ┌──────────────────┐   │
│ │Extended Audit    │   │      │ │Extended Audit    │   │
│ │ - LogExtended    │   │      │ │ - LogExtended    │   │
│ │ - GetStatistics  │   │      │ │ - GetStatistics  │   │
│ └──────────────────┘   │      │ └──────────────────┘   │
│                        │      │                        │
│ ┌──────────────────┐   │      │ ┌──────────────────┐   │
│ │History Repository│   │      │ │History Repository│   │
│ │ - RecordChange   │   │      │ │ - RecordChange   │   │
│ │ - GetHistory     │   │      │ │ - GetHistory     │   │
│ └──────────────────┘   │      │ └──────────────────┘   │
│                        │      │                        │
│ ┌──────────────────┐   │      │ ┌──────────────────┐   │
│ │Snapshot Repo     │   │      │ │Snapshot Repo     │   │
│ │ - CreateSnapshot │   │      │ │ - CreateSnapshot │   │
│ │ - GetSnapshots   │   │      │ │ - GetSnapshots   │   │
│ └──────────────────┘   │      │ └──────────────────┘   │
└────────┬───────────────┘      └────────┬───────────────┘
         │                               │
         │                               │
┌────────▼───────────────┐      ┌────────▼───────────────┐
│   PostgreSQL Database  │      │   MongoDB Database     │
├────────────────────────┤      ├────────────────────────┤
│                        │      │                        │
│ Tables:                │      │ Collections:           │
│ ┌──────────────────┐   │      │ ┌──────────────────┐   │
│ │ equipment        │   │      │ │ equipment        │   │
│ │ - BTREE indexes  │   │      │ │ - unique index   │   │
│ │ - unique IMEI    │   │      │ │ - compound index │   │
│ │ - GIN metadata   │   │      │ │ - validators     │   │
│ └──────────────────┘   │      │ └──────────────────┘   │
│                        │      │                        │
│ ┌──────────────────┐   │      │ ┌──────────────────┐   │
│ │ audit_log        │   │      │ │ audit_log        │   │
│ │ - Partitioned    │   │      │ │ - TTL index      │   │
│ │   (quarterly)    │   │      │ │ - compound index │   │
│ │ - Time indexes   │   │      │ │ - aggregation    │   │
│ └──────────────────┘   │      │ └──────────────────┘   │
│                        │      │                        │
│ ┌──────────────────┐   │      │ ┌──────────────────┐   │
│ │audit_log_extended│   │      │ │ embedded in      │   │
│ │ - JOIN with      │   │      │ │ audit_log doc    │   │
│ │   audit_log      │   │      │ │                  │   │
│ └──────────────────┘   │      │ └──────────────────┘   │
│                        │      │                        │
│ ┌──────────────────┐   │      │ ┌──────────────────┐   │
│ │equipment_history │   │      │ │equipment_history │   │
│ │ - Partitioned    │   │      │ │ - time index     │   │
│ │ - Triggers       │   │      │ │ - change type idx│   │
│ └──────────────────┘   │      │ └──────────────────┘   │
│                        │      │                        │
│ ┌──────────────────┐   │      │ ┌──────────────────┐   │
│ │equipment_snapshot│   │      │ │equipment_snapshot│   │
│ │ - Time index     │   │      │ │ - time index     │   │
│ │ - Trigger create │   │      │ │ - type index     │   │
│ └──────────────────┘   │      │ └──────────────────┘   │
│                        │      │                        │
│ Stored Procedures:     │      │ Features:              │
│ - increment_check_cnt  │      │ - Change streams       │
│ - record_change        │      │ - Aggregation          │
│ - create_snapshot      │      │ - Sharding ready       │
│ - cleanup_old_data     │      │ - Transactions         │
│                        │      │                        │
│ Triggers:              │      │                        │
│ - auto_history         │      │                        │
│ - auto_snapshot        │      │                        │
│ - update_timestamp     │      │                        │
└────────────────────────┘      └────────────────────────┘
```

## Factory Pattern

```
┌──────────────────────────────────────────────────┐
│          DatabaseAdapterFactory                  │
├──────────────────────────────────────────────────┤
│ + CreateAdapter(config) → DatabaseAdapter        │
│ + CreateAndConnectAdapter(ctx, config)           │
│ + ValidateConfig(config) → error                 │
├──────────────────────────────────────────────────┤
│                                                  │
│   switch config.Type:                            │
│   ┌────────────────────┐  ┌─────────────────┐   │
│   │ DatabaseTypePostgre│  │ DatabaseTypeMongo│  │
│   │        SQL         │  │       DB        │   │
│   └────────┬───────────┘  └────────┬────────┘   │
│            │                       │            │
│            ▼                       ▼            │
│   ┌────────────────┐      ┌────────────────┐   │
│   │ PostgresAdapter│      │ MongoDBAdapter │   │
│   └────────────────┘      └────────────────┘   │
│                                                  │
│   Default Configs:                               │
│   - GetDefaultPostgresConfig()                   │
│   - GetDefaultMongoDBConfig()                    │
│   - CreateDefaultConfig(dbType)                  │
└──────────────────────────────────────────────────┘
```

## Data Flow - Equipment Check

```
┌─────────────┐
│ HTTP/Diameter│
│   Request   │
└──────┬──────┘
       │
       ▼
┌──────────────┐
│ EIR Service  │
└──────┬───────┘
       │
       │ 1. Get equipment by IMEI
       ▼
┌──────────────────────┐
│ DatabaseAdapter      │
│  .GetIMEIRepository()│
└──────┬───────────────┘
       │
       ▼
┌──────────────────┐         ┌─────────────────┐
│ IMEI Repository  │────────▶│ Equipment Table │
│  .GetByIMEI()    │◀────────│  or Collection  │
└──────┬───────────┘         └─────────────────┘
       │
       │ Returns Equipment(status, ...)
       ▼
┌──────────────┐
│ EIR Service  │
└──────┬───────┘
       │
       │ 2. Log audit entry
       ▼
┌─────────────────────────────┐
│ DatabaseAdapter             │
│  .GetExtendedAuditRepo()    │
└──────┬──────────────────────┘
       │
       ▼
┌──────────────────────┐      ┌──────────────────┐
│Extended Audit Repo   │─────▶│ audit_log        │
│ .LogCheckExtended()  │      │ + extended data  │
└──────┬───────────────┘      └──────────────────┘
       │
       │ Also records history
       ▼
┌──────────────────┐          ┌──────────────────┐
│ History Repo     │─────────▶│ equipment_history│
│ .RecordChange()  │          │  (automatic/     │
└──────────────────┘          │   manual)        │
                              └──────────────────┘
       │
       │ 3. Increment check count
       ▼
┌──────────────────┐          ┌──────────────────┐
│ IMEI Repository  │─────────▶│ UPDATE equipment │
│ .IncrementCheck()│          │ SET check_count++│
└──────────────────┘          └──────────────────┘
       │
       ▼
┌──────────────┐
│ Return Status│
│  to Client   │
└──────────────┘
```

## Transaction Flow

```
┌──────────────────┐
│ Application      │
└────────┬─────────┘
         │
         │ adapter.BeginTransaction(ctx)
         ▼
┌─────────────────────────────┐
│ DatabaseAdapter             │
│  .BeginTransaction()        │
└────────┬────────────────────┘
         │
         ├─ PostgreSQL: BEGIN; CREATE TRANSACTION
         └─ MongoDB: START SESSION; START TRANSACTION
         │
         ▼
┌─────────────────────────────┐
│ Transaction Object          │
│  - tx.GetIMEIRepository()   │
│  - tx.GetAuditRepository()  │
└────────┬────────────────────┘
         │
         │ Perform operations
         │ 1. Update equipment
         │ 2. Log audit
         │ 3. Record history
         ▼
┌─────────────────────────────┐
│ All operations succeed?     │
└────────┬────────────────────┘
         │
    ┌────┴────┐
    │         │
   YES       NO
    │         │
    ▼         ▼
┌────────┐ ┌─────────┐
│ COMMIT │ │ROLLBACK │
└────────┘ └─────────┘
    │         │
    │         └─ All changes reverted
    │
    └─ Changes persisted
       to database
```

## Configuration Flow

```
┌────────────────────┐
│ config/database.yaml│
│   or ENV vars      │
└─────────┬──────────┘
          │
          ▼
┌──────────────────────────┐
│ DatabaseConfig           │
│  - Type: postgres/mongodb│
│  - PostgresConfig {...}  │
│  - MongoDBConfig {...}   │
└─────────┬────────────────┘
          │
          ▼
┌──────────────────────────┐
│ DatabaseAdapterFactory   │
│  .ValidateConfig()       │
│  .CreateAdapter()        │
└─────────┬────────────────┘
          │
          ├─ If PostgreSQL
          │  ├─ Create PostgresAdapter
          │  ├─ Set connection params
          │  ├─ Configure pool
          │  └─ Initialize repos
          │
          └─ If MongoDB
             ├─ Create MongoDBAdapter
             ├─ Set MongoDB URI
             ├─ Configure pool
             └─ Initialize repos
          │
          ▼
┌──────────────────────────┐
│ Connected DatabaseAdapter│
│  Ready for use           │
└──────────────────────────┘
```

## Key Design Decisions

### 1. Hexagonal Architecture
- **Why**: Clean separation between business logic and infrastructure
- **Benefit**: Easy to swap implementations, test, and maintain

### 2. Repository Pattern
- **Why**: Encapsulate data access logic
- **Benefit**: Consistent interface, easier testing

### 3. Factory Pattern
- **Why**: Centralize adapter creation
- **Benefit**: Single point of configuration, easier to add new adapters

### 4. Interface-Based Design
- **Why**: Depend on abstractions, not concrete implementations
- **Benefit**: Flexibility, testability, maintainability

### 5. Automatic History (PostgreSQL)
- **Why**: Ensure complete audit trail
- **Benefit**: No manual tracking needed, guaranteed consistency

### 6. Partitioning (PostgreSQL)
- **Why**: Manage large audit datasets
- **Benefit**: Better query performance, easier data archival

### 7. Document Embedding (MongoDB)
- **Why**: Reduce joins, improve read performance
- **Benefit**: Faster queries, simpler data model

### 8. Flexible Metadata
- **Why**: Support future extensions without schema changes
- **Benefit**: JSONB (PostgreSQL) and documents (MongoDB) allow evolution

## Comparison Matrix

| Feature | PostgreSQL | MongoDB |
|---------|-----------|---------|
| **History Tracking** | Automatic (triggers) | Manual (app-level) |
| **Partitioning** | Built-in (quarterly) | Sharding |
| **Transactions** | Full ACID | Multi-document (4.0+) |
| **Indexes** | BTREE, GIN (JSONB) | Compound, Hash, TTL |
| **Scalability** | Vertical + replication | Horizontal sharding |
| **Flexibility** | Schema + JSONB | Schemaless documents |
| **Queries** | SQL + procedures | Aggregation pipeline |
| **Cleanup** | Manual VACUUM | TTL indexes |
| **Real-time** | LISTEN/NOTIFY | Change streams |
| **Best For** | Structured, ACID | Flexible, scalable |

---

This architecture provides maximum flexibility while maintaining clean separation of concerns and ensuring both databases have identical capabilities from the application's perspective.
