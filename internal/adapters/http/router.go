package http

import (
	"github.com/gin-gonic/gin"
	"github.com/hsdfat8/eir/internal/domain/ports"
)

// SetupRouter creates and configures the HTTP router
func SetupRouter(eirService ports.EIRService) *gin.Engine {
	router := gin.Default()

	handler := NewHandler(eirService)

	// 5G N5g-eir API (3GPP TS 29.511)
	v1 := router.Group("/n5g-eir-eic/v1")
	{
		v1.GET("/equipment-status", handler.GetEquipmentStatus)
	}

	// Management API (non-standard, for provisioning)
	api := router.Group("/api/v1")
	{
		api.POST("/equipment", handler.ProvisionEquipment)
		api.GET("/equipment/:imei", handler.GetEquipment)
		api.DELETE("/equipment/:imei", handler.DeleteEquipment)
		api.GET("/equipment", handler.ListEquipment)
	}

	// Health check
	router.GET("/health", handler.HealthCheck)

	return router
}
