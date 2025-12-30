package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	govclient "github.com/chronnie/governance/client"
	"github.com/chronnie/governance/models"
	"github.com/gin-gonic/gin"
	"github.com/hsdfat/go-eir/internal/adapters/diameter"
	httpAdapter "github.com/hsdfat/go-eir/internal/adapters/http"
	"github.com/hsdfat/go-eir/internal/adapters/memory"
	"github.com/hsdfat/go-eir/internal/adapters/postgres"
	"github.com/hsdfat/go-eir/internal/config"
	"github.com/hsdfat/go-eir/internal/domain/ports"
	"github.com/hsdfat/go-eir/internal/logger"
	"github.com/jmoiron/sqlx"
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

// initializePostgresAdapter sets up PostgreSQL database adapter with migration
func initializePostgresAdapter(cfg *config.Config, log logger.Logger) (ports.DatabaseAdapter, error) {
	// Build PostgreSQL config for database connection
	dbConfig := postgres.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Database,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
	}

	log.Infow("Connecting to PostgreSQL",
		"host", dbConfig.Host,
		"port", dbConfig.Port,
		"database", dbConfig.Database,
	)

	// Create database connection first
	db, err := postgres.NewDB(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	log.Info("✓ Connected to PostgreSQL")

	// Run database migration before creating adapter
	if err := runDatabaseMigration(db, log); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run database migration: %w", err)
	}

	log.Info("✓ Database migration completed")

	// Build PostgreSQL config for adapter
	adapterConfig := &ports.PostgresConfig{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Database,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: int(cfg.Database.ConnMaxLifetime.Seconds()),
		ConnMaxIdleTime: int(cfg.Database.ConnMaxIdleTime.Seconds()),
	}

	// Create PostgreSQL adapter (it will create its own connection)
	adapter := postgres.NewPostgresAdapter(adapterConfig)

	// Connect adapter
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := adapter.Connect(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect adapter: %w", err)
	}

	// Close the migration connection since adapter has its own
	db.Close()

	// Verify schema
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel2()

	if err := adapter.HealthCheck(ctx2); err != nil {
		log.Warnw("Health check failed", "error", err)
	}

	log.Info("✓ Database adapter initialized")

	return adapter, nil
}

// runDatabaseMigration runs database migrations similar to governance
func runDatabaseMigration(db *sqlx.DB, log logger.Logger) error {
	// Create migrator with direct database connection
	migrator := postgres.NewMigrator(db)

	// Run migration
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	log.Info("Running database migration...")
	if err := migrator.Migrate(ctx); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	// Verify schema
	if err := migrator.VerifySchema(ctx); err != nil {
		log.Warnw("Schema verification had warnings", "error", err)
	}

	// Create partitions for current and next year
	currentYear := time.Now().Year()
	for _, year := range []int{currentYear, currentYear + 1} {
		if err := migrator.CreatePartitionsForYear(ctx, year); err != nil {
			log.Warnw("Failed to create partitions", "year", year, "error", err)
		}
	}

	return nil
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

	// Create HTTP server logger with mod: eir
	httpLog := log.With("component", "http-server").(logger.Logger)
	httpServer := httpAdapter.NewServer(httpServerConfig, eirService, httpLog)

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
func registerWithGovernance(cfg *config.Config, log logger.Logger, httpServer *httpAdapter.Server) *govclient.Client {
	if !cfg.Governance.Enabled {
		log.Info("Governance registration disabled")
		return nil
	}

	// Governance URL is now loaded from environment variables via pkg/config/env
	governanceURL := cfg.Governance.URL

	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podName, _ = os.Hostname()
	}

	// Create governance client
	govClient := govclient.NewClient(&govclient.ClientConfig{
		ManagerURL:  governanceURL,
		ServiceName: "eir",
		PodName:     podName,
	})

	// Add governance endpoints to HTTP server using the new handler functions
	router := httpServer.GetRouter()
	governance := router.Group("/governance")
	{
		// Use gin wrapper to convert http.HandlerFunc to gin.HandlerFunc
		governance.POST("/notify", func(c *gin.Context) {
			govClient.CreateNotificationHandler()(c.Writer, c.Request)
		})
		governance.Any("/heartbeat", func(c *gin.Context) {
			govClient.CreateHeartbeatHandler(nil)(c.Writer, c.Request)
		})
	}

	log.Info("✓ Governance HTTP endpoints added to router")

	httpRegIP := getRegistrationIP(cfg.Server.Host)
	diameterRegIP := getRegistrationIP(cfg.Diameter.Host)

	if cfg.Server.Host == "0.0.0.0" || cfg.Server.Host == "" {
		log.Infow("HTTP server listening on all interfaces, using local IP for registration", "local_ip", httpRegIP)
	}
	if cfg.Diameter.Host == "0.0.0.0" || cfg.Diameter.Host == "" {
		log.Infow("Diameter server listening on all interfaces, using local IP for registration", "local_ip", diameterRegIP)
	}

	registration := &models.ServiceRegistration{
		ServiceName: models.ServiceNameEir,
		PodName:     podName,
		Providers: []models.ProviderInfo{
			{
				ProviderID: string(models.ProviderEIRHTTP),
				Protocol:   models.ProtocolHTTP,
				IP:         httpRegIP,
				Port:       cfg.Server.Port,
			},
			{
				ProviderID: string(models.ProviderEIRDiameter),
				Protocol:   models.ProtocolTCP,
				IP:         diameterRegIP,
				Port:       cfg.Diameter.Port,
			},
		},
		HealthCheckURL:  fmt.Sprintf("http://%s:%d/health", httpRegIP, cfg.Server.Port),
		NotificationURL: fmt.Sprintf("http://%s:%d/governance/notify", httpRegIP, cfg.Server.Port),
		Subscriptions:   nil,
	}

	if _, err := govClient.Register(registration); err != nil {
		if cfg.Governance.FailOnError {
			log.Fatalw("Failed to register EIR service", "error", err)
		}
		log.Warnw("Failed to register EIR service", "error", err)
	} else {
		log.Infow("✓ Registered EIR service with multiple providers",
			"url", governanceURL,
			"service", registration.ServiceName,
			"pod", podName,
			"providers", registration.Providers,
			"subcriptions", registration.Subscriptions,
		)
	}

	// Start heartbeat
	govClient.StartHeartbeat()
	log.Info("✓ Started governance heartbeat")

	return govClient
}

// shutdown performs graceful shutdown of all services
func (app *Application) shutdown() {
	app.logger.Info("Shutting down servers...")

	if app.govClient != nil {
		app.govClient.StopHeartbeat()
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
