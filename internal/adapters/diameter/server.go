package diameter

import (
	"context"

	"github.com/hsdfat/diam-gw/commands/s13"
	"github.com/hsdfat/diam-gw/pkg/connection"
	"github.com/hsdfat/diam-gw/pkg/logger"
	"github.com/hsdfat/diam-gw/server"
	"github.com/hsdfat8/eir/internal/domain/ports"
)

// ServerConfig holds Diameter server configuration
type ServerConfig struct {
	ListenAddr  string
	OriginHost  string
	OriginRealm string
	ProductName string
	VendorID    uint32
}

// Server represents a Diameter S13 server
type Server struct {
	config      ServerConfig
	handler     *S13Handler
	diamServer  *server.Server
	logger      logger.Logger
}

// NewServer creates a new Diameter S13 server using diam-gw server package
func NewServer(config ServerConfig, eirService ports.EIRService) *Server {
	handler := NewS13Handler(eirService, config.OriginHost, config.OriginRealm)

	// Initialize logger
	log := logger.New("diameter-eir", "info")

	// Create server configuration
	serverConfig := &server.ServerConfig{
		ListenAddress: config.ListenAddr,
		MaxConnections: 1000,
		ConnectionConfig: &server.ConnectionConfig{
			ReadTimeout:      30000000000,  // 30 seconds
			WriteTimeout:     10000000000,  // 10 seconds
			WatchdogInterval: 30000000000,  // 30 seconds
			WatchdogTimeout:  10000000000,  // 10 seconds
			MaxMessageSize:   65535,
			SendChannelSize:  100,
			RecvChannelSize:  100,
			OriginHost:       config.OriginHost,
			OriginRealm:      config.OriginRealm,
			ProductName:      config.ProductName,
			VendorID:         config.VendorID,
			HandleWatchdog:   true,
		},
		RecvChannelSize: 1000,
	}

	// Create diam-gw server
	diamServer := server.NewServer(serverConfig, log)

	s := &Server{
		config:     config,
		handler:    handler,
		diamServer: diamServer,
		logger:     log,
	}

	// Register S13 ME-Identity-Check-Request handler (Command Code 324)
	diamServer.HandleFunc(connection.Command{Interface: 16777252, Code: 324, Request: true}, s.handleMEIdentityCheck)

	return s
}

// Start starts the Diameter server
func (s *Server) Start() error {
	s.logger.Infow("Starting Diameter S13 server", "address", s.config.ListenAddr)

	// Start the diam-gw server in a goroutine
	go func() {
		if err := s.diamServer.Start(); err != nil {
			s.logger.Errorw("Diameter server error", "error", err)
		}
	}()

	return nil
}

// Stop stops the Diameter server
func (s *Server) Stop() error {
	s.logger.Info("Stopping Diameter S13 server")
	return s.diamServer.Stop()
}

// handleMEIdentityCheck processes ME-Identity-Check-Request using the diam-gw handler pattern
func (s *Server) handleMEIdentityCheck(msg *connection.Message, conn connection.Conn) {
	// Reconstruct full message from header and body
	fullMsg := append(msg.Header, msg.Body...)

	// Parse request
	req := s13.NewMEIdentityCheckRequest()
	if err := req.Unmarshal(fullMsg); err != nil {
		s.logger.Errorw("Failed to unmarshal ME-Identity-Check-Request", "error", err)
		return
	}

	// Process request
	ctx := context.Background()
	answer, err := s.handler.HandleMEIdentityCheckRequest(ctx, req)
	if err != nil {
		s.logger.Errorw("Error processing ME-Identity-Check", "error", err)
		// Still send the answer (it contains error information)
	}

	// Marshal answer
	response, err := answer.Marshal()
	if err != nil {
		s.logger.Errorw("Failed to marshal ME-Identity-Check-Answer", "error", err)
		return
	}

	// Send response
	if _, err := conn.Write(response); err != nil {
		s.logger.Errorw("Failed to send ME-Identity-Check-Answer", "error", err)
	} else {
		s.logger.Infow("Sent ME-Identity-Check-Answer",
			"imei", string(*req.TerminalInformation.Imei),
			"result", answer.ResultCode)
	}
}
