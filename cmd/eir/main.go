package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/hsdfat8/eir/internal/config"
	"github.com/hsdfat8/eir/internal/domain/service"
	"github.com/hsdfat8/eir/internal/logger"
)

func main() {
	log := logger.New("eir-main", "info")

	cfg, err := config.Load("")
	if err != nil {
		log.Fatalw("Failed to load configuration", "error", err)
	}

	imeiRepo, auditRepo := initializeRepositories(log)

	eirService := service.NewEIRService(cfg, imeiRepo, auditRepo, nil)
	log.Info("✓ EIR service initialized")

	app := &Application{
		cfg:            cfg,
		logger:         log,
		httpServer:     initializeHTTPServer(cfg, eirService, log),
		diameterServer: initializeDiameterServer(cfg, eirService, log),
		govClient:      registerWithGovernance(cfg, log),
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	app.shutdown()
}
