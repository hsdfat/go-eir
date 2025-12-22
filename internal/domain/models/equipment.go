package models

import (
	"errors"
	"regexp"
	"time"
)

// EquipmentStatus represents the status of a mobile equipment
type EquipmentStatus string

const (
	EquipmentStatusWhitelisted EquipmentStatus = "WHITELISTED" // Permitted
	EquipmentStatusBlacklisted EquipmentStatus = "BLACKLISTED" // Prohibited (stolen, fraudulent)
	EquipmentStatusGreylisted  EquipmentStatus = "GREYLISTED"  // Under observation/tracking
)

// DiameterEquipmentStatus represents Diameter AVP values for Equipment-Status (AVP 1445)
type DiameterEquipmentStatus int32

const (
	DiameterEquipmentStatusWhitelisted DiameterEquipmentStatus = 0 // WHITELISTED
	DiameterEquipmentStatusBlacklisted DiameterEquipmentStatus = 1 // BLACKLISTED
	DiameterEquipmentStatusGreylisted  DiameterEquipmentStatus = 2 // GREYLISTED
)

// Equipment represents a mobile equipment entity
type Equipment struct {
	ID               int64           `json:"id" db:"id"`
	IMEI             string          `json:"imei" db:"imei"`
	IMEISV           *string         `json:"imeisv,omitempty" db:"imeisv"`
	Status           EquipmentStatus `json:"status" db:"status"`
	Reason           *string         `json:"reason,omitempty" db:"reason"`
	LastUpdated      time.Time       `json:"last_updated" db:"last_updated"`
	LastCheckTime    *time.Time      `json:"last_check_time,omitempty" db:"last_check_time"`
	CheckCount       int64           `json:"check_count" db:"check_count"`
	AddedBy          string          `json:"added_by" db:"added_by"`
	Metadata         *string         `json:"metadata,omitempty" db:"metadata"`
	ManufacturerTAC  *string         `json:"manufacturer_tac,omitempty" db:"manufacturer_tac"`
	ManufacturerName *string         `json:"manufacturer_name,omitempty" db:"manufacturer_name"`
}

// AuditLog represents an audit entry for equipment check operations
type AuditLog struct {
	ID            int64           `json:"id" db:"id"`
	IMEI          string          `json:"imei" db:"imei"`
	IMEISV        *string         `json:"imeisv,omitempty" db:"imeisv"`
	Status        EquipmentStatus `json:"status" db:"status"`
	CheckTime     time.Time       `json:"check_time" db:"check_time"`
	OriginHost    *string         `json:"origin_host,omitempty" db:"origin_host"`
	OriginRealm   *string         `json:"origin_realm,omitempty" db:"origin_realm"`
	UserName      *string         `json:"user_name,omitempty" db:"user_name"`
	SUPI          *string         `json:"supi,omitempty" db:"supi"`
	GPSI          *string         `json:"gpsi,omitempty" db:"gpsi"`
	RequestSource string          `json:"request_source" db:"request_source"` // "DIAMETER_S13", "HTTP_5G", etc.
	SessionID     *string         `json:"session_id,omitempty" db:"session_id"`
	ResultCode    *int32          `json:"result_code,omitempty" db:"result_code"`
}

// IMEI validation constants
const (
	IMEILength   = 15
	IMEISVLength = 16
)

var (
	ErrInvalidIMEI       = errors.New("invalid IMEI format")
	ErrInvalidIMEISV     = errors.New("invalid IMEISV format")
	ErrIMEITooShort      = errors.New("IMEI too short")
	ErrIMEITooLong       = errors.New("IMEI too long")
	ErrIMEINotNumeric    = errors.New("IMEI must contain only digits")
	ErrInvalidLuhnCheck  = errors.New("IMEI failed Luhn check")
	ErrInvalidStatus     = errors.New("invalid equipment status")
)

var imeiRegex = regexp.MustCompile(`^\d{14,16}$`)

// ValidateIMEI validates IMEI format according to 3GPP TS 23.003
func ValidateIMEI(imei string) error {
	if imei == "" {
		return ErrInvalidIMEI
	}

	// Check length
	length := len(imei)
	if length < 14 {
		return ErrIMEITooShort
	}
	if length > 16 {
		return ErrIMEITooLong
	}

	// Check if all digits
	if !imeiRegex.MatchString(imei) {
		return ErrIMEINotNumeric
	}

	// For 15-digit IMEI, validate Luhn check digit
	if length == IMEILength {
		if err := validateLuhn(imei); err != nil {
			return err
		}
	}

	return nil
}

// validateLuhn validates the Luhn algorithm (mod 10) check digit
func validateLuhn(imei string) error {
	sum := 0
	alternate := false

	// Process digits from right to left
	for i := len(imei) - 1; i >= 0; i-- {
		digit := int(imei[i] - '0')

		if alternate {
			digit *= 2
			if digit > 9 {
				digit = digit - 9
			}
		}

		sum += digit
		alternate = !alternate
	}

	if sum%10 != 0 {
		return ErrInvalidLuhnCheck
	}

	return nil
}

// ExtractTAC extracts the Type Allocation Code (first 8 digits) from IMEI
func ExtractTAC(imei string) string {
	if len(imei) >= 8 {
		return imei[:8]
	}
	return ""
}

// ValidateStatus checks if the status is valid
func ValidateStatus(status EquipmentStatus) error {
	switch status {
	case EquipmentStatusWhitelisted, EquipmentStatusBlacklisted, EquipmentStatusGreylisted:
		return nil
	default:
		return ErrInvalidStatus
	}
}

// ToDialDialStatus converts EquipmentStatus to Diameter Enumerated value
func ToDialDialStatus(status EquipmentStatus) DiameterEquipmentStatus {
	switch status {
	case EquipmentStatusWhitelisted:
		return DiameterEquipmentStatusWhitelisted
	case EquipmentStatusBlacklisted:
		return DiameterEquipmentStatusBlacklisted
	case EquipmentStatusGreylisted:
		return DiameterEquipmentStatusGreylisted
	default:
		return DiameterEquipmentStatusWhitelisted
	}
}

// FromDiameterStatus converts Diameter Enumerated value to EquipmentStatus
func FromDiameterStatus(diamStatus DiameterEquipmentStatus) EquipmentStatus {
	switch diamStatus {
	case DiameterEquipmentStatusBlacklisted:
		return EquipmentStatusBlacklisted
	case DiameterEquipmentStatusGreylisted:
		return EquipmentStatusGreylisted
	default:
		return EquipmentStatusWhitelisted
	}
}

// SystemStatus represents the operational status of the EIR system
// Used for overload control and rate limiting
type SystemStatus struct {
	OverloadLevel int  // Current overload level (0 = normal)
	TPSOverload   bool // Transaction Per Second overload flag
}
