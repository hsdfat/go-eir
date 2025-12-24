package http

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
)

// ginLogger returns a gin.HandlerFunc (middleware) that logs requests using our observability logger
func ginLogger() gin.HandlerFunc {
	logger := logger.New("gin-http", "info")

	return func(c *gin.Context) {
		// Start timer
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Get status code and other details
		statusCode := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()
		errorMessage := c.Errors.ByType(gin.ErrorTypePrivate).String()

		// Build log fields
		fields := []interface{}{
			"status", statusCode,
			"method", method,
			"path", path,
			"ip", clientIP,
			"latency_ms", latency.Milliseconds(),
		}

		if query != "" {
			fields = append(fields, "query", query)
		}

		if errorMessage != "" {
			fields = append(fields, "error", errorMessage)
		}

		// Log based on status code
		if statusCode >= 500 {
			logger.Errorw("HTTP request error", fields...)
		} else if statusCode >= 400 {
			logger.Warnw("HTTP request warning", fields...)
		} else {
			logger.Infow("HTTP request", fields...)
		}
	}
}

// ginRecovery returns a gin.HandlerFunc (middleware) that recovers from panics and logs using our observability logger
func ginRecovery() gin.HandlerFunc {
	logger := logger.New("gin-recovery", "info")

	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get the full stack trace
				stack := debug.Stack()

				// Log the panic with full stack trace
				logger.Errorw("Panic recovered",
					"error", err,
					"path", c.Request.URL.Path,
					"method", c.Request.Method,
					"ip", c.ClientIP(),
					"stack", string(stack),
				)

				// Also print to stderr for immediate visibility
				fmt.Printf("\n=== PANIC RECOVERED ===\n")
				fmt.Printf("Error: %v\n", err)
				fmt.Printf("Path: %s\n", c.Request.URL.Path)
				fmt.Printf("Method: %s\n", c.Request.Method)
				fmt.Printf("Client IP: %s\n", c.ClientIP())
				fmt.Printf("\nStack Trace:\n%s\n", string(stack))
				fmt.Printf("======================\n\n")

				// Abort with 500 status
				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}

// SetupRouter creates and configures the HTTP router
func SetupRouter(eirService ports.EIRService) *gin.Engine {
	// Set Gin to release mode to disable debug logging
	gin.SetMode(gin.ReleaseMode)

	// Create router without default middleware
	router := gin.New()

	// Add custom recovery middleware (must be first)
	router.Use(ginRecovery())

	// Add custom logger middleware
	router.Use(ginLogger())

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
		api.GET("/check-imei/:imei", handler.GetCheckImei)
		api.GET("/check-tac/:imei", handler.GetCheckTac)
		api.POST("/insert-tac", handler.PostInsertTac)
	}

	// Health check
	router.GET("/health", handler.HealthCheck)

	return router
}
