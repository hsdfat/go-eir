# MongoDB Schema for EIR Project

This document describes the MongoDB collections and indexes for the Equipment Identity Register (EIR) system.

## Collections

### 1. equipment

Stores equipment (mobile devices) information.

```javascript
{
  _id: ObjectId,
  id: Long,                          // Sequential ID for compatibility
  imei: String,                      // UNIQUE - International Mobile Equipment Identity
  imeisv: String,                    // Optional IMEI Software Version
  status: String,                    // "WHITELISTED", "BLACKLISTED", "GREYLISTED"
  reason: String,                    // Optional reason for status
  last_updated: ISODate,             // Last modification timestamp
  last_check_time: ISODate,          // Last equipment check timestamp
  check_count: Long,                 // Number of times checked
  added_by: String,                  // User/admin who added the equipment
  metadata: String,                  // JSON string for extensibility
  manufacturer_tac: String,          // Type Allocation Code (first 8 digits)
  manufacturer_name: String          // Device manufacturer name
}
```

**Indexes:**
- `{ imei: 1 }` - UNIQUE
- `{ status: 1 }`
- `{ manufacturer_tac: 1 }`
- `{ last_check_time: -1 }`
- `{ check_count: -1 }`
- `{ last_updated: -1 }`

**Validators:**
```javascript
db.createCollection("equipment", {
  validator: {
    $jsonSchema: {
      required: ["imei", "status", "added_by"],
      properties: {
        imei: {
          bsonType: "string",
          pattern: "^[0-9]{14,16}$"
        },
        status: {
          enum: ["WHITELISTED", "BLACKLISTED", "GREYLISTED"]
        }
      }
    }
  }
})
```

### 2. audit_log

Stores audit trail of all equipment checks.

```javascript
{
  _id: ObjectId,
  id: Long,
  imei: String,                      // IMEI being checked
  imeisv: String,                    // Optional IMEISV
  status: String,                    // Check result status
  check_time: ISODate,               // Timestamp of check
  origin_host: String,               // Diameter origin host (4G)
  origin_realm: String,              // Diameter origin realm (4G)
  user_name: String,                 // Subscriber username
  supi: String,                      // 5G Subscription Permanent Identifier
  gpsi: String,                      // 5G Generic Public Subscription Identifier
  request_source: String,            // "DIAMETER_S13", "HTTP_5G", etc.
  session_id: String,                // Session correlation ID
  result_code: Int,                  // Diameter result code

  // Extended fields (for AuditLogExtended)
  ip_address: String,                // Client IP address
  user_agent: String,                // Client user agent
  additional_data: Object,           // Additional metadata
  processing_time_ms: Long           // Request processing time in milliseconds
}
```

**Indexes:**
- `{ imei: 1 }`
- `{ check_time: -1 }`
- `{ status: 1 }`
- `{ request_source: 1 }`
- `{ supi: 1 }`
- `{ imei: 1, check_time: -1 }` - Compound index

**TTL Index (Optional):**
```javascript
db.audit_log.createIndex(
  { check_time: 1 },
  { expireAfterSeconds: 7776000 }  // 90 days
)
```

### 3. equipment_history

Tracks all changes to equipment records.

```javascript
{
  _id: ObjectId,
  id: Long,
  imei: String,                      // Equipment IMEI
  change_type: String,               // "CREATE", "UPDATE", "DELETE", "CHECK"
  changed_at: ISODate,               // Timestamp of change
  changed_by: String,                // User who made the change
  previous_status: String,           // Status before change
  new_status: String,                // Status after change
  previous_reason: String,           // Previous reason
  new_reason: String,                // New reason
  change_details: Object,            // Additional change details
  session_id: String                 // Session correlation ID
}
```

**Indexes:**
- `{ imei: 1 }`
- `{ changed_at: -1 }`
- `{ change_type: 1 }`
- `{ changed_by: 1 }`

**Validators:**
```javascript
db.createCollection("equipment_history", {
  validator: {
    $jsonSchema: {
      required: ["imei", "change_type", "changed_at", "changed_by", "new_status"],
      properties: {
        change_type: {
          enum: ["CREATE", "UPDATE", "DELETE", "CHECK"]
        }
      }
    }
  }
})
```

### 4. equipment_snapshots

Stores point-in-time snapshots of equipment state.

```javascript
{
  _id: ObjectId,
  id: Long,
  equipment_id: Long,                // Reference to equipment.id
  imei: String,                      // Equipment IMEI
  snapshot_time: ISODate,            // Snapshot timestamp
  status: String,                    // Equipment status at snapshot time
  reason: String,                    // Reason at snapshot time
  check_count: Long,                 // Check count at snapshot time
  metadata: String,                  // Metadata at snapshot time
  created_by: String,                // Who created the snapshot
  snapshot_type: String              // "MANUAL", "SCHEDULED", "PRE_UPDATE"
}
```

**Indexes:**
- `{ imei: 1 }`
- `{ snapshot_time: -1 }`
- `{ equipment_id: 1 }`
- `{ snapshot_type: 1 }`

**Validators:**
```javascript
db.createCollection("equipment_snapshots", {
  validator: {
    $jsonSchema: {
      required: ["equipment_id", "imei", "snapshot_time", "status", "created_by", "snapshot_type"],
      properties: {
        snapshot_type: {
          enum: ["MANUAL", "SCHEDULED", "PRE_UPDATE"]
        }
      }
    }
  }
})
```

