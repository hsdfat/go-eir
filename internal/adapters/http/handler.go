package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hsdfat8/eir/internal/domain/models"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/domain/service"
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
	if pei == "" {
		c.JSON(http.StatusBadRequest, ProblemDetails{
			Type:   "about:blank",
			Title:  "Bad Request",
			Status: http.StatusBadRequest,
			Detail: "Required parameter 'pei' is missing",
		})
		return
	}

	supi := c.Query("supi")
	gpsi := c.Query("gpsi")

	// Build request
	checkRequest := &ports.CheckEquipmentRequest{
		IMEI:          pei,
		SUPI:          stringPtr(supi),
		GPSI:          stringPtr(gpsi),
		RequestSource: "HTTP_5G",
	}

	// Perform equipment check
	response, err := h.eirService.CheckEquipment(c.Request.Context(), checkRequest)
	if err != nil {
		if errors.Is(err, models.ErrInvalidIMEI) {
			c.JSON(http.StatusBadRequest, ProblemDetails{
				Type:   "about:blank",
				Title:  "Invalid PEI",
				Status: http.StatusBadRequest,
				Detail: err.Error(),
			})
			return
		}

		c.JSON(http.StatusInternalServerError, ProblemDetails{
			Type:   "about:blank",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: "Failed to check equipment status",
		})
		return
	}

	// Return response
	c.JSON(http.StatusOK, EirResponseData{
		Status: response.Status,
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

	// Build provisioning request
	provisionReq := &ports.ProvisionEquipmentRequest{
		IMEI:             req.IMEI,
		IMEISV:           req.IMEISV,
		Status:           req.Status,
		Reason:           req.Reason,
		AddedBy:          "http_api", // TODO: Extract from auth context
		Metadata:         req.Metadata,
		ManufacturerTAC:  req.ManufacturerTAC,
		ManufacturerName: req.ManufacturerName,
	}

	// Provision equipment
	if err := h.eirService.ProvisionEquipment(c.Request.Context(), provisionReq); err != nil {
		c.JSON(http.StatusInternalServerError, ProblemDetails{
			Type:   "about:blank",
			Title:  "Internal Server Error",
			Status: http.StatusInternalServerError,
			Detail: err.Error(),
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

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
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
