package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// historyRepository implements the HistoryRepository interface using MongoDB
type historyRepository struct {
	collection *mongo.Collection
}

// NewHistoryRepository creates a new MongoDB history repository
func NewHistoryRepository(db *mongo.Database) ports.HistoryRepository {
	return &historyRepository{
		collection: db.Collection("equipment_history"),
	}
}

// RecordChange records a change to equipment status or metadata
func (r *historyRepository) RecordChange(ctx context.Context, history *models.EquipmentHistory) error {
	result, err := r.collection.InsertOne(ctx, history)
	if err != nil {
		return fmt.Errorf("failed to record change: %w", err)
	}

	if oid, ok := result.InsertedID.(int64); ok {
		history.ID = oid
	}

	return nil
}

// GetHistoryByIMEI retrieves change history for a specific IMEI
func (r *historyRepository) GetHistoryByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.EquipmentHistory, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "changed_at", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{"imei": imei}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get history by IMEI: %w", err)
	}
	defer cursor.Close(ctx)

	var history []*models.EquipmentHistory
	if err = cursor.All(ctx, &history); err != nil {
		return nil, fmt.Errorf("failed to decode history: %w", err)
	}

	return history, nil
}

// GetHistoryByTimeRange retrieves change history within a time range
func (r *historyRepository) GetHistoryByTimeRange(ctx context.Context, startTime, endTime time.Time, offset, limit int) ([]*models.EquipmentHistory, error) {
	filter := bson.M{
		"changed_at": bson.M{
			"$gte": startTime,
			"$lte": endTime,
		},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "changed_at", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get history by time range: %w", err)
	}
	defer cursor.Close(ctx)

	var history []*models.EquipmentHistory
	if err = cursor.All(ctx, &history); err != nil {
		return nil, fmt.Errorf("failed to decode history: %w", err)
	}

	return history, nil
}

// GetHistoryByChangeType retrieves history filtered by change type
func (r *historyRepository) GetHistoryByChangeType(ctx context.Context, changeType models.ChangeType, offset, limit int) ([]*models.EquipmentHistory, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "changed_at", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{"change_type": changeType}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get history by change type: %w", err)
	}
	defer cursor.Close(ctx)

	var history []*models.EquipmentHistory
	if err = cursor.All(ctx, &history); err != nil {
		return nil, fmt.Errorf("failed to decode history: %w", err)
	}

	return history, nil
}
