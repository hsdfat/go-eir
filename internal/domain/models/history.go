package models

import "time"

// ChangeType represents the type of change made to a record
type ChangeType string

const (
	ChangeTypeCreate ChangeType = "CREATE"
	ChangeTypeUpdate ChangeType = "UPDATE"
	ChangeTypeDelete ChangeType = "DELETE"
	ChangeTypeCheck  ChangeType = "CHECK"
)

// EquipmentHistory represents a historical record of equipment changes
type EquipmentHistory struct {
	ID               int64          `json:"id" bson:"_id,omitempty"`
	IMEI             string         `json:"imei" bson:"imei"`
	ChangeType       ChangeType     `json:"change_type" bson:"change_type"`
	ChangedAt        time.Time      `json:"changed_at" bson:"changed_at"`
	ChangedBy        string         `json:"changed_by" bson:"changed_by"`
	PreviousStatus   *EquipmentStatus `json:"previous_status,omitempty" bson:"previous_status,omitempty"`
	NewStatus        EquipmentStatus  `json:"new_status" bson:"new_status"`
	PreviousReason   *string        `json:"previous_reason,omitempty" bson:"previous_reason,omitempty"`
	NewReason        *string        `json:"new_reason,omitempty" bson:"new_reason,omitempty"`
	ChangeDetails    map[string]interface{} `json:"change_details,omitempty" bson:"change_details,omitempty"`
	SessionID        *string        `json:"session_id,omitempty" bson:"session_id,omitempty"`
}

// AuditLogExtended extends AuditLog with additional tracking fields
type AuditLogExtended struct {
	AuditLog
	IPAddress         *string                `json:"ip_address,omitempty" bson:"ip_address,omitempty"`
	UserAgent         *string                `json:"user_agent,omitempty" bson:"user_agent,omitempty"`
	ChangeHistory     *EquipmentHistory      `json:"change_history,omitempty" bson:"change_history,omitempty"`
	AdditionalData    map[string]interface{} `json:"additional_data,omitempty" bson:"additional_data,omitempty"`
	ProcessingTimeMs  *int64                 `json:"processing_time_ms,omitempty" bson:"processing_time_ms,omitempty"`
}

// EquipmentSnapshot represents a point-in-time snapshot of equipment state
type EquipmentSnapshot struct {
	ID            int64           `json:"id" bson:"_id,omitempty"`
	EquipmentID   int64           `json:"equipment_id" bson:"equipment_id"`
	IMEI          string          `json:"imei" bson:"imei"`
	SnapshotTime  time.Time       `json:"snapshot_time" bson:"snapshot_time"`
	Status        EquipmentStatus `json:"status" bson:"status"`
	Reason        *string         `json:"reason,omitempty" bson:"reason,omitempty"`
	CheckCount    int64           `json:"check_count" bson:"check_count"`
	Metadata      *string         `json:"metadata,omitempty" bson:"metadata,omitempty"`
	CreatedBy     string          `json:"created_by" bson:"created_by"`
	SnapshotType  string          `json:"snapshot_type" bson:"snapshot_type"` // "MANUAL", "SCHEDULED", "PRE_UPDATE"
}
