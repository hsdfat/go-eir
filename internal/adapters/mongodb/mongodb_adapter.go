package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/hsdfat8/eir/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// MongoDBAdapter implements the DatabaseAdapter interface for MongoDB
type MongoDBAdapter struct {
	client              *mongo.Client
	db                  *mongo.Database
	config              *ports.MongoDBConfig
	imeiRepo            ports.IMEIRepository
	auditRepo           ports.AuditRepository
	extendedAuditRepo   ports.ExtendedAuditRepository
	historyRepo         ports.HistoryRepository
	snapshotRepo        ports.SnapshotRepository
}

// NewMongoDBAdapter creates a new MongoDB database adapter
func NewMongoDBAdapter(config *ports.MongoDBConfig) *MongoDBAdapter {
	return &MongoDBAdapter{
		config: config,
	}
}

// Connect establishes a connection to the MongoDB database
func (a *MongoDBAdapter) Connect(ctx context.Context) error {
	clientOpts := options.Client().ApplyURI(a.config.URI)

	// Configure connection pool
	if a.config.MaxPoolSize > 0 {
		clientOpts.SetMaxPoolSize(uint64(a.config.MaxPoolSize))
	}
	if a.config.MinPoolSize > 0 {
		clientOpts.SetMinPoolSize(uint64(a.config.MinPoolSize))
	}
	if a.config.MaxConnIdleTime > 0 {
		clientOpts.SetMaxConnIdleTime(time.Duration(a.config.MaxConnIdleTime) * time.Second)
	}
	if a.config.ServerTimeout > 0 {
		clientOpts.SetServerSelectionTimeout(time.Duration(a.config.ServerTimeout) * time.Second)
	}
	if a.config.SocketTimeout > 0 {
		clientOpts.SetSocketTimeout(time.Duration(a.config.SocketTimeout) * time.Second)
	}

	// Set read preference
	if a.config.ReadPreference != "" {
		switch a.config.ReadPreference {
		case "primary":
			clientOpts.SetReadPreference(readpref.Primary())
		case "secondary":
			clientOpts.SetReadPreference(readpref.Secondary())
		case "primaryPreferred":
			clientOpts.SetReadPreference(readpref.PrimaryPreferred())
		case "secondaryPreferred":
			clientOpts.SetReadPreference(readpref.SecondaryPreferred())
		}
	}

	// Set write concern
	if a.config.WriteConcern != "" {
		switch a.config.WriteConcern {
		case "majority":
			clientOpts.SetWriteConcern(writeconcern.Majority())
		}
	}

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	// Ping to verify connection
	if err = client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping mongodb: %w", err)
	}

	a.client = client
	a.db = client.Database(a.config.Database)

	// Initialize repositories
	a.imeiRepo = NewIMEIRepository(a.db)
	a.auditRepo = NewAuditRepository(a.db)
	a.extendedAuditRepo = NewExtendedAuditRepository(a.db)
	a.historyRepo = NewHistoryRepository(a.db)
	a.snapshotRepo = NewSnapshotRepository(a.db)

	// Create indexes
	if err = a.createIndexes(ctx); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	return nil
}

// Disconnect closes the database connection
func (a *MongoDBAdapter) Disconnect(ctx context.Context) error {
	if a.client != nil {
		return a.client.Disconnect(ctx)
	}
	return nil
}

// Ping checks if the database connection is alive
func (a *MongoDBAdapter) Ping(ctx context.Context) error {
	if a.client == nil {
		return fmt.Errorf("database not connected")
	}
	return a.client.Ping(ctx, nil)
}

// GetType returns the database type
func (a *MongoDBAdapter) GetType() ports.DatabaseType {
	return ports.DatabaseTypeMongoDB
}

// BeginTransaction starts a new database transaction (session)
func (a *MongoDBAdapter) BeginTransaction(ctx context.Context) (ports.Transaction, error) {
	session, err := a.client.StartSession()
	if err != nil {
		return nil, fmt.Errorf("failed to start session: %w", err)
	}

	err = session.StartTransaction()
	if err != nil {
		session.EndSession(ctx)
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	return &mongoTransaction{
		session:   session,
		db:        a.db,
		imeiRepo:  NewIMEIRepository(a.db),
		auditRepo: NewAuditRepository(a.db),
	}, nil
}

// GetIMEIRepository returns the IMEI repository
func (a *MongoDBAdapter) GetIMEIRepository() ports.IMEIRepository {
	return a.imeiRepo
}

// GetAuditRepository returns the audit repository
func (a *MongoDBAdapter) GetAuditRepository() ports.AuditRepository {
	return a.auditRepo
}

// GetExtendedAuditRepository returns the extended audit repository
func (a *MongoDBAdapter) GetExtendedAuditRepository() ports.ExtendedAuditRepository {
	return a.extendedAuditRepo
}

// GetHistoryRepository returns the history repository
func (a *MongoDBAdapter) GetHistoryRepository() ports.HistoryRepository {
	return a.historyRepo
}

// GetSnapshotRepository returns the snapshot repository
func (a *MongoDBAdapter) GetSnapshotRepository() ports.SnapshotRepository {
	return a.snapshotRepo
}

// HealthCheck performs a health check on the database
func (a *MongoDBAdapter) HealthCheck(ctx context.Context) error {
	if err := a.Ping(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}

	// Test a simple query
	_, err := a.db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("health check query failed: %w", err)
	}

	return nil
}

