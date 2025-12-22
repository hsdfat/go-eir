package diameter

import (
	"context"
	"fmt"

	"github.com/hsdfat/diam-gw/commands/s13"
	"github.com/hsdfat/diam-gw/models_base"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
)

const (
	DiameterResultCodeSuccess                 = 2001
	DiameterResultCodeUnableToComply          = 5012
	DiameterResultCodeInvalidAVPValue         = 5004
	DiameterAuthSessionStateNoStateMaintained = 1
)

// S13Handler handles Diameter S13 interface messages
type S13Handler struct {
	eirService  ports.EIRService
	originHost  string
	originRealm string
}

// NewS13Handler creates a new Diameter S13 handler
func NewS13Handler(eirService ports.EIRService, originHost, originRealm string) *S13Handler {
	return &S13Handler{
		eirService:  eirService,
		originHost:  originHost,
		originRealm: originRealm,
	}
}

// HandleMEIdentityCheckRequest processes ME-Identity-Check-Request and returns ME-Identity-Check-Answer
func (h *S13Handler) HandleMEIdentityCheckRequest(ctx context.Context, req *s13.MEIdentityCheckRequest) (*s13.MEIdentityCheckAnswer, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return h.buildErrorAnswer(req, DiameterResultCodeInvalidAVPValue), fmt.Errorf("invalid request: %w", err)
	}

	// Extract IMEI from TerminalInformation
	if req.TerminalInformation == nil {
		return h.buildErrorAnswer(req, DiameterResultCodeInvalidAVPValue), fmt.Errorf("terminal information is missing")
	}

	var imei string
	if req.TerminalInformation.Imei != nil {
		imei = string(*req.TerminalInformation.Imei)
	} else {
		return h.buildErrorAnswer(req, DiameterResultCodeInvalidAVPValue), fmt.Errorf("IMEI is missing")
	}

	// Build system status (default: normal operation)
	systemStatus := models.SystemStatus{
		OverloadLevel: 0,
		TPSOverload:   false,
	}

	// Perform equipment check using TAC-based logic
	checkResponse, err := h.eirService.CheckTac(ctx, imei, systemStatus)
	if err != nil {
		return h.buildErrorAnswer(req, DiameterResultCodeUnableToComply), fmt.Errorf("equipment check failed: %w", err)
	}

	// Build successful answer
	return h.buildSuccessAnswerFromTac(req, checkResponse), nil
}

// buildSuccessAnswerFromTac creates a successful ME-Identity-Check-Answer from TAC check result
func (h *S13Handler) buildSuccessAnswerFromTac(req *s13.MEIdentityCheckRequest, checkResponse *ports.CheckTacResult) *s13.MEIdentityCheckAnswer {
	answer := s13.NewMEIdentityCheckAnswer()

	// Copy from request
	answer.SessionId = req.SessionId
	answer.AuthSessionState = models_base.Enumerated(DiameterAuthSessionStateNoStateMaintained)

	// Set origin
	answer.OriginHost = models_base.DiameterIdentity(h.originHost)
	answer.OriginRealm = models_base.DiameterIdentity(h.originRealm)

	// Set result code
	resultCode := models_base.Unsigned32(DiameterResultCodeSuccess)
	answer.ResultCode = &resultCode

	// Convert color to equipment status
	equipmentStatus := convertColorToEquipmentStatus(checkResponse.Color)
	diameterStatus := models_base.Enumerated(models.ToDialDialStatus(equipmentStatus))
	answer.EquipmentStatus = &diameterStatus

	return answer
}

// convertColorToEquipmentStatus converts pkg/logic color codes to EquipmentStatus
func convertColorToEquipmentStatus(color string) models.EquipmentStatus {
	switch color {
	case "black":
		return models.EquipmentStatusBlacklisted
	case "grey":
		return models.EquipmentStatusGreylisted
	case "white":
		return models.EquipmentStatusWhitelisted
	default:
		// Default to whitelisted for unknown
		return models.EquipmentStatusWhitelisted
	}
}

// buildErrorAnswer creates an error ME-Identity-Check-Answer
func (h *S13Handler) buildErrorAnswer(req *s13.MEIdentityCheckRequest, resultCode uint32) *s13.MEIdentityCheckAnswer {
	answer := s13.NewMEIdentityCheckAnswer()

	// Copy from request
	answer.SessionId = req.SessionId
	answer.AuthSessionState = models_base.Enumerated(DiameterAuthSessionStateNoStateMaintained)

	// Set origin
	answer.OriginHost = models_base.DiameterIdentity(h.originHost)
	answer.OriginRealm = models_base.DiameterIdentity(h.originRealm)

	// Set error result code
	rc := models_base.Unsigned32(resultCode)
	answer.ResultCode = &rc

	return answer
}
