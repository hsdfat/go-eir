package main

import (
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

	imeiRepo, auditRepo := initializeRepositories(log)

	eirService := service.NewEIRService(cfg, imeiRepo, auditRepo, nil)
	log.Info("✓ EIR service initialized")

	// Initialize servers
	httpServer := initializeHTTPServer(cfg, eirService, log)
	diameterServer := initializeDiameterServer(cfg, eirService, log)

	// Register with governance (must be done after HTTP server is initialized)
	govClient := registerWithGovernance(cfg, log, httpServer)

	app := &Application{
		cfg:            cfg,
		logger:         log,
		httpServer:     httpServer,
		diameterServer: diameterServer,
		govClient:      govClient,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	app.shutdown()
}