// GetConnectionStats returns database connection statistics
func (a *MongoDBAdapter) GetConnectionStats() ports.ConnectionStats {
	healthy := a.Ping(context.Background()) == nil

	return ports.ConnectionStats{
		OpenConnections:  -1, // MongoDB driver doesn't expose this easily
		IdleConnections:  -1,
		MaxConnections:   a.config.MaxPoolSize,
		DatabaseType:     string(ports.DatabaseTypeMongoDB),
		ConnectionString: a.config.Database, // Don't expose full URI
		Healthy:          healthy,
	}
}

// PurgeOldAudits removes audit logs older than the specified date
func (a *MongoDBAdapter) PurgeOldAudits(ctx context.Context, beforeDate string) (int64, error) {
	result, err := a.db.Collection("audit_log").DeleteMany(ctx, bson.M{
		"check_time": bson.M{"$lt": beforeDate},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to purge old audits: %w", err)
	}

	return result.DeletedCount, nil
}

// PurgeOldHistory removes history records older than the specified date
func (a *MongoDBAdapter) PurgeOldHistory(ctx context.Context, beforeDate string) (int64, error) {
	result, err := a.db.Collection("equipment_history").DeleteMany(ctx, bson.M{
		"changed_at": bson.M{"$lt": beforeDate},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to purge old history: %w", err)
	}

	return result.DeletedCount, nil
}

// OptimizeDatabase performs database optimization operations
func (a *MongoDBAdapter) OptimizeDatabase(ctx context.Context) error {
	// Run compact on collections
	collections := []string{"equipment", "audit_log", "equipment_history", "equipment_snapshots", "imei_info", "tac_info"}

	for _, collection := range collections {
		var result bson.M
		err := a.db.RunCommand(ctx, bson.D{
			{Key: "compact", Value: collection},
		}).Decode(&result)
		if err != nil {
			return fmt.Errorf("failed to compact collection %s: %w", collection, err)
		}
	}

	return nil
}

// createIndexes creates necessary indexes for optimal performance
func (a *MongoDBAdapter) createIndexes(ctx context.Context) error {
	// Equipment collection indexes
	equipmentIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "imei", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "manufacturer_tac", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "last_check_time", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "check_count", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "last_updated", Value: -1}},
		},
	}

	_, err := a.db.Collection("equipment").Indexes().CreateMany(ctx, equipmentIndexes)
	if err != nil {
		return fmt.Errorf("failed to create equipment indexes: %w", err)
	}

	// Audit log collection indexes
	auditIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "imei", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "check_time", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "request_source", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "supi", Value: 1}},
		},
		{
			Keys: bson.D{
				{Key: "imei", Value: 1},
				{Key: "check_time", Value: -1},
			},
		},
	}

	_, err = a.db.Collection("audit_log").Indexes().CreateMany(ctx, auditIndexes)
	if err != nil {
		return fmt.Errorf("failed to create audit log indexes: %w", err)
	}

	// Equipment history collection indexes
	historyIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "imei", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "changed_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "change_type", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "changed_by", Value: 1}},
		},
	}

	_, err = a.db.Collection("equipment_history").Indexes().CreateMany(ctx, historyIndexes)
	if err != nil {
		return fmt.Errorf("failed to create history indexes: %w", err)
	}

	// Equipment snapshots collection indexes
	snapshotIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "imei", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "snapshot_time", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "equipment_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "snapshot_type", Value: 1}},
		},
	}

	_, err = a.db.Collection("equipment_snapshots").Indexes().CreateMany(ctx, snapshotIndexes)
	if err != nil {
		return fmt.Errorf("failed to create snapshot indexes: %w", err)
	}

	// IMEI info collection indexes
	imeiInfoIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "startimei", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err = a.db.Collection("imei_info").Indexes().CreateMany(ctx, imeiInfoIndexes)
	if err != nil {
		return fmt.Errorf("failed to create imei_info indexes: %w", err)
	}

	// TAC info collection indexes
	tacInfoIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "keytac", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err = a.db.Collection("tac_info").Indexes().CreateMany(ctx, tacInfoIndexes)
	if err != nil {
		return fmt.Errorf("failed to create tac_info indexes: %w", err)
	}

	return nil
}

// mongoTransaction implements the Transaction interface
type mongoTransaction struct {
	session   mongo.Session
	db        *mongo.Database
	imeiRepo  ports.IMEIRepository
	auditRepo ports.AuditRepository
}

// Commit commits the transaction
func (t *mongoTransaction) Commit(ctx context.Context) error {
	err := t.session.CommitTransaction(ctx)
	t.session.EndSession(ctx)
	return err
}

// Rollback rolls back the transaction
func (t *mongoTransaction) Rollback(ctx context.Context) error {
	err := t.session.AbortTransaction(ctx)
	t.session.EndSession(ctx)
	return err
}

// GetIMEIRepository returns a transactional IMEI repository
func (t *mongoTransaction) GetIMEIRepository() ports.IMEIRepository {
	return t.imeiRepo
}

// GetAuditRepository returns a transactional audit repository
func (t *mongoTransaction) GetAuditRepository() ports.AuditRepository {
	return t.auditRepo
}
