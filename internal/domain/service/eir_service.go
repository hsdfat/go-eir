package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
)

var (
	ErrEquipmentNotFound = errors.New("equipment not found")
	ErrInvalidRequest    = errors.New("invalid request")
)

// eirService implements the EIRService interface
type eirService struct {
	imeiRepo  ports.IMEIRepository
	auditRepo ports.AuditRepository
	cache     ports.CacheRepository // Optional
}

// NewEIRService creates a new EIR service instance
func NewEIRService(
	imeiRepo ports.IMEIRepository,
	auditRepo ports.AuditRepository,
	cache ports.CacheRepository,
) ports.EIRService {
	return &eirService{
		imeiRepo:  imeiRepo,
		auditRepo: auditRepo,
		cache:     cache,
	}
}

// CheckEquipment performs the core equipment identity check logic
func (s *eirService) CheckEquipment(ctx context.Context, request *ports.CheckEquipmentRequest) (*ports.CheckEquipmentResponse, error) {
	// Step 1: Validate IMEI format
	if err := models.ValidateIMEI(request.IMEI); err != nil {
		return nil, fmt.Errorf("IMEI validation failed: %w", err)
	}

	// Step 2: Check cache first (if available)
	var equipment *models.Equipment
	var err error

	if s.cache != nil {
		equipment, err = s.cache.Get(ctx, request.IMEI)
		if err == nil && equipment != nil {
			// Cache hit - increment check count asynchronously
			go s.incrementCheckCountAsync(request.IMEI)

			// Log audit asynchronously
			go s.logAuditAsync(ctx, request, equipment)

			return s.buildResponse(equipment), nil
		}
	}

	// Step 3: Query database
	equipment, err = s.imeiRepo.GetByIMEI(ctx, request.IMEI)
	if err != nil {
		// Equipment not in database - apply default policy
		return s.applyDefaultPolicy(ctx, request)
	}

	// Step 4: Increment check count and update last check time
	if err := s.imeiRepo.IncrementCheckCount(ctx, request.IMEI); err != nil {
		// Log error but don't fail the check
		fmt.Printf("Warning: failed to increment check count for IMEI %s: %v\n", request.IMEI, err)
	}

	// Step 5: Update cache
	if s.cache != nil {
		go s.updateCacheAsync(ctx, request.IMEI, equipment)
	}

	// Step 6: Log audit
	if err := s.logAudit(ctx, request, equipment); err != nil {
		// Log error but don't fail the check
		fmt.Printf("Warning: failed to log audit for IMEI %s: %v\n", request.IMEI, err)
	}

	// Step 7: Return response
	return s.buildResponse(equipment), nil
}

// applyDefaultPolicy applies the default policy for unknown equipment
// By default, unknown equipment is WHITELISTED (permissive policy)
// This can be configured based on operator requirements
func (s *eirService) applyDefaultPolicy(ctx context.Context, request *ports.CheckEquipmentRequest) (*ports.CheckEquipmentResponse, error) {
	// Default policy: WHITELIST unknown devices
	defaultStatus := models.EquipmentStatusWhitelisted
	reason := "Unknown equipment - default policy applied"

	// Log the check even for unknown equipment
	audit := &models.AuditLog{
		IMEI:          request.IMEI,
		IMEISV:        request.IMEISV,
		Status:        defaultStatus,
		CheckTime:     time.Now(),
		OriginHost:    request.OriginHost,
		OriginRealm:   request.OriginRealm,
		UserName:      request.UserName,
		SUPI:          request.SUPI,
		GPSI:          request.GPSI,
		RequestSource: request.RequestSource,
		SessionID:     request.SessionID,
	}

	if err := s.auditRepo.LogCheck(ctx, audit); err != nil {
		fmt.Printf("Warning: failed to log audit for unknown IMEI %s: %v\n", request.IMEI, err)
	}

	tac := models.ExtractTAC(request.IMEI)
	return &ports.CheckEquipmentResponse{
		IMEI:            request.IMEI,
		Status:          defaultStatus,
		Reason:          &reason,
		ManufacturerTAC: &tac,
	}, nil
}

// buildResponse constructs the check response from equipment data
func (s *eirService) buildResponse(equipment *models.Equipment) *ports.CheckEquipmentResponse {
	return &ports.CheckEquipmentResponse{
		IMEI:            equipment.IMEI,
		Status:          equipment.Status,
		Reason:          equipment.Reason,
		ManufacturerTAC: equipment.ManufacturerTAC,
	}
}