## Initialization Script

```javascript
// MongoDB initialization script
// Run this to set up the EIR database

use eir;

// Create equipment collection with validators
db.createCollection("equipment", {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["imei", "status", "added_by"],
      properties: {
        imei: {
          bsonType: "string",
          pattern: "^[0-9]{14,16}$",
          description: "IMEI must be 14-16 digits"
        },
        status: {
          enum: ["WHITELISTED", "BLACKLISTED", "GREYLISTED"],
          description: "Status must be one of the enum values"
        },
        added_by: {
          bsonType: "string",
          minLength: 1,
          description: "Added by must be a non-empty string"
        }
      }
    }
  }
});

// Create indexes for equipment
db.equipment.createIndex({ imei: 1 }, { unique: true });
db.equipment.createIndex({ status: 1 });
db.equipment.createIndex({ manufacturer_tac: 1 });
db.equipment.createIndex({ last_check_time: -1 });
db.equipment.createIndex({ check_count: -1 });
db.equipment.createIndex({ last_updated: -1 });

// Create audit_log collection
db.createCollection("audit_log");

// Create indexes for audit_log
db.audit_log.createIndex({ imei: 1 });
db.audit_log.createIndex({ check_time: -1 });
db.audit_log.createIndex({ status: 1 });
db.audit_log.createIndex({ request_source: 1 });
db.audit_log.createIndex({ supi: 1 });
db.audit_log.createIndex({ imei: 1, check_time: -1 });

// Optional: Create TTL index to automatically delete old audit logs after 90 days
// db.audit_log.createIndex({ check_time: 1 }, { expireAfterSeconds: 7776000 });

// Create equipment_history collection with validators
db.createCollection("equipment_history", {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["imei", "change_type", "changed_at", "changed_by", "new_status"],
      properties: {
        change_type: {
          enum: ["CREATE", "UPDATE", "DELETE", "CHECK"],
          description: "Change type must be one of the enum values"
        }
      }
    }
  }
});

// Create indexes for equipment_history
db.equipment_history.createIndex({ imei: 1 });
db.equipment_history.createIndex({ changed_at: -1 });
db.equipment_history.createIndex({ change_type: 1 });
db.equipment_history.createIndex({ changed_by: 1 });

// Create equipment_snapshots collection with validators
db.createCollection("equipment_snapshots", {
  validator: {
    $jsonSchema: {
      bsonType: "object",
      required: ["equipment_id", "imei", "snapshot_time", "status", "created_by", "snapshot_type"],
      properties: {
        snapshot_type: {
          enum: ["MANUAL", "SCHEDULED", "PRE_UPDATE"],
          description: "Snapshot type must be one of the enum values"
        }
      }
    }
  }
});

// Create indexes for equipment_snapshots
db.equipment_snapshots.createIndex({ imei: 1 });
db.equipment_snapshots.createIndex({ snapshot_time: -1 });
db.equipment_snapshots.createIndex({ equipment_id: 1 });
db.equipment_snapshots.createIndex({ snapshot_type: 1 });

print("EIR MongoDB schema initialized successfully!");
```

## Change Streams (Optional)

For real-time change notifications, you can use MongoDB Change Streams:

```javascript
// Watch for equipment changes
const changeStream = db.equipment.watch([
  {
    $match: {
      'operationType': { $in: ['insert', 'update', 'delete'] }
    }
  }
]);

changeStream.on('change', (change) => {
  console.log('Equipment changed:', change);
});
```

## Aggregation Pipelines

### Get Equipment Statistics

```javascript
db.equipment.aggregate([
  {
    $group: {
      _id: "$status",
      count: { $sum: 1 },
      avg_check_count: { $avg: "$check_count" },
      max_check_count: { $max: "$check_count" }
    }
  }
])
```

### Get Audit Statistics by Day

```javascript
db.audit_log.aggregate([
  {
    $match: {
      check_time: {
        $gte: ISODate("2025-01-01"),
        $lte: ISODate("2025-12-31")
      }
    }
  },
  {
    $group: {
      _id: {
        date: { $dateToString: { format: "%Y-%m-%d", date: "$check_time" } },
        status: "$status",
        request_source: "$request_source"
      },
      count: { $sum: 1 },
      unique_imeis: { $addToSet: "$imei" }
    }
  },
  {
    $project: {
      _id: 0,
      date: "$_id.date",
      status: "$_id.status",
      request_source: "$_id.request_source",
      count: 1,
      unique_imeis: { $size: "$unique_imeis" }
    }
  },
  {
    $sort: { date: -1 }
  }
])
```

## Sharding Recommendations

For high-volume deployments, consider sharding:

```javascript
// Enable sharding on the database
sh.enableSharding("eir")

// Shard audit_log by hashed IMEI for even distribution
sh.shardCollection("eir.audit_log", { imei: "hashed" })

// Shard equipment_history by hashed IMEI
sh.shardCollection("eir.equipment_history", { imei: "hashed" })
```

## Backup and Maintenance

```bash
# Backup database
mongodump --db=eir --out=/backup/eir-$(date +%Y%m%d)

# Restore database
mongorestore --db=eir /backup/eir-20251223/eir

# Compact collections (run periodically)
db.runCommand({ compact: 'equipment' })
db.runCommand({ compact: 'audit_log' })
db.runCommand({ compact: 'equipment_history' })
db.runCommand({ compact: 'equipment_snapshots' })
```
