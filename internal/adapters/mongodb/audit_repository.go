package mongodb

import (
	"context"
	"fmt"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// auditRepository implements the AuditRepository interface using MongoDB
type auditRepository struct {
	collection *mongo.Collection
}

// NewAuditRepository creates a new MongoDB audit repository
func NewAuditRepository(db *mongo.Database) ports.AuditRepository {
	return &auditRepository{
		collection: db.Collection("audit_log"),
	}
}

// LogCheck records an equipment check operation
func (r *auditRepository) LogCheck(ctx context.Context, audit *models.AuditLog) error {
	result, err := r.collection.InsertOne(ctx, audit)
	if err != nil {
		return fmt.Errorf("failed to log check: %w", err)
	}

	if oid, ok := result.InsertedID.(int64); ok {
		audit.ID = oid
	}

	return nil
}

// GetAuditsByIMEI retrieves audit logs for a specific IMEI
func (r *auditRepository) GetAuditsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.AuditLog, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "check_time", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{"imei": imei}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get audits by IMEI: %w", err)
	}
	defer cursor.Close(ctx)

	var audits []*models.AuditLog
	if err = cursor.All(ctx, &audits); err != nil {
		return nil, fmt.Errorf("failed to decode audits: %w", err)
	}

	return audits, nil
}

// GetAuditsByTimeRange retrieves audit logs within a time range
func (r *auditRepository) GetAuditsByTimeRange(ctx context.Context, startTime, endTime string, offset, limit int) ([]*models.AuditLog, error) {
	filter := bson.M{
		"check_time": bson.M{
			"$gte": startTime,
			"$lte": endTime,
		},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "check_time", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get audits by time range: %w", err)
	}
	defer cursor.Close(ctx)

	var audits []*models.AuditLog
	if err = cursor.All(ctx, &audits); err != nil {
		return nil, fmt.Errorf("failed to decode audits: %w", err)
	}

	return audits, nil
}
