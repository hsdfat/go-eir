package service

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/hsdfat8/eir/internal/adapters/postgres"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
	legacyModels "github.com/hsdfat8/eir/models"
	"github.com/hsdfat8/eir/pkg/logic"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
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
	logger    logger.Logger         // Optional custom logger
}

// NewEIRService creates a new EIR service instance
func NewEIRService(
	imeiRepo ports.IMEIRepository,
	auditRepo ports.AuditRepository,
	cache ports.CacheRepository,
) ports.EIRService {
	_ = godotenv.Load("../../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	db, _ := sqlx.Connect("postgres", dbURL)
	defer db.Close()
	return &eirService{
		imeiRepo:  postgres.NewIMEIRepository(db),
		auditRepo: auditRepo,
		cache:     cache,
		logger:    nil, // Use global logger by default
	}
}

// SetLogger sets a custom logger for this service instance
func (s *eirService) SetLogger(l logger.Logger) {
	s.logger = l
}

// getLogger returns the custom logger if set, otherwise returns the global logger
func (s *eirService) getLogger() logger.Logger {
	if s.logger != nil {
		return s.logger
	}
	return logger.Log
}

// CheckImei performs IMEI check using pkg/logic
func (s *eirService) CheckImei(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckImeiResult, error) {
	s.getLogger().Infow("CheckImei started", "imei", imei, "overload_level", status.OverloadLevel, "tps_overload", status.TPSOverload)

	// Convert domain model to legacy model
	legacyStatus := legacyModels.SystemStatus{
		OverloadLevel: status.OverloadLevel,
		TPSOverload:   status.TPSOverload,
	}

	// Use pkg/logic for IMEI checking
	result := logic.CheckImei(imei, legacyStatus)

	s.getLogger().Infow("CheckImei completed", "imei", imei, "status", result.Status, "color", result.Color)

	return &ports.CheckImeiResult{
		Status: result.Status,
		IMEI:   result.IMEI,
		Color:  result.Color,
	}, nil
}

// CheckTac performs TAC-based equipment check using pkg/logic
func (s *eirService) CheckTac(ctx context.Context, imei string, status models.SystemStatus) (*ports.CheckTacResult, error) {
	s.getLogger().Infow("CheckTac started", "imei", imei, "overload_level", status.OverloadLevel, "tps_overload", status.TPSOverload)

	// Validate IMEI format
	if err := models.ValidateIMEI(imei); err != nil {
		s.getLogger().Errorw("CheckTac IMEI validation failed", "imei", imei, "error", err)
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
		s.getLogger().Infow("CheckTac completed successfully", "imei", imei, "status", result.Status, "color", result.Color, "key_tac", tacInfo.KeyTac)
	} else {
		s.getLogger().Warnw("CheckTac completed with error", "imei", imei, "status", result.Status, "color", result.Color)
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
	s.getLogger().Infow("InsertImei started", "imei", imei, "color", color, "overload_level", status.OverloadLevel, "tps_overload", status.TPSOverload)

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
		s.getLogger().Errorw("InsertImei failed", "imei", imei, "color", color, "status", result.Status, "error", result.Error)
	} else {
		s.getLogger().Infow("InsertImei completed successfully", "imei", imei, "color", color, "status", result.Status)
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
		s.getLogger().Error("InsertTac failed: tacInfo is nil")
		return &ports.InsertTacResult{
			Status: "error",
			Error:  strPtr("invalid_parameter"),
		}, fmt.Errorf("tacInfo is required")
	}

	s.getLogger().Infow("InsertTac started", "start_range", tacInfo.StartRangeTac, "end_range", tacInfo.EndRangeTac, "color", tacInfo.Color)

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
		s.getLogger().Errorw("InsertTac failed", "start_range", tacInfo.StartRangeTac, "end_range", tacInfo.EndRangeTac, "status", result.Status, "error", result.Error)
	} else {
		s.getLogger().Infow("InsertTac completed successfully", "start_range", tacInfo.StartRangeTac, "end_range", tacInfo.EndRangeTac, "status", result.Status, "key_tac", result.TacInfo.KeyTac)
	}

	return &ports.InsertTacResult{
		Status:  result.Status,
		Error:   errorPtr,
		TacInfo: resultTacInfo,
	}, nil
}

func (s *eirService) ClearTacInfo(ctx context.Context) {
	logic.ClearTacInfo(s.imeiRepo)
}

// Helper function
func strPtr(s string) *string {
	return &s
}

// GetEquipment retrieves equipment information
func (s *eirService) GetEquipment(ctx context.Context, imei string) (*models.Equipment, error) {
	s.getLogger().Infow("GetEquipment started", "imei", imei)

	if err := models.ValidateIMEI(imei); err != nil {
		s.getLogger().Errorw("GetEquipment IMEI validation failed", "imei", imei, "error", err)
		return nil, fmt.Errorf("invalid IMEI: %w", err)
	}

	// Try cache first
	if s.cache != nil {
		equipment, err := s.cache.Get(ctx, imei)
		if err == nil && equipment != nil {
			s.getLogger().Debugw("GetEquipment cache hit", "imei", imei)
			return equipment, nil
		}
		s.getLogger().Debugw("GetEquipment cache miss", "imei", imei)
	}

	// Query database
	equipment, err := s.imeiRepo.GetByIMEI(ctx, imei)
	if err != nil {
		s.getLogger().Errorw("GetEquipment not found in database", "imei", imei, "error", err)
		return nil, ErrEquipmentNotFound
	}

	s.getLogger().Infow("GetEquipment completed successfully", "imei", imei, "status", equipment.Status)

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
	s.getLogger().Infow("ListEquipment started", "offset", offset, "limit", limit)

	equipments, err := s.imeiRepo.List(ctx, offset, limit)
	if err != nil {
		s.getLogger().Errorw("ListEquipment failed", "offset", offset, "limit", limit, "error", err)
		return nil, err
	}

	s.getLogger().Infow("ListEquipment completed successfully", "offset", offset, "limit", limit, "count", len(equipments))
	return equipments, nil
}

// RemoveEquipment removes equipment from the system
func (s *eirService) RemoveEquipment(ctx context.Context, imei string) error {
	s.getLogger().Infow("RemoveEquipment started", "imei", imei)

	if err := models.ValidateIMEI(imei); err != nil {
		s.getLogger().Errorw("RemoveEquipment IMEI validation failed", "imei", imei, "error", err)
		return fmt.Errorf("invalid IMEI: %w", err)
	}

	if err := s.imeiRepo.Delete(ctx, imei); err != nil {
		s.getLogger().Errorw("RemoveEquipment failed to delete from database", "imei", imei, "error", err)
		return fmt.Errorf("failed to delete equipment: %w", err)
	}

	// Invalidate cache
	if s.cache != nil {
		_ = s.cache.Delete(ctx, imei)
	}

	s.getLogger().Infow("RemoveEquipment completed successfully", "imei", imei)
	return nil
}
