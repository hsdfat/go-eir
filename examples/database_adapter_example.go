package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/hsdfat8/eir/internal/adapters/factory"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
)

var log = logger.New("database-adapter-example", "info")

func main() {

	// Parse command-line flags
	dbType := flag.String("db", "postgres", "Database type: postgres or mongodb")
	action := flag.String("action", "demo", "Action to perform: demo, migrate, cleanup, stats")
	flag.Parse()

	ctx := context.Background()

	// Create database configuration
	var config *ports.DatabaseConfig
	switch *dbType {
	case "postgres":
		config = createPostgresConfig()
	case "mongodb":
		config = createMongoDBConfig()
	default:
		log.Fatalw("Unsupported database type", "type", *dbType)
	}

	// Create and connect adapter
	dbFactory := factory.NewDatabaseAdapterFactory()

	if err := dbFactory.ValidateConfig(config); err != nil {
		log.Fatalw("Invalid configuration: %v", err)
	}

	adapter, err := dbFactory.CreateAndConnectAdapter(ctx, config)
	if err != nil {
		log.Fatalw("Failed to connect to database: %v", err)
	}
	defer adapter.Disconnect(ctx)

	log.Infof("✓ Connected to %s database", adapter.GetType())

	// Perform health check
	if err := adapter.HealthCheck(ctx); err != nil {
		log.Fatalw("Health check failed: %v", err)
	}
	log.Infof("✓ Health check passed")

	// Print connection stats
	stats := adapter.GetConnectionStats()
	log.Infof("✓ Connection stats: %d/%d connections (healthy: %v)",
		stats.OpenConnections, stats.MaxConnections, stats.Healthy)

	// Execute action
	switch *action {
	case "demo":
		runDemoScenario(ctx, adapter)
	case "migrate":
		runMigration(ctx)
	case "cleanup":
		runCleanup(ctx, adapter)
	case "stats":
		runStatistics(ctx, adapter)
	default:
		log.Fatalw("Unknown action: %s", *action)
	}
}

