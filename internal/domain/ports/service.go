package ports

import (
	"context"

	"github.com/hsdfat8/eir/internal/domain/models"
)

// EIRService defines the core business operations for EIR
// This interface follows the business logic patterns defined in pkg/logic
type EIRService interface {
	// CheckImei performs IMEI check using IMEI-based logic
	// Maps to pkg/logic.CheckImei
	CheckImei(ctx context.Context, imei string, status models.SystemStatus) (*CheckImeiResult, error)

	// CheckTac performs TAC-based equipment check
	// Maps to pkg/logic.CheckTac
	CheckTac(ctx context.Context, imei string, status models.SystemStatus) (*CheckTacResult, error)

	// InsertImei provisions equipment using IMEI logic
	// Maps to pkg/logic.InsertImei
	InsertImei(ctx context.Context, imei string, color string, status models.SystemStatus) (*InsertImeiResult, error)

	// InsertTac provisions equipment using TAC range logic
	// Maps to pkg/logic.InsertTac
	InsertTac(ctx context.Context, tacInfo *TacInfo) (*InsertTacResult, error)

	// GetEquipment retrieves equipment information (for management/audit)
	GetEquipment(ctx context.Context, imei string) (*models.Equipment, error)

	// ListEquipment retrieves paginated equipment list (for management/audit)
	ListEquipment(ctx context.Context, offset, limit int) ([]*models.Equipment, error)

	// RemoveEquipment removes equipment from the database (for management)
	RemoveEquipment(ctx context.Context, imei string) error
}

// CheckImeiResult represents the result of IMEI check
type CheckImeiResult struct {
	Status string  // "ok" or "error"
	IMEI   string  // The checked IMEI
	Color  string  // "b" (black), "g" (grey), "w" (white), "unknown", "overload"
}

// CheckTacResult represents the result of TAC-based check
type CheckTacResult struct {
	Status  string   // "ok" or "error"
	IMEI    string   // The checked IMEI
	Color   string   // "black", "grey", "white", "unknown"
	TacInfo *TacInfo // TAC information if found
}

// InsertImeiResult represents the result of IMEI insertion
type InsertImeiResult struct {
	Status string  // "ok" or "error"
	IMEI   string  // The inserted IMEI
	Error  *string // Error code: "overload", "invalid_parameter", "invalid_value", "invalid_length", "invalid_color", "color_conflict", "imei_exist"
}

// InsertTacResult represents the result of TAC insertion
type InsertTacResult struct {
	Status  string   // "ok" or "error"
	Error   *string  // Error code: "invalid_length", "invalid_color", "invalid_value", "range_exist"
	TacInfo *TacInfo // The TAC info that was processed
}

// TacInfo represents TAC range information
type TacInfo struct {
	KeyTac        string  // Computed key for storage
	StartRangeTac string  // Start of TAC range
	EndRangeTac   string  // End of TAC range
	Color         string  // "black", "grey", "white"
	PrevLink      *string // Link to previous range (for optimization)
}
