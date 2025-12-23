package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	legacyModels "github.com/hsdfat8/eir/models"
	"github.com/hsdfat8/eir/pkg/logic"
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

// CheckImei performs IMEI check using pkg/logic
func (s *eirService) CheckImei(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckImeiResult, error) {
	// Convert domain model to legacy model
	legacyStatus := legacyModels.SystemStatus{
		OverloadLevel: status.OverloadLevel,
		TPSOverload:   status.TPSOverload,
	}

	// Use pkg/logic for IMEI checking
	result := logic.CheckImei(imei, legacyStatus)

	return &ports.CheckImeiResult{
		Status: result.Status,
		IMEI:   result.IMEI,
		Color:  result.Color,
	}, nil
}

// CheckTac performs TAC-based equipment check using pkg/logic
func (s *eirService) CheckTac(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckTacResult, error) {
	// Validate IMEI format
	if err := models.ValidateIMEI(imei); err != nil {
		return &ports.CheckTacResult{
			Status: "error",
			IMEI:   imei,
			Color:  "unknown",
		}, fmt.Errorf("IMEI validation failed: %w", err)
	}

	// Convert domain model to legacy model
	legacyStatus := legacyModels.SystemStatus{
		OverloadLevel: status.OverloadLevel,
		TPSOverload:   status.TPSOverload,
	}

	// Use pkg/logic for TAC checking
	result, tacInfo := logic.CheckTac(imei, legacyStatus)

	var tacInfoPtr *ports.TacInfo
	if result.Status == "ok" {
		tacInfoPtr = &ports.TacInfo{
			KeyTac:        tacInfo.KeyTac,
			StartRangeTac: tacInfo.StartRangeTac,
			EndRangeTac:   tacInfo.EndRangeTac,
			Color:         tacInfo.Color,
			PrevLink:      tacInfo.PrevLink,
		}
	}

	return &ports.CheckTacResult{
		Status:  result.Status,
		IMEI:    result.IMEI,
		Color:   result.Color,
		TacInfo: tacInfoPtr,
	}, nil
}

// InsertImei provisions equipment using pkg/logic
func (s *eirService) InsertImei(ctx context.Context, imei string, color string, status models.SystemStatus) (*ports.InsertImeiResult, error) {
	// Convert domain model to legacy model
	legacyStatus := legacyModels.SystemStatus{
		OverloadLevel: status.OverloadLevel,
		TPSOverload:   status.TPSOverload,
	}

	// Use pkg/logic for IMEI insertion with the imeiRepo
	result := logic.InsertImei(s.imeiRepo, imei, color, legacyStatus)

	errorPtr := (*string)(nil)
	if result.Error != "" {
		errorPtr = &result.Error
	}

	return &ports.InsertImeiResult{
		Status: result.Status,
		IMEI:   result.IMEI,
		Error:  errorPtr,
	}, nil
}

// InsertTac provisions equipment using pkg/logic
func (s *eirService) InsertTac(ctx context.Context, tacInfo *ports.TacInfo) (*ports.InsertTacResult, error) {
	if tacInfo == nil {
		return &ports.InsertTacResult{
			Status: "error",
			Error:  strPtr("invalid_parameter"),
		}, fmt.Errorf("tacInfo is required")
	}

	// Convert domain TAC info to legacy model
	legacyTacInfo := legacyModels.TacInfo{
		KeyTac:        tacInfo.KeyTac,
		StartRangeTac: tacInfo.StartRangeTac,
		EndRangeTac:   tacInfo.EndRangeTac,
		Color:         tacInfo.Color,
		PrevLink:      tacInfo.PrevLink,
	}

	// Use pkg/logic for TAC insertion with the imeiRepo
	result := logic.InsertTac(s.imeiRepo, legacyTacInfo)

	var resultTacInfo *ports.TacInfo
	if result.TacInfo.KeyTac != "" {
		resultTacInfo = &ports.TacInfo{
			KeyTac:        result.TacInfo.KeyTac,
			StartRangeTac: result.TacInfo.StartRangeTac,
			EndRangeTac:   result.TacInfo.EndRangeTac,
			Color:         result.TacInfo.Color,
			PrevLink:      result.TacInfo.PrevLink,
		}
	}

	errorPtr := (*string)(nil)
	if result.Error != "" {
		errorPtr = &result.Error
	}

	return &ports.InsertTacResult{
		Status:  result.Status,
		Error:   errorPtr,
		TacInfo: resultTacInfo,
	}, nil
}

// Helper function
func strPtr(s string) *string {
	return &s
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

	// Update cache asynchronously
	if s.cache != nil {
		go func() {
			_ = s.cache.Set(ctx, imei, equipment, 300) // 5 minutes TTL
		}()
	}

	return equipment, nil
}

// ListEquipment retrieves paginated equipment list
func (s *eirService) ListEquipment(ctx context.Context, offset, limit int) ([]*models.Equipment, error) {
	return s.imeiRepo.List(ctx, offset, limit)
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
