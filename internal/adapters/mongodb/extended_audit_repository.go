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

// extendedAuditRepository implements the ExtendedAuditRepository interface using MongoDB
type extendedAuditRepository struct {
	auditRepository
	historyRepo ports.HistoryRepository
}

// NewExtendedAuditRepository creates a new MongoDB extended audit repository
func NewExtendedAuditRepository(db *mongo.Database) ports.ExtendedAuditRepository {
	return &extendedAuditRepository{
		auditRepository: auditRepository{collection: db.Collection("audit_log")},
		historyRepo:     NewHistoryRepository(db),
	}
}

// LogCheckExtended records an extended equipment check with additional metadata
func (r *extendedAuditRepository) LogCheckExtended(ctx context.Context, audit *models.AuditLogExtended) error {
	// MongoDB stores the extended audit as a single document with embedded fields
	result, err := r.collection.InsertOne(ctx, audit)
	if err != nil {
		return fmt.Errorf("failed to log extended check: %w", err)
	}

	if oid, ok := result.InsertedID.(int64); ok {
		audit.ID = oid
	}

	// Record change history if provided
	if audit.ChangeHistory != nil {
		err = r.historyRepo.RecordChange(ctx, audit.ChangeHistory)
		if err != nil {
			return fmt.Errorf("failed to record change history: %w", err)
		}
	}

	return nil
}

// GetExtendedAuditsByIMEI retrieves extended audit logs for a specific IMEI
func (r *extendedAuditRepository) GetExtendedAuditsByIMEI(ctx context.Context, imei string, offset, limit int) ([]*models.AuditLogExtended, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "check_time", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{"imei": imei}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get extended audits by IMEI: %w", err)
	}
	defer cursor.Close(ctx)

	var audits []*models.AuditLogExtended
	if err = cursor.All(ctx, &audits); err != nil {
		return nil, fmt.Errorf("failed to decode extended audits: %w", err)
	}

	return audits, nil
}

// GetAuditsByRequestSource retrieves audits filtered by request source
func (r *extendedAuditRepository) GetAuditsByRequestSource(ctx context.Context, requestSource string, offset, limit int) ([]*models.AuditLog, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "check_time", Value: -1}}).
		SetSkip(int64(offset)).
		SetLimit(int64(limit))

	cursor, err := r.collection.Find(ctx, bson.M{"request_source": requestSource}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get audits by request source: %w", err)
	}
	defer cursor.Close(ctx)

	var audits []*models.AuditLog
	if err = cursor.All(ctx, &audits); err != nil {
		return nil, fmt.Errorf("failed to decode audits: %w", err)
	}

	return audits, nil
}

// GetAuditStatistics retrieves aggregated audit statistics
func (r *extendedAuditRepository) GetAuditStatistics(ctx context.Context, startTime, endTime time.Time) (map[string]interface{}, error) {
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"check_time": bson.M{
					"$gte": startTime,
					"$lte": endTime,
				},
			},
		},
		{
			"$group": bson.M{
				"_id": nil,
				"total_checks": bson.M{"$sum": 1},
				"unique_imeis": bson.M{"$addToSet": "$imei"},
				"whitelisted_count": bson.M{
					"$sum": bson.M{
						"$cond": bson.A{
							bson.M{"$eq": bson.A{"$status", "WHITELISTED"}},
							1,
							0,
						},
					},
				},
				"blacklisted_count": bson.M{
					"$sum": bson.M{
						"$cond": bson.A{
							bson.M{"$eq": bson.A{"$status", "BLACKLISTED"}},
							1,
							0,
						},
					},
				},
				"greylisted_count": bson.M{
					"$sum": bson.M{
						"$cond": bson.A{
							bson.M{"$eq": bson.A{"$status", "GREYLISTED"}},
							1,
							0,
						},
					},
				},
				"diameter_checks": bson.M{
					"$sum": bson.M{
						"$cond": bson.A{
							bson.M{"$eq": bson.A{"$request_source", "DIAMETER_S13"}},
							1,
							0,
						},
					},
				},
				"http_checks": bson.M{
					"$sum": bson.M{
						"$cond": bson.A{
							bson.M{"$eq": bson.A{"$request_source", "HTTP_5G"}},
							1,
							0,
						},
					},
				},
				"avg_processing_time_ms": bson.M{"$avg": "$processing_time_ms"},
			},
		},
		{
			"$project": bson.M{
				"_id":                    0,
				"total_checks":           1,
				"unique_imeis":           bson.M{"$size": "$unique_imeis"},
				"whitelisted_count":      1,
				"blacklisted_count":      1,
				"greylisted_count":       1,
				"diameter_checks":        1,
				"http_checks":            1,
				"avg_processing_time_ms": 1,
			},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit statistics: %w", err)
	}
	defer cursor.Close(ctx)

	var results []map[string]interface{}
	if err = cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode statistics: %w", err)
	}

	if len(results) == 0 {
		return map[string]interface{}{
			"total_checks":           0,
			"unique_imeis":           0,
			"whitelisted_count":      0,
			"blacklisted_count":      0,
			"greylisted_count":       0,
			"diameter_checks":        0,
			"http_checks":            0,
			"avg_processing_time_ms": 0,
		}, nil
	}

	return results[0], nil
}
