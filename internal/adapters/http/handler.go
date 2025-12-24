package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/domain/service"
	"github.com/hsdfat8/eir/internal/logger"
)

// Handler handles HTTP requests for the EIR service
type Handler struct {
	eirService ports.EIRService
}

// NewHandler creates a new HTTP handler
func NewHandler(eirService ports.EIRService) *Handler {
	return &Handler{
		eirService: eirService,
	}
}

// GetEquipmentStatus handles GET /equipment-status (5G N5g-eir API)
// @Summary Retrieves the status of the UE
// @Param pei query string true "PEI of the UE (IMEI)"
// @Param supi query string false "SUPI of the UE"
// @Param gpsi query string false "GPSI of the UE"
// @Success 200 {object} EirResponseData
// @Failure 400 {object} ProblemDetails
// @Failure 404 {object} ProblemDetails
// @Router /equipment-status [get]
func (h *Handler) GetEquipmentStatus(c *gin.Context) {
	pei := c.Query("pei")
	logger.Log.Infow("HTTP GetEquipmentStatus request", "pei", pei, "client_ip", c.ClientIP())

	if pei == "" {
		logger.Log.Warnw("HTTP GetEquipmentStatus missing pei parameter", "client_ip", c.ClientIP())
		c.JSON(http.StatusBadRequest, ProblemDetails{
			Type:   "about:blank",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: "Required parameter 'pei' is missing",
		})
		return
	}

	// Build system status (default: normal operation)
	systemStatus := models.SystemStatus{
		OverloadLevel: 0,
		TPSOverload:   false,
	}

	// Perform equipment check using TAC-based logic
	response, err := h.eirService.CheckTac(c.Request.Context(), pei, systemStatus)
	if err != nil {
		if errors.Is(err, models.ErrInvalidIMEI) {
			logger.Log.Warnw("HTTP GetEquipmentStatus invalid PEI", "pei", pei, "error", err)
			c.JSON(http.StatusBadRequest, ProblemDetails{
				Type:   "about:blank",
				Title:  "Invalid PEI",
				Status: http.StatusBadRequest,
				Detail: err.Error(),
			})
			return
		}

		logger.Log.Errorw("HTTP GetEquipmentStatus failed", "pei", pei, "error", err)
		c.JSON(http.StatusInternalServerError, ProblemDetails{
			Type:   "about:blank",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: "Failed to check equipment status",
		})
		return
	}

	// Convert color to equipment status
	equipmentStatus := convertColorToEquipmentStatus(response.Color)

	logger.Log.Infow("HTTP GetEquipmentStatus response", "pei", pei, "status", equipmentStatus, "color", response.Color)
	// Return response
	c.JSON(http.StatusOK, EirResponseData{
		Status: equipmentStatus,
	})
}

