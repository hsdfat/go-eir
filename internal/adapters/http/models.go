package http

import "github.com/hsdfat8/eir/internal/domain/models"

// EirResponseData represents the response for equipment status query (5G N5g-eir API)
type EirResponseData struct {
	Status models.EquipmentStatus `json:"status"`
}

// ProblemDetails represents an error response following RFC 7807
type ProblemDetails struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// ProvisionRequest represents equipment provisioning request
type ProvisionRequest struct {
	IMEI             string                  `json:"imei" binding:"required"`
	IMEISV           *string                 `json:"imeisv,omitempty"`
	Status           models.EquipmentStatus  `json:"status" binding:"required"`
	Reason           *string                 `json:"reason,omitempty"`
	Metadata         *string                 `json:"metadata,omitempty"`
	ManufacturerTAC  *string                 `json:"manufacturer_tac,omitempty"`
	ManufacturerName *string                 `json:"manufacturer_name,omitempty"`
}

// EquipmentResponse represents equipment information response
type EquipmentResponse struct {
	IMEI             string                  `json:"imei"`
	IMEISV           *string                 `json:"imeisv,omitempty"`
	Status           models.EquipmentStatus  `json:"status"`
	Reason           *string                 `json:"reason,omitempty"`
	LastUpdated      string                  `json:"last_updated"`
	LastCheckTime    *string                 `json:"last_check_time,omitempty"`
	CheckCount       int64                   `json:"check_count"`
	ManufacturerTAC  *string                 `json:"manufacturer_tac,omitempty"`
	ManufacturerName *string                 `json:"manufacturer_name,omitempty"`
}