// createPostgresConfig creates PostgreSQL configuration
func createPostgresConfig() *ports.DatabaseConfig {
	return &ports.DatabaseConfig{
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
}

// createMongoDBConfig creates MongoDB configuration
func createMongoDBConfig() *ports.DatabaseConfig {
	return &ports.DatabaseConfig{
		Type: ports.DatabaseTypeMongoDB,
		MongoDBConfig: &ports.MongoDBConfig{
			URI:                "mongodb://localhost:27017",
			Database:           "eir",
			MaxPoolSize:        100,
			MinPoolSize:        10,
			MaxConnIdleTime:    600,
			ServerTimeout:      30,
			SocketTimeout:      30,
			ReadPreference:     "primary",
			WriteConcern:       "majority",
			EnableChangeStream: false,
		},
	}
}

// runDemoScenario demonstrates all database adapter features
func runDemoScenario(ctx context.Context, adapter ports.DatabaseAdapter) {
	imeiRepo := adapter.GetIMEIRepository()
	auditRepo := adapter.GetAuditRepository()
	extAuditRepo := adapter.GetExtendedAuditRepository()
	historyRepo := adapter.GetHistoryRepository()
	snapshotRepo := adapter.GetSnapshotRepository()

	// 1. Create equipment
	log.Info("1. Creating equipment...")
	equipment := &models.Equipment{
		IMEI:             "123456789012345",
		Status:           models.EquipmentStatusWhitelisted,
		AddedBy:          "admin",
		LastUpdated:      time.Now(),
		CheckCount:       0,
		ManufacturerTAC:  strPtr("12345678"),
		ManufacturerName: strPtr("Apple iPhone"),
		Reason:           strPtr("New device registration"),
	}

	if err := imeiRepo.Create(ctx, equipment); err != nil {
		log.Infof("  ⚠ Equipment already exists or error: %v", err)
		// Try to get existing
		equipment, _ = imeiRepo.GetByIMEI(ctx, "123456789012345")
	} else {
		log.Infof("  ✓ Created equipment ID: %d", equipment.ID)
	}

	// 2. Perform equipment checks with basic audit
	log.Info("\n2. Performing equipment checks with audit logging...")
	for i := 0; i < 3; i++ {
		audit := &models.AuditLog{
			IMEI:          equipment.IMEI,
			Status:        equipment.Status,
			CheckTime:     time.Now(),
			RequestSource: "HTTP_5G",
			SUPI:          strPtr("imsi-123456789012345"),
			GPSI:          strPtr("msisdn-1234567890"),
			SessionID:     strPtr(fmt.Sprintf("session-%d", i+1)),
		}

		if err := auditRepo.LogCheck(ctx, audit); err != nil {
			log.Infof("  ✗ Failed to log audit: %v", err)
		} else {
			log.Infof("  ✓ Logged check #%d (audit ID: %d)", i+1, audit.ID)
		}

		// Increment check count
		if err := imeiRepo.IncrementCheckCount(ctx, equipment.IMEI); err != nil {
			log.Infof("  ✗ Failed to increment count: %v", err)
		}

		time.Sleep(100 * time.Millisecond)
	}

	// 3. Extended audit with metrics
	log.Info("\n3. Performing extended audit with metrics...")
	startTime := time.Now()
	time.Sleep(50 * time.Millisecond) // Simulate processing
	processingTime := time.Since(startTime).Milliseconds()

	extAudit := &models.AuditLogExtended{
		AuditLog: models.AuditLog{
			IMEI:          equipment.IMEI,
			Status:        equipment.Status,
			CheckTime:     time.Now(),
			RequestSource: "HTTP_5G",
			SUPI:          strPtr("imsi-123456789012345"),
		},
		IPAddress:        strPtr("192.168.1.100"),
		UserAgent:        strPtr("EIR-Client/1.0"),
		ProcessingTimeMs: &processingTime,
		AdditionalData: map[string]interface{}{
			"client_version": "1.0.0",
			"region":         "US-WEST",
			"protocol":       "5G",
		},
		ChangeHistory: &models.EquipmentHistory{
			IMEI:       equipment.IMEI,
			ChangeType: models.ChangeTypeCheck,
			ChangedAt:  time.Now(),
			ChangedBy:  "system",
			NewStatus:  equipment.Status,
		},
	}

	if err := extAuditRepo.LogCheckExtended(ctx, extAudit); err != nil {
		log.Infof("  ✗ Failed to log extended audit: %v", err)
	} else {
		log.Infof("  ✓ Logged extended audit (processing time: %d ms)", processingTime)
	}

	// 4. Create snapshot
	log.Info("\n4. Creating equipment snapshot...")
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

	if err := snapshotRepo.CreateSnapshot(ctx, snapshot); err != nil {
		log.Infof("  ✗ Failed to create snapshot: %v", err)
	} else {
		log.Infof("  ✓ Created snapshot ID: %d", snapshot.ID)
	}

	// 5. Update equipment status with transaction
	log.Info("\n5. Updating equipment status (with transaction)...")
	if err := updateEquipmentWithTransaction(ctx, adapter, equipment.IMEI, models.EquipmentStatusGreylisted); err != nil {
		log.Infof("  ✗ Failed to update: %v", err)
	} else {
		log.Infof("  ✓ Updated equipment status to GREYLISTED")
	}

	// 6. Query audit history
	log.Info("\n6. Querying audit history...")
	audits, err := auditRepo.GetAuditsByIMEI(ctx, equipment.IMEI, 0, 10)
	if err != nil {
		log.Infof("  ✗ Failed to query audits: %v", err)
	} else {
		log.Infof("  ✓ Found %d audit entries", len(audits))
		for i, audit := range audits {
			log.Infof("    [%d] %s - Status: %s, Source: %s",
				i+1, audit.CheckTime.Format("2006-01-02 15:04:05"),
				audit.Status, audit.RequestSource)
		}
	}

	// 7. Query change history
	log.Info("\n7. Querying change history...")
	history, err := historyRepo.GetHistoryByIMEI(ctx, equipment.IMEI, 0, 10)
	if err != nil {
		log.Infof("  ✗ Failed to query history: %v", err)
	} else {
		log.Infof("  ✓ Found %d change history entries", len(history))
		for i, h := range history {
			log.Infof("    [%d] %s - Type: %s, By: %s",
				i+1, h.ChangedAt.Format("2006-01-02 15:04:05"),
				h.ChangeType, h.ChangedBy)
		}
	}

	// 8. Get equipment with updated info
	log.Info("\n8. Retrieving updated equipment...")
	updatedEquipment, err := imeiRepo.GetByIMEI(ctx, equipment.IMEI)
	if err != nil {
		log.Infof("  ✗ Failed to retrieve: %v", err)
	} else {
		log.Infof("  ✓ Equipment Status: %s", updatedEquipment.Status)
		log.Infof("  ✓ Check Count: %d", updatedEquipment.CheckCount)
		log.Infof("  ✓ Last Updated: %s", updatedEquipment.LastUpdated.Format("2006-01-02 15:04:05"))
	}

	log.Info("\n=== Demo Complete ===")
}

// updateEquipmentWithTransaction demonstrates transaction usage
func updateEquipmentWithTransaction(ctx context.Context, adapter ports.DatabaseAdapter, imei string, newStatus models.EquipmentStatus) error {
	tx, err := adapter.BeginTransaction(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	imeiRepo := tx.GetIMEIRepository()
	auditRepo := tx.GetAuditRepository()

	// Get equipment
	equipment, err := imeiRepo.GetByIMEI(ctx, imei)
	if err != nil {
		return err
	}

	// Update status
	equipment.Status = newStatus
	equipment.LastUpdated = time.Now()
	equipment.Reason = strPtr("Status updated via transaction")

	if err := imeiRepo.Update(ctx, equipment); err != nil {
		return err
	}

	// Log audit
	audit := &models.AuditLog{
		IMEI:          imei,
		Status:        newStatus,
		CheckTime:     time.Now(),
		RequestSource: "ADMIN_UPDATE",
	}

	if err := auditRepo.LogCheck(ctx, audit); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// runMigration demonstrates data migration between databases
func runMigration(ctx context.Context) {

	dbFactory := factory.NewDatabaseAdapterFactory()

	// Connect to source (PostgreSQL)
	pgConfig := createPostgresConfig()
	pgAdapter, err := dbFactory.CreateAndConnectAdapter(ctx, pgConfig)
	if err != nil {
		log.Fatalw("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgAdapter.Disconnect(ctx)

	// Connect to target (MongoDB)
	mongoConfig := createMongoDBConfig()
	mongoAdapter, err := dbFactory.CreateAndConnectAdapter(ctx, mongoConfig)
	if err != nil {
		log.Fatalw("Failed to connect to MongoDB: %v", err)
	}
	defer mongoAdapter.Disconnect(ctx)

	log.Info("✓ Connected to both databases")

	// Migrate equipment
	offset := 0
	limit := 100
	totalMigrated := 0

	for {
		equipments, err := pgAdapter.GetIMEIRepository().List(ctx, offset, limit)
		if err != nil || len(equipments) == 0 {
			break
		}

		for _, equipment := range equipments {
			err := mongoAdapter.GetIMEIRepository().Create(ctx, equipment)
			if err != nil {
				log.Infof("  ⚠ Failed to migrate %s: %v", equipment.IMEI, err)
			} else {
				totalMigrated++
			}
		}

		offset += limit
		log.Infof("  Migrated %d equipment records...", totalMigrated)
	}

	log.Infof("\n✓ Migration complete: %d records migrated", totalMigrated)
}

// runCleanup demonstrates data cleanup operations
func runCleanup(ctx context.Context, adapter ports.DatabaseAdapter) {

	// Delete audits older than 90 days
	cutoffDate := time.Now().Add(-90 * 24 * time.Hour).Format("2006-01-02")
	log.Infof("Cleaning up data older than %s...", cutoffDate)

	auditCount, err := adapter.PurgeOldAudits(ctx, cutoffDate)
	if err != nil {
		log.Infof("  ✗ Failed to purge audits: %v", err)
	} else {
		log.Infof("  ✓ Purged %d old audit records", auditCount)
	}

	historyCount, err := adapter.PurgeOldHistory(ctx, cutoffDate)
	if err != nil {
		log.Infof("  ✗ Failed to purge history: %v", err)
	} else {
		log.Infof("  ✓ Purged %d old history records", historyCount)
	}

	// Delete old snapshots
	snapshotRepo := adapter.GetSnapshotRepository()
	snapshotCutoff := time.Now().Add(-30 * 24 * time.Hour)
	snapshotCount, err := snapshotRepo.DeleteOldSnapshots(ctx, snapshotCutoff)
	if err != nil {
		log.Infof("  ✗ Failed to delete snapshots: %v", err)
	} else {
		log.Infof("  ✓ Deleted %d old snapshots", snapshotCount)
	}

	// Optimize database
	log.Info("\nOptimizing database...")
	if err := adapter.OptimizeDatabase(ctx); err != nil {
		log.Infof("  ✗ Failed to optimize: %v", err)
	} else {
		log.Infof("  ✓ Database optimized")
	}

	log.Info("\n=== Cleanup Complete ===")
}

// runStatistics demonstrates statistics gathering
func runStatistics(ctx context.Context, adapter ports.DatabaseAdapter) {

	extAuditRepo := adapter.GetExtendedAuditRepository()

	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	stats, err := extAuditRepo.GetAuditStatistics(ctx, startTime, endTime)
	if err != nil {
		log.Infof("✗ Failed to get statistics: %v", err)
		return
	}

	log.Info("Audit Statistics (last 24 hours):")
	log.Infof("  Total Checks:           %v", stats["total_checks"])
	log.Infof("  Unique IMEIs:           %v", stats["unique_imeis"])
	log.Infof("  Whitelisted:            %v", stats["whitelisted_count"])
	log.Infof("  Blacklisted:            %v", stats["blacklisted_count"])
	log.Infof("  Greylisted:             %v", stats["greylisted_count"])
	log.Infof("  Diameter Checks:        %v", stats["diameter_checks"])
	log.Infof("  HTTP Checks:            %v", stats["http_checks"])
	log.Infof("  Avg Processing Time:    %.2f ms", stats["avg_processing_time_ms"])

	log.Info("\n=== Statistics Complete ===")
}

// strPtr is a helper function to create string pointers
func strPtr(s string) *string {
	return &s
}
