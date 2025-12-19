package ports

import (
	"context"

	"github.com/hsdfat8/eir/internal/domain/models"
)

// EIRService defines the core business operations for EIR
// This is the primary port for the EIR domain
type EIRService interface {
	// CheckEquipment performs equipment identity check and returns status
	CheckEquipment(ctx context.Context, request *CheckEquipmentRequest) (*CheckEquipmentResponse, error)

	// ProvisionEquipment adds or updates equipment in the database
	ProvisionEquipment(ctx context.Context, request *ProvisionEquipmentRequest) error

	// RemoveEquipment removes equipment from the database
	RemoveEquipment(ctx context.Context, imei string) error

	// GetEquipment retrieves equipment information
	GetEquipment(ctx context.Context, imei string) (*models.Equipment, error)

	// ListEquipment retrieves paginated equipment list
	ListEquipment(ctx context.Context, offset, limit int) ([]*models.Equipment, error)
}

// CheckEquipmentRequest represents an equipment check request
type CheckEquipmentRequest struct {
	IMEI          string  // Required: IMEI or IMEISV
	IMEISV        *string // Optional: IMEISV if separate from IMEI
	SUPI          *string // Optional: Subscriber Permanent Identifier (5G)
	GPSI          *string // Optional: Generic Public Subscription Identifier (5G)
	UserName      *string // Optional: User identifier (4G)
	OriginHost    *string // Optional: Request origin (for audit)
	OriginRealm   *string // Optional: Request origin realm (for audit)
	SessionID     *string // Optional: Session identifier (for audit)
	RequestSource string  // Required: "DIAMETER_S13", "HTTP_5G", etc.
}

// CheckEquipmentResponse represents the result of an equipment check
type CheckEquipmentResponse struct {
	IMEI            string                  // The checked IMEI
	Status          models.EquipmentStatus  // Equipment status
	Reason          *string                 // Optional reason for the status
	ManufacturerTAC *string                 // Optional Type Allocation Code
}

// ProvisionEquipmentRequest represents a provisioning request
type ProvisionEquipmentRequest struct {
	IMEI             string                 // Required: IMEI
	IMEISV           *string                // Optional: IMEISV
	Status           models.EquipmentStatus // Required: Equipment status
	Reason           *string                // Optional: Reason for status
	AddedBy          string                 // Required: Who provisioned this
	Metadata         *string                // Optional: Additional metadata (JSON)
	ManufacturerTAC  *string                // Optional: Type Allocation Code
	ManufacturerName *string                // Optional: Manufacturer name
}
