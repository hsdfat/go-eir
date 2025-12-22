package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/hsdfat8/eir/internal/adapters/diameter"
	httpAdapter "github.com/hsdfat8/eir/internal/adapters/http"
	"github.com/hsdfat8/eir/internal/adapters/postgres"
	"github.com/hsdfat8/eir/internal/config"
	"github.com/hsdfat8/eir/internal/domain/service"
)

func main() {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize PostgreSQL database
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

	db, err := postgres.NewDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("✓ Connected to PostgreSQL database")

	// Initialize repositories
	imeiRepo := postgres.NewIMEIRepository(db)
	auditRepo := postgres.NewAuditRepository(db)

	// Initialize cache (optional, nil if disabled)
	// TODO: Implement Redis cache adapter if cfg.Cache.Enabled

	// Initialize EIR service
	eirService := service.NewEIRService(imeiRepo, auditRepo, nil)

	log.Println("✓ EIR service initialized")

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
		log.Fatalf("Failed to start HTTP server: %v", err)
	}

	log.Printf("✓ HTTP/2 server listening on %s", httpServer.GetAddr())

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
		log.Fatalf("Failed to start Diameter server: %v", err)
	}

	log.Println("✓ Diameter S13 server started")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down servers...")

	// Shutdown HTTP/2 server
	if err := httpServer.Stop(); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	// Shutdown Diameter server
	if err := diameterServer.Stop(); err != nil {
		log.Printf("Diameter server shutdown error: %v", err)
	}

	log.Println("Servers stopped gracefully")
}
