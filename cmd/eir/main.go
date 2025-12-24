package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hsdfat8/eir/internal/adapters/diameter"
	httpAdapter "github.com/hsdfat8/eir/internal/adapters/http"
	"github.com/hsdfat8/eir/internal/adapters/memory"
	"github.com/hsdfat8/eir/internal/config"
	"github.com/hsdfat8/eir/internal/domain/service"
	"github.com/hsdfat8/eir/internal/logger"
)

func main() {
	// Initialize logger
	logger := logger.New("eir-main", "info")

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		logger.Fatalw("Failed to load configuration", "error", err)
	}

	// Initialize persistence repositories
	imeiRepo := memory.NewInMemoryIMEIRepository()
	auditRepo := memory.NewInMemoryAuditRepository()

	logger.Info("✓ Repositories initialized")

	// Initialize cache (optional, nil if disabled)
	// TODO: Implement Redis cache adapter if cfg.Cache.Enabled

	// Initialize EIR service with persistence repositories
	eirService := service.NewEIRService(imeiRepo, auditRepo, nil)

	logger.Info("✓ EIR service initialized")

	// Initialize HTTP/2 server
	httpServerConfig := httpAdapter.ServerConfig{
		ListenAddr:   fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
		EnableH2C:    true, // Enable HTTP/2 Cleartext for testing
		// For production with TLS:
		// EnableTLS:   true,
		// TLSCertFile: cfg.Server.TLSCertFile,
		// TLSKeyFile:  cfg.Server.TLSKeyFile,
	}

	httpServer := httpAdapter.NewServer(httpServerConfig, eirService)

	// Start HTTP/2 server
	if err := httpServer.Start(); err != nil {
		logger.Fatalw("Failed to start HTTP server", "error", err)
	}

	logger.Infow("✓ HTTP/2 server listening", "address", httpServer.GetAddr())

	// Initialize Diameter S13 server
	diameterConfig := diameter.ServerConfig{
		ListenAddr:  cfg.Diameter.ListenAddr,
		OriginHost:  cfg.Diameter.OriginHost,
		OriginRealm: cfg.Diameter.OriginRealm,
		ProductName: cfg.Diameter.ProductName,
		VendorID:    cfg.Diameter.VendorID,
	}

	diameterServer := diameter.NewServer(diameterConfig, eirService)

	// Start Diameter server
	if err := diameterServer.Start(); err != nil {
		logger.Fatalw("Failed to start Diameter server", "error", err)
	}

	logger.Info("✓ Diameter S13 server started")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	// Shutdown HTTP/2 server
	if err := httpServer.Stop(); err != nil {
		logger.Errorw("HTTP server shutdown error", "error", err)
	}

	// Shutdown Diameter server
	if err := diameterServer.Stop(); err != nil {
		logger.Errorw("Diameter server shutdown error", "error", err)
	}

	logger.Info("Servers stopped gracefully")
}
