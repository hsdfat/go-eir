package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	legacyModels "github.com/hsdfat8/eir/models"
	"github.com/hsdfat8/eir/pkg/repository"
	"github.com/hsdfat8/eir/utils"
)

var (
	ErrEquipmentNotFound = errors.New("equipment not found")
	ErrInvalidRequest    = errors.New("invalid request")
)

// eirService implements the EIRService interface
type eirService struct {
	imeiRepo      ports.IMEIRepository
	auditRepo     ports.AuditRepository
	cache         ports.CacheRepository    // Optional
	imeiLogicRepo repository.ImeiRepository // For internal logic
	tacLogicRepo  repository.TacRepository  // For internal logic

	// Internal business logic services
	imeiLogic *ImeiLogicService
	tacLogic  *TacLogicService
}

// NewEIRService creates a new EIR service instance with default sample data
func NewEIRService(
	imeiRepo ports.IMEIRepository,
	auditRepo ports.AuditRepository,
	cache ports.CacheRepository,
) ports.EIRService {
	return NewEIRServiceWithSampleData(imeiRepo, auditRepo, cache, utils.ImeiSampleData, utils.TacSampleData)
}

// NewEIRServiceWithSampleData creates a new EIR service with custom sample data
func NewEIRServiceWithSampleData(
	imeiRepo ports.IMEIRepository,
	auditRepo ports.AuditRepository,
	cache ports.CacheRepository,
	imeiSampleData map[string]*legacyModels.ImeiInfo,
	tacSampleData []legacyModels.TacInfo,
) ports.EIRService {
	// Initialize repositories for logic
	imeiLogicRepo := repository.NewInMemoryImeiRepo()
	tacLogicRepo := repository.NewInMemoryTacRepo()

	// Get configuration values
	imeiCheckLength := utils.GetImeiCheckLength()
	imeiMaxLength := utils.GetImeiMaxLength()
	tacMaxLength := utils.GetTacMaxLength()

	// Initialize logic services with sample data
	imeiLogic := NewImeiLogicService(imeiCheckLength, imeiMaxLength, imeiLogicRepo, imeiSampleData)
	tacLogic := NewTacLogicService(tacMaxLength, tacLogicRepo, tacSampleData)

	return &eirService{
		imeiRepo:      imeiRepo,
		auditRepo:     auditRepo,
		cache:         cache,
		imeiLogicRepo: imeiLogicRepo,
		tacLogicRepo:  tacLogicRepo,
		imeiLogic:     imeiLogic,
		tacLogic:      tacLogic,
	}
}

// CheckImei performs IMEI check using internal IMEI logic
func (s *eirService) CheckImei(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckImeiResult, error) {
	// Use internal IMEI logic service
	result := s.imeiLogic.CheckImei(imei, status)

	return &result, nil
}

// CheckTac performs TAC-based equipment check using internal TAC logic
func (s *eirService) CheckTac(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckTacResult, error) {
	// Validate IMEI format
	if err := models.ValidateIMEI(imei); err != nil {
		return &ports.CheckTacResult{
			Status: "error",
			IMEI:   imei,
			Color:  "unknown",
		}, fmt.Errorf("IMEI validation failed: %w", err)
	}

	// Use internal TAC logic service
	result, tacInfo := s.tacLogic.CheckTac(imei)

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

// InsertImei provisions equipment using internal IMEI logic
func (s *eirService) InsertImei(ctx context.Context, imei string, color string, status models.SystemStatus) (*ports.InsertImeiResult, error) {
	// Use internal IMEI logic service
	result := s.imeiLogic.InsertImei(imei, color, status)

	return &result, nil
}

// InsertTac provisions equipment using internal TAC range logic
func (s *eirService) InsertTac(ctx context.Context, tacInfo *ports.TacInfo) (*ports.InsertTacResult, error) {
	if tacInfo == nil {
		return &ports.InsertTacResult{
			Status: "error",
			Error:  strPtr("invalid_parameter"),
		}, fmt.Errorf("tacInfo is required")
	}

	// Use internal TAC logic service
	result := s.tacLogic.InsertTac(*tacInfo)

	return &result, nil
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

// Helper function
func strPtr(s string) *string {
	return &s
}