// logAudit records the equipment check in audit log
func (s *eirService) logAudit(ctx context.Context, request *ports.CheckEquipmentRequest, equipment *models.Equipment) error {
	audit := &models.AuditLog{
		IMEI:          request.IMEI,
		IMEISV:        request.IMEISV,
		Status:        equipment.Status,
		CheckTime:     time.Now(),
		OriginHost:    request.OriginHost,
		OriginRealm:   request.OriginRealm,
		UserName:      request.UserName,
		SUPI:          request.SUPI,
		GPSI:          request.GPSI,
		RequestSource: request.RequestSource,
		SessionID:     request.SessionID,
		ResultCode:    nil, // Success
	}

	return s.auditRepo.LogCheck(ctx, audit)
}

// logAuditAsync logs audit asynchronously (best effort)
func (s *eirService) logAuditAsync(ctx context.Context, request *ports.CheckEquipmentRequest, equipment *models.Equipment) {
	if err := s.logAudit(ctx, request, equipment); err != nil {
		fmt.Printf("Warning: async audit logging failed for IMEI %s: %v\n", request.IMEI, err)
	}
}

// updateCacheAsync updates cache asynchronously
func (s *eirService) updateCacheAsync(ctx context.Context, imei string, equipment *models.Equipment) {
	if err := s.cache.Set(ctx, imei, equipment, 300); err != nil { // 5 minutes TTL
		fmt.Printf("Warning: failed to update cache for IMEI %s: %v\n", imei, err)
	}
}

// incrementCheckCountAsync increments check count asynchronously
func (s *eirService) incrementCheckCountAsync(imei string) {
	ctx := context.Background()
	if err := s.imeiRepo.IncrementCheckCount(ctx, imei); err != nil {
		fmt.Printf("Warning: async increment check count failed for IMEI %s: %v\n", imei, err)
	}
}

// ProvisionEquipment adds or updates equipment in the system
func (s *eirService) ProvisionEquipment(ctx context.Context, request *ports.ProvisionEquipmentRequest) error {
	// Validate IMEI
	if err := models.ValidateIMEI(request.IMEI); err != nil {
		return fmt.Errorf("invalid IMEI: %w", err)
	}

	// Validate status
	if err := models.ValidateStatus(request.Status); err != nil {
		return fmt.Errorf("invalid status: %w", err)
	}

	// Check if equipment exists
	existing, err := s.imeiRepo.GetByIMEI(ctx, request.IMEI)

	equipment := &models.Equipment{
		IMEI:             request.IMEI,
		IMEISV:           request.IMEISV,
		Status:           request.Status,
		Reason:           request.Reason,
		LastUpdated:      time.Now(),
		AddedBy:          request.AddedBy,
		Metadata:         request.Metadata,
		ManufacturerTAC:  request.ManufacturerTAC,
		ManufacturerName: request.ManufacturerName,
	}

	if err != nil || existing == nil {
		// Create new equipment
		equipment.CheckCount = 0
		if err := s.imeiRepo.Create(ctx, equipment); err != nil {
			return fmt.Errorf("failed to create equipment: %w", err)
		}
	} else {
		// Update existing equipment
		equipment.ID = existing.ID
		equipment.CheckCount = existing.CheckCount // Preserve check count
		if err := s.imeiRepo.Update(ctx, equipment); err != nil {
			return fmt.Errorf("failed to update equipment: %w", err)
		}
	}

	// Invalidate cache
	if s.cache != nil {
		_ = s.cache.Delete(ctx, request.IMEI)
	}

	return nil
}

// RemoveEquipment removes equipment from the system
func (s *eirService) RemoveEquipment(ctx context.Context, imei string) error {
	if err := models.ValidateIMEI(imei); err != nil {
		return fmt.Errorf("invalid IMEI: %w", err)
	}

	if err := s.imeiRepo.Delete(ctx, imei); err != nil {
		return fmt.Errorf("failed to delete equipment: %w", err)
	}

	// Invalidate cache
	if s.cache != nil {
		_ = s.cache.Delete(ctx, imei)
	}

	return nil
}

// GetEquipment retrieves equipment information
func (s *eirService) GetEquipment(ctx context.Context, imei string) (*models.Equipment, error) {
	if err := models.ValidateIMEI(imei); err != nil {
		return nil, fmt.Errorf("invalid IMEI: %w", err)
	}

	// Try cache first
	if s.cache != nil {
		equipment, err := s.cache.Get(ctx, imei)
		if err == nil && equipment != nil {
			return equipment, nil
		}
	}

	// Query database
	equipment, err := s.imeiRepo.GetByIMEI(ctx, imei)
	if err != nil {
		return nil, ErrEquipmentNotFound
	}

	// Update cache
	if s.cache != nil {
		go s.updateCacheAsync(ctx, imei, equipment)
	}

	return equipment, nil
}

// ListEquipment retrieves paginated equipment list
func (s *eirService) ListEquipment(ctx context.Context, offset, limit int) ([]*models.Equipment, error) {
	return s.imeiRepo.List(ctx, offset, limit)
}