// ProvisionEquipment handles POST /equipment (provisioning API - not part of 3GPP spec)
func (h *Handler) ProvisionEquipment(c *gin.Context) {
	var req ProvisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ProblemDetails{
			Type:   "about:blank",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: err.Error(),
		})
		return
	}

	// Convert equipment status to color code
	color := convertEquipmentStatusToColor(req.Status)

	// Build system status (default: normal operation)
	systemStatus := models.SystemStatus{
		OverloadLevel: 0,
		TPSOverload:   false,
	}

	// Provision equipment using IMEI logic
	result, err := h.eirService.InsertImei(c.Request.Context(), req.IMEI, color, systemStatus)
	if err != nil || result.Status != "ok" {
		detail := "Failed to provision equipment"
		if result.Error != nil {
			detail = *result.Error
		}
		c.JSON(http.StatusInternalServerError, ProblemDetails{
			Type:   "about:blank",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: detail,
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Equipment provisioned successfully"})
}

// GetEquipment handles GET /equipment/:imei
func (h *Handler) GetEquipment(c *gin.Context) {
	imei := c.Param("imei")

	equipment, err := h.eirService.GetEquipment(c.Request.Context(), imei)
	if err != nil {
		if errors.Is(err, service.ErrEquipmentNotFound) {
			c.JSON(http.StatusNotFound, ProblemDetails{
				Type:   "about:blank",
				Title:  "Not Found",
				Status: http.StatusNotFound,
				Detail: "Equipment not found",
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ProblemDetails{
			Type:   "about:blank",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: "Failed to retrieve equipment",
		})
		return
	}

	// Convert to response
	response := EquipmentResponse{
		IMEI:             equipment.IMEI,
		IMEISV:           equipment.IMEISV,
		Status:           equipment.Status,
		Reason:           equipment.Reason,
		LastUpdated:      equipment.LastUpdated.Format("2006-01-02T15:04:05Z07:00"),
		CheckCount:       equipment.CheckCount,
		ManufacturerTAC:  equipment.ManufacturerTAC,
		ManufacturerName: equipment.ManufacturerName,
	}

	if equipment.LastCheckTime != nil {
		lastCheckTime := equipment.LastCheckTime.Format("2006-01-02T15:04:05Z07:00")
		response.LastCheckTime = &lastCheckTime
	}

	c.JSON(http.StatusOK, response)
}

// DeleteEquipment handles DELETE /equipment/:imei
func (h *Handler) DeleteEquipment(c *gin.Context) {
	imei := c.Param("imei")

	if err := h.eirService.RemoveEquipment(c.Request.Context(), imei); err != nil {
		c.JSON(http.StatusInternalServerError, ProblemDetails{
			Type:   "about:blank",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ListEquipment handles GET /equipment
func (h *Handler) ListEquipment(c *gin.Context) {
	offset := 0
	limit := 100

	if offsetParam := c.Query("offset"); offsetParam != "" {
		if _, err := c.GetQuery("offset"); err {
			offset = c.GetInt("offset")
		}
	}

	if limitParam := c.Query("limit"); limitParam != "" {
		if _, err := c.GetQuery("limit"); err {
			limit = c.GetInt("limit")
		}
	}

	equipments, err := h.eirService.ListEquipment(c.Request.Context(), offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ProblemDetails{
			Type:   "about:blank",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: "Failed to list equipment",
		})
		return
	}

	// Convert to response
	var responses []EquipmentResponse
	for _, equipment := range equipments {
		response := EquipmentResponse{
			IMEI:             equipment.IMEI,
			IMEISV:           equipment.IMEISV,
			Status:           equipment.Status,
			Reason:           equipment.Reason,
			LastUpdated:      equipment.LastUpdated.Format("2006-01-02T15:04:05Z07:00"),
			CheckCount:       equipment.CheckCount,
			ManufacturerTAC:  equipment.ManufacturerTAC,
			ManufacturerName: equipment.ManufacturerName,
		}

		if equipment.LastCheckTime != nil {
			lastCheckTime := equipment.LastCheckTime.Format("2006-01-02T15:04:05Z07:00")
			response.LastCheckTime = &lastCheckTime
		}

		responses = append(responses, response)
	}

	c.JSON(http.StatusOK, responses)
}

func (h *Handler) GetCheckImei(c *gin.Context) {
	imei := c.Param("imei")
	logger.Log.Infow("HTTP GetCheckImei request", "imei", imei, "client_ip", c.ClientIP())

	if imei == "" {
		logger.Log.Warnw("HTTP GetCheckImei missing imei parameter", "client_ip", c.ClientIP())
		c.JSON(http.StatusBadRequest, ProblemDetails{
			Type:   "about:blank",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: "Required parameter 'imei' is missing",
		})
		return
	}

	// Build system status (default: normal operation)
	systemStatus := models.SystemStatus{
		OverloadLevel: 0,
		TPSOverload:   false,
	}

	// Perform equipment check using TAC-based logic
	response, err := h.eirService.CheckImei(c.Request.Context(), imei, systemStatus)
	if err != nil {
		if errors.Is(err, models.ErrInvalidIMEI) {
			logger.Log.Warnw("HTTP GetCheckImei invalid IMEI", "imei", imei, "error", err)
			c.JSON(http.StatusBadRequest, ProblemDetails{
				Type:   "about:blank",
				Title:  "Invalid IMEI",
				Status: http.StatusBadRequest,
				Detail: err.Error(),
			})
			return
		}

		logger.Log.Errorw("HTTP GetCheckImei failed", "imei", imei, "error", err)
		c.JSON(http.StatusInternalServerError, ProblemDetails{
			Type:   "about:blank",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: "Failed to check equipment status",
		})
		return
	}

	// Convert color to equipment status
	equipmentStatus := convertColorToEquipmentStatus(response.Color)

	logger.Log.Infow("HTTP GetCheckImei response", "imei", imei, "status", equipmentStatus, "color", response.Color)
	// Return response
	c.JSON(http.StatusOK, EirResponseData{
		Status: equipmentStatus,
	})
}

func (h *Handler) GetCheckTac(c *gin.Context) {
	imei := c.Param("imei")
	logger.Log.Infow("HTTP GetCheckTac request", "imei", imei, "client_ip", c.ClientIP())

	if imei == "" {
		logger.Log.Warnw("HTTP GetCheckTac missing imei parameter", "client_ip", c.ClientIP())
		c.JSON(http.StatusBadRequest, ProblemDetails{
			Type:   "about:blank",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: "Required parameter 'imei' is missing",
		})
		return
	}

	// Build system status (default: normal operation)
	systemStatus := models.SystemStatus{
		OverloadLevel: 0,
		TPSOverload:   false,
	}

	// Perform equipment check using TAC-based logic
	response, err := h.eirService.CheckTac(c.Request.Context(), imei, systemStatus)
	if err != nil {
		if errors.Is(err, models.ErrInvalidIMEI) {
			logger.Log.Warnw("HTTP GetCheckTac invalid IMEI", "imei", imei, "error", err)
			c.JSON(http.StatusBadRequest, ProblemDetails{
				Type:   "about:blank",
				Title:  "Invalid IMEI",
				Status: http.StatusBadRequest,
				Detail: err.Error(),
			})
			return
		}

		logger.Log.Errorw("HTTP GetCheckTac failed", "imei", imei, "error", err)
		c.JSON(http.StatusInternalServerError, ProblemDetails{
			Type:   "about:blank",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: "Failed to check equipment status",
		})
		return
	}

	// Convert color to equipment status
	equipmentStatus := convertColorToEquipmentStatus(response.Color)

	logger.Log.Infow("HTTP GetCheckTac response", "imei", imei, "status", equipmentStatus, "color", response.Color)
	// Return response
	c.JSON(http.StatusOK, EirResponseData{
		Status: equipmentStatus,
	})
}

func (h *Handler) PostInsertTac(c *gin.Context) {
	logger.Log.Infow("HTTP PostInsertTac request", "client_ip", c.ClientIP())
	var tacInfo ports.TacInfo

	if err := c.ShouldBindJSON(&tacInfo); err != nil {
		logger.Log.Warnw("HTTP PostInsertTac invalid request body", "error", err, "client_ip", c.ClientIP())
		c.JSON(http.StatusBadRequest, ProblemDetails{
			Type:   "about:blank",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: "Invalid request body",
		})
		return
	}

	logger.Log.Infow("HTTP PostInsertTac parsed request", "start_range", tacInfo.StartRangeTac, "end_range", tacInfo.EndRangeTac, "color", tacInfo.Color)

	// Perform equipment check using TAC-based logic
	response, err := h.eirService.InsertTac(c.Request.Context(), &tacInfo)
	if err != nil {
		if errors.Is(err, models.ErrInvalidIMEI) {
			logger.Log.Warnw("HTTP PostInsertTac invalid TAC info", "start_range", tacInfo.StartRangeTac, "error", err)
			c.JSON(http.StatusBadRequest, ProblemDetails{
				Type:   "about:blank",
				Title:  "Invalid IMEI",
				Status: http.StatusBadRequest,
				Detail: err.Error(),
			})
			return
		}

		logger.Log.Errorw("HTTP PostInsertTac failed", "start_range", tacInfo.StartRangeTac, "error", err)
		c.JSON(http.StatusInternalServerError, ProblemDetails{
			Type:   "about:blank",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: "Failed to check equipment status",
		})
		return
	}

	// Convert color to equipment status
	var equipmentStatus models.EquipmentStatus
	if response.TacInfo != nil {
		equipmentStatus = convertColorToEquipmentStatus(response.TacInfo.Color)
	} else {
		equipmentStatus = convertColorToEquipmentStatus(tacInfo.Color)
	}

	logger.Log.Infow("HTTP PostInsertTac response", "start_range", tacInfo.StartRangeTac, "status", response.Status, "equipment_status", equipmentStatus)
	// Return response
	if response.Status == "error" {
		c.JSON(http.StatusBadRequest, EirResponseData{
			Status: equipmentStatus,
		})
	} else {
		c.JSON(http.StatusCreated, EirResponseData{
			Status: equipmentStatus,
		})
	}
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "eir",
	})
}

// Helper function to convert string to pointer
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// convertColorToEquipmentStatus converts pkg/logic color codes to EquipmentStatus
func convertColorToEquipmentStatus(color string) models.EquipmentStatus {
	switch color {
	case "black", "b":
		return models.EquipmentStatusBlacklisted
	case "grey", "g":
		return models.EquipmentStatusGreylisted
	case "white", "w":
		return models.EquipmentStatusWhitelisted
	default:
		// Default to whitelisted for unknown
		return models.EquipmentStatusWhitelisted
	}
}

// convertEquipmentStatusToColor converts EquipmentStatus to pkg/logic color codes
func convertEquipmentStatusToColor(status models.EquipmentStatus) string {
	switch status {
	case models.EquipmentStatusBlacklisted:
		return "b"
	case models.EquipmentStatusGreylisted:
		return "g"
	case models.EquipmentStatusWhitelisted:
		return "w"
	default:
		return "w" // Default to white
	}
}
