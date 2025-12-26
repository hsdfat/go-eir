package main

import (
	"fmt"
	"net"
	"os"

	govclient "github.com/chronnie/governance/client"
	"github.com/chronnie/governance/models"
	"github.com/hsdfat8/eir/internal/adapters/diameter"
	httpAdapter "github.com/hsdfat8/eir/internal/adapters/http"
	"github.com/hsdfat8/eir/internal/adapters/memory"
	"github.com/hsdfat8/eir/internal/config"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/logger"
)

// Application holds the application state
type Application struct {
	cfg            *config.Config
	logger         logger.Logger
	httpServer     *httpAdapter.Server
	diameterServer *diameter.Server
	govClient      *govclient.Client
}

// getLocalIP returns the non-loopback local IP of the host
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, address := range addrs {
		// Check if it's an IP address (not a network)
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

// getRegistrationIP returns the appropriate IP for service registration
// If the configured host is 0.0.0.0 or empty, returns the local IP
func getRegistrationIP(configuredHost string) string {
	if configuredHost == "" || configuredHost == "0.0.0.0" {
		return getLocalIP()
	}
	return configuredHost
}

// initializeRepositories sets up IMEI and audit repositories
func initializeRepositories(log logger.Logger) (ports.IMEIRepository, ports.AuditRepository) {
	imeiRepo := memory.NewInMemoryIMEIRepository()
	auditRepo := memory.NewInMemoryAuditRepository()
	log.Info("✓ Repositories initialized")
	return imeiRepo, auditRepo
}

// initializeHTTPServer configures and starts the HTTP/2 server
func initializeHTTPServer(cfg *config.Config, eirService ports.EIRService, log logger.Logger) *httpAdapter.Server {
	httpServerConfig := httpAdapter.ServerConfig{
		ListenAddr:   fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
		EnableH2C:    true, // Enable HTTP/2 Cleartext for testing
	}

	httpServer := httpAdapter.NewServer(httpServerConfig, eirService)

	if err := httpServer.Start(); err != nil {
		log.Fatalw("Failed to start HTTP server", "error", err)
	}

	log.Infow("✓ HTTP/2 server listening", "address", httpServer.GetAddr())
	return httpServer
}

// initializeDiameterServer configures and starts the Diameter S13 server
func initializeDiameterServer(cfg *config.Config, eirService ports.EIRService, log logger.Logger) *diameter.Server {
	diameterConfig := diameter.ServerConfig{
		Host:             cfg.Diameter.Host,
		Port:             cfg.Diameter.Port,
		OriginHost:       cfg.Diameter.OriginHost,
		OriginRealm:      cfg.Diameter.OriginRealm,
		ProductName:      cfg.Diameter.ProductName,
		VendorID:         cfg.Diameter.VendorID,
		MaxConnections:   cfg.Diameter.MaxConnections,
		ReadTimeout:      int64(cfg.Diameter.ReadTimeout),
		WriteTimeout:     int64(cfg.Diameter.WriteTimeout),
		WatchdogInterval: int64(cfg.Diameter.WatchdogInterval),
		WatchdogTimeout:  int64(cfg.Diameter.WatchdogTimeout),
		MaxMessageSize:   cfg.Diameter.MaxMessageSize,
		SendChannelSize:  cfg.Diameter.SendChannelSize,
		RecvChannelSize:  cfg.Diameter.RecvChannelSize,
	}

	diameterServer := diameter.NewServer(diameterConfig, eirService)

	if err := diameterServer.Start(); err != nil {
		log.Fatalw("Failed to start Diameter server", "error", err)
	}

	log.Infow("✓ Diameter S13 server listening", "address", diameterServer.GetAddr())
	return diameterServer
}

// registerWithGovernance handles governance/service discovery registration
func registerWithGovernance(cfg *config.Config, log logger.Logger) *govclient.Client {
	if !cfg.Governance.Enabled {
		log.Info("Governance registration disabled")
		return nil
	}

	governanceURL := cfg.Governance.URL
	if envURL := os.Getenv("GOVERNANCE_URL"); envURL != "" {
		governanceURL = envURL
	}

	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podName, _ = os.Hostname()
	}

	govClient := govclient.NewClient(&govclient.ClientConfig{
		ManagerURL:  governanceURL,
		ServiceName: "eir",
		PodName:     podName,
	})

	httpRegIP := getRegistrationIP(cfg.Server.Host)
	diameterRegIP := getRegistrationIP(cfg.Diameter.Host)

	if cfg.Server.Host == "0.0.0.0" || cfg.Server.Host == "" {
		log.Infow("HTTP server listening on all interfaces, using local IP for registration", "local_ip", httpRegIP)
	}
	if cfg.Diameter.Host == "0.0.0.0" || cfg.Diameter.Host == "" {
		log.Infow("Diameter server listening on all interfaces, using local IP for registration", "local_ip", diameterRegIP)
	}

	registration := &models.ServiceRegistration{
		ServiceName: "eir-diameter",
		PodName:     podName,
		Providers: []models.ProviderInfo{
			{
				Protocol: models.ProtocolHTTP,
				IP:       httpRegIP,
				Port:     cfg.Server.Port,
			},
			{
				Protocol: models.ProtocolTCP,
				IP:       diameterRegIP,
				Port:     cfg.Diameter.Port,
			},
		},
		HealthCheckURL:  fmt.Sprintf("http://%s:%d/health", httpRegIP, cfg.Server.Port),
		NotificationURL: fmt.Sprintf("http://%s:%d/governance/notify", httpRegIP, cfg.Server.Port),
		Subscriptions:   []string{},
	}

	if _,err := govClient.Register(registration); err != nil {
		if cfg.Governance.FailOnError {
			log.Fatalw("Failed to register EIR service", "error", err)
		}
		log.Warnw("Failed to register EIR service", "error", err)
	} else {
		log.Infow("✓ Registered EIR service with multiple providers", "url", governanceURL, "providers", len(registration.Providers))
	}

	return govClient
}

// shutdown performs graceful shutdown of all services
func (app *Application) shutdown() {
	app.logger.Info("Shutting down servers...")

	if app.govClient != nil {
		if err := app.govClient.Unregister(); err != nil {
			app.logger.Warnw("Failed to unregister EIR service", "error", err)
		} else {
			app.logger.Info("✓ Unregistered EIR service")
		}
	}

	if err := app.httpServer.Stop(); err != nil {
		app.logger.Errorw("HTTP server shutdown error", "error", err)
	}

	if err := app.diameterServer.Stop(); err != nil {
		app.logger.Errorw("Diameter server shutdown error", "error", err)
	}

	app.logger.Info("Servers stopped gracefully")
}
