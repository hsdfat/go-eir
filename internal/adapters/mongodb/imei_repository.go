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

var (
	ErrNotFound      = errors.New("equipment not found")
	ErrAlreadyExists = errors.New("equipment already exists")
)

// imeiRepository implements the IMEIRepository interface using MongoDB
type imeiRepository struct {
	collection *mongo.Collection
}

// NewIMEIRepository creates a new MongoDB IMEI repository
func NewIMEIRepository(db *mongo.Database) ports.IMEIRepository {
	return &imeiRepository{
		collection: db.Collection("equipment"),
	}
}

// GetByIMEI retrieves equipment by IMEI
func (r *imeiRepository) GetByIMEI(ctx context.Context, imei string) (*models.Equipment, error) {
	var equipment models.Equipment

	err := r.collection.FindOne(ctx, bson.M{"imei": imei}).Decode(&equipment)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get equipment: %w", err)
	}

	return &equipment, nil
}

// GetByIMEISV retrieves equipment by IMEISV
func (r *imeiRepository) GetByIMEISV(ctx context.Context, imeisv string) (*models.Equipment, error) {
	var equipment models.Equipment

	err := r.collection.FindOne(ctx, bson.M{"imeisv": imeisv}).Decode(&equipment)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get equipment by IMEISV: %w", err)
	}

	return &equipment, nil
}

// Create adds a new equipment record
func (r *imeiRepository) Create(ctx context.Context, equipment *models.Equipment) error {
	// Check if IMEI already exists
	existing, err := r.GetByIMEI(ctx, equipment.IMEI)
	if err == nil && existing != nil {
		return ErrAlreadyExists
	}

	equipment.LastUpdated = time.Now()

	result, err := r.collection.InsertOne(ctx, equipment)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return ErrAlreadyExists
		}
		return fmt.Errorf("failed to create equipment: %w", err)
	}

	if oid, ok := result.InsertedID.(int64); ok {
		equipment.ID = oid
	}

	return nil
}

// Update updates an existing equipment record
func (r *imeiRepository) Update(ctx context.Context, equipment *models.Equipment) error {
	equipment.LastUpdated = time.Now()

	update := bson.M{
		"$set": bson.M{
			"imeisv":            equipment.IMEISV,
			"status":            equipment.Status,
			"reason":            equipment.Reason,
			"last_updated":      equipment.LastUpdated,
			"metadata":          equipment.Metadata,
			"manufacturer_tac":  equipment.ManufacturerTAC,
			"manufacturer_name": equipment.ManufacturerName,
		},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"imei": equipment.IMEI}, update)
	if err != nil {
		return fmt.Errorf("failed to update equipment: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete removes an equipment record
func (r *imeiRepository) Delete(ctx context.Context, imei string) error {
	result, err := r.collection.DeleteOne(ctx, bson.M{"imei": imei})
	if err != nil {
		return fmt.Errorf("failed to delete equipment: %w", err)
	}

	if result.DeletedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// List retrieves equipment with pagination
func (r *imeiRepository) List(ctx context.Context, offset, limit int) ([]*models.Equipment, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "last_updated", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list equipment: %w", err)
	}
	defer cursor.Close(ctx)

	var equipments []*models.Equipment
	if err = cursor.All(ctx, &equipments); err != nil {
		return nil, fmt.Errorf("failed to decode equipment: %w", err)
	}

	return equipments, nil
}

// ListByStatus retrieves equipment by status with pagination
func (r *imeiRepository) ListByStatus(ctx context.Context, status models.EquipmentStatus, offset, limit int) ([]*models.Equipment, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "last_updated", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{"status": status}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list equipment by status: %w", err)
	}
	defer cursor.Close(ctx)

	var equipments []*models.Equipment
	if err = cursor.All(ctx, &equipments); err != nil {
		return nil, fmt.Errorf("failed to decode equipment: %w", err)
	}

	return equipments, nil
}

// IncrementCheckCount atomically increments check counter and updates last check time
func (r *imeiRepository) IncrementCheckCount(ctx context.Context, imei string) error {
	update := bson.M{
		"$inc": bson.M{"check_count": 1},
		"$set": bson.M{"last_check_time": time.Now()},
	}

	result, err := r.collection.UpdateOne(ctx, bson.M{"imei": imei}, update)
	if err != nil {
		return fmt.Errorf("failed to increment check count: %w", err)
	}

	if result.MatchedCount == 0 {
		return ErrNotFound
	}

	return nil
}

// IMEI logic operations (not implemented for MongoDB - use in-memory for testing)
func (r *imeiRepository) LookupImeiInfo(ctx context.Context, startRange string) (*ports.ImeiInfo, bool) {
	return nil, false
}

func (r *imeiRepository) SaveImeiInfo(ctx context.Context, info *ports.ImeiInfo) error {
	return fmt.Errorf("not implemented")
}

func (r *imeiRepository) ListAllImeiInfo(ctx context.Context) []ports.ImeiInfo {
	return []ports.ImeiInfo{}
}

func (r *imeiRepository) ClearImeiInfo() {
	// No-op
}

// TAC logic operations (not implemented for MongoDB - use in-memory for testing)
func (r *imeiRepository) SaveTacInfo(ctx context.Context, info *ports.TacInfo) error {
	return fmt.Errorf("not implemented")
}

func (r *imeiRepository) LookupTacInfo(ctx context.Context, key string) (*ports.TacInfo, bool) {
	return nil, false
}

func (r *imeiRepository) PrevTacInfo(ctx context.Context, key string) (*ports.TacInfo, bool) {
	return nil, false
}

func (r *imeiRepository) NextTacInfo(ctx context.Context, key string) (*ports.TacInfo, bool) {
	return nil, false
}

func (r *imeiRepository) ListAllTacInfo(ctx context.Context) []*ports.TacInfo {
	return []*ports.TacInfo{}
}
