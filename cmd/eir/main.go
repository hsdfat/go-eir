package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	govclient "github.com/chronnie/governance/client"
	"github.com/chronnie/governance/models"
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
	eirService := service.NewEIRService(cfg, imeiRepo, auditRepo, nil)

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

	// Register with governance manager
	governanceURL := os.Getenv("GOVERNANCE_URL")
	if governanceURL == "" {
		governanceURL = "http://telco-governance:8080"
	}

	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podName, _ = os.Hostname()
	}

	// Create separate clients for HTTP and Diameter service groups
	govClientHTTP := govclient.NewClient(&govclient.ClientConfig{
		ManagerURL:  governanceURL,
		ServiceName: "eir-http",
		PodName:     podName,
	})

	govClientDiameter := govclient.NewClient(&govclient.ClientConfig{
		ManagerURL:  governanceURL,
		ServiceName: "eir-diameter",
		PodName:     podName,
	})

	// Register EIR HTTP service group
	httpRegistration := &models.ServiceRegistration{
		ServiceName: "eir-http",
		PodName:     podName,
		Providers: []models.ProviderInfo{
			{
				Protocol: models.ProtocolHTTP,
				IP:       cfg.Server.Host,
				Port:     cfg.Server.Port,
			},
		},
		HealthCheckURL:  fmt.Sprintf("http://%s:%d/health", cfg.Server.Host, cfg.Server.Port),
		NotificationURL: fmt.Sprintf("http://%s:%d/governance/notify", cfg.Server.Host, cfg.Server.Port),
		Subscriptions:   []string{},
	}

	if err := govClientHTTP.Register(httpRegistration); err != nil {
		logger.Warnw("Failed to register eir-http", "error", err)
	} else {
		logger.Infow("✓ Registered eir-http group", "url", governanceURL)
	}

	// Register EIR Diameter service group
	diameterRegistration := &models.ServiceRegistration{
		ServiceName: "eir-diameter",
		PodName:     podName,
		Providers: []models.ProviderInfo{
			{
				Protocol: models.ProtocolTCP,
				IP:       diameterConfig.ListenAddr[:len(diameterConfig.ListenAddr)-5], // Remove port
				Port:     3868,
			},
		},
		HealthCheckURL:  fmt.Sprintf("http://%s:%d/health", cfg.Server.Host, cfg.Server.Port),
		NotificationURL: fmt.Sprintf("http://%s:%d/governance/notify", cfg.Server.Host, cfg.Server.Port),
		Subscriptions:   []string{},
	}

	if err := govClientDiameter.Register(diameterRegistration); err != nil {
		logger.Warnw("Failed to register eir-diameter", "error", err)
	} else {
		logger.Infow("✓ Registered eir-diameter group", "url", governanceURL)
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down servers...")

	// Unregister from governance
	if err := govClientHTTP.Unregister(); err != nil {
		logger.Warnw("Failed to unregister eir-http", "error", err)
	} else {
		logger.Info("✓ Unregistered eir-http group")
	}

	if err := govClientDiameter.Unregister(); err != nil {
		logger.Warnw("Failed to unregister eir-diameter", "error", err)
	} else {
		logger.Info("✓ Unregistered eir-diameter group")
	}

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
