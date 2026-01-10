package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/hsdfat/go-eir/internal/config"
	"github.com/hsdfat/go-eir/internal/domain/service"
	"github.com/hsdfat/go-eir/internal/logger"
)

func main() {
	log := logger.New("eir", "info")

	cfg, err := config.Load("")
	if err != nil {
		log.Fatalw("Failed to load configuration", "error", err)
	}

	// Initialize PostgreSQL database adapter with migration
	dbAdapter, err := initializePostgresAdapter(cfg, log)
	if err != nil {
		log.Fatalw("Failed to initialize database", "error", err)
	}
	defer dbAdapter.Disconnect(context.Background())

	// Get repositories from database adapter
	imeiRepo := dbAdapter.GetIMEIRepository()
	auditRepo := dbAdapter.GetAuditRepository()

	eirService := service.NewEIRService(cfg, imeiRepo, auditRepo, nil)
	log.Info("✓ EIR service initialized")

	// Initialize stats collector
	statsCollector := initializeStatsCollector(log)

	// Initialize servers
	httpServer := initializeHTTPServer(cfg, eirService, statsCollector, log)
	diameterServer := initializeDiameterServer(cfg, eirService, statsCollector, log)

	// Register with governance (must be done after HTTP server is initialized)
	govClient := registerWithGovernance(cfg, log, httpServer)

	app := &Application{
		cfg:            cfg,
		logger:         log,
		httpServer:     httpServer,
		diameterServer: diameterServer,
		govClient:      govClient,
		statsCollector: statsCollector,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	app.shutdown()
}
