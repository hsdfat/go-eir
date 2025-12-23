package mongodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// snapshotRepository implements the SnapshotRepository interface using MongoDB
type snapshotRepository struct {
	collection *mongo.Collection
}

// NewSnapshotRepository creates a new MongoDB snapshot repository
func NewSnapshotRepository(db *mongo.Database) ports.SnapshotRepository {
	return &snapshotRepository{
		collection: db.Collection("equipment_snapshots"),
	}
}

// CreateSnapshot creates a point-in-time snapshot of equipment
func (r *snapshotRepository) CreateSnapshot(ctx context.Context, snapshot *models.EquipmentSnapshot) error {
	result, err := r.collection.InsertOne(ctx, snapshot)
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	if oid, ok := result.InsertedID.(int64); ok {
		snapshot.ID = oid
	}

	return nil
}

// GetSnapshotsByIMEI retrieves snapshots for a specific IMEI
func (r *snapshotRepository) GetSnapshotsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.EquipmentSnapshot, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "snapshot_time", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{"imei": imei}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshots by IMEI: %w", err)
	}
	defer cursor.Close(ctx)

	var snapshots []*models.EquipmentSnapshot
	if err = cursor.All(ctx, &snapshots); err != nil {
		return nil, fmt.Errorf("failed to decode snapshots: %w", err)
	}

	return snapshots, nil
}

// GetSnapshotByID retrieves a specific snapshot
func (r *snapshotRepository) GetSnapshotByID(ctx context.Context, id int64) (*models.EquipmentSnapshot, error) {
	var snapshot models.EquipmentSnapshot

	err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&snapshot)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	return &snapshot, nil
}

// DeleteOldSnapshots removes snapshots older than the specified date
func (r *snapshotRepository) DeleteOldSnapshots(ctx context.Context, before time.Time) (int64, error) {
	result, err := r.collection.DeleteMany(ctx, bson.M{
		"snapshot_time": bson.M{"$lt": before},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to delete old snapshots: %w", err)
	}

	return result.DeletedCount, nil
}
