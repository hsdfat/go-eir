package diameter

import (
	"context"
	"fmt"
	"time"

	"github.com/hsdfat/diam-gw/commands/s13"
	"github.com/hsdfat/diam-gw/pkg/connection"
	"github.com/hsdfat/diam-gw/pkg/logger"
	"github.com/hsdfat/diam-gw/server"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Diameter request metrics
	diameterRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "diameter_requests_total",
			Help: "Total number of Diameter requests received",
		},
		[]string{"command", "result"},
	)

	diameterRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "diameter_request_duration_seconds",
			Help:    "Duration of Diameter request processing",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"command"},
	)

	diameterActiveConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "diameter_active_connections",
			Help: "Number of active Diameter connections",
		},
	)

	diameterErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "diameter_errors_total",
			Help: "Total number of Diameter errors",
		},
		[]string{"type"},
	)
)

// ServerConfig holds Diameter server configuration
type ServerConfig struct {
	Host             string
	Port             int
	OriginHost       string
	OriginRealm      string
	ProductName      string
	VendorID         uint32
	MaxConnections   int
	ReadTimeout      int64
	WriteTimeout     int64
	WatchdogInterval int64
	WatchdogTimeout  int64
	MaxMessageSize   int
	SendChannelSize  int
	RecvChannelSize  int
}

// Server represents a Diameter S13 server
type Server struct {
	config      ServerConfig
	handler     *S13Handler
	diamServer  *server.Server
	logger      logger.Logger
	listenAddr  string
}

// NewServer creates a new Diameter S13 server using diam-gw server package
func NewServer(config ServerConfig, eirService ports.EIRService) *Server {
	handler := NewS13Handler(eirService, config.OriginHost, config.OriginRealm)

	// Initialize logger
	log := logger.New("diameter-eir", "info")

	// Construct listen address from host and port
	listenAddr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	// Create server configuration using values from config
	serverConfig := &server.ServerConfig{
		ListenAddress:  listenAddr,
		MaxConnections: config.MaxConnections,
		ConnectionConfig: &server.ConnectionConfig{
			ReadTimeout:      time.Duration(config.ReadTimeout),
			WriteTimeout:     time.Duration(config.WriteTimeout),
			WatchdogInterval: time.Duration(config.WatchdogInterval),
			WatchdogTimeout:  time.Duration(config.WatchdogTimeout),
			MaxMessageSize:   config.MaxMessageSize,
			SendChannelSize:  config.SendChannelSize,
			RecvChannelSize:  config.RecvChannelSize,
			OriginHost:       config.OriginHost,
			OriginRealm:      config.OriginRealm,
			ProductName:      config.ProductName,
			VendorID:         config.VendorID,
			HandleWatchdog:   true,
		},
		RecvChannelSize: config.RecvChannelSize * 10, // Use a larger buffer for server recv
	}

	// Create diam-gw server
	diamServer := server.NewServer(serverConfig, log)

	s := &Server{
		config:     config,
		handler:    handler,
		diamServer: diamServer,
		logger:     log,
		listenAddr: listenAddr,
	}

	// Register S13 ME-Identity-Check-Request handler (Command Code 324)
	diamServer.HandleFunc(connection.Command{Interface: 16777252, Code: 324, Request: true}, s.handleMEIdentityCheck)

	return s
}

// Start starts the Diameter server
func (s *Server) Start() error {
	s.logger.Infow("Starting Diameter S13 server", "address", s.listenAddr)

	// Start the diam-gw server in a goroutine
	go func() {
		if err := s.diamServer.Start(); err != nil {
			s.logger.Errorw("Diameter server error", "error", err)
			diameterErrors.WithLabelValues("server_start").Inc()
			panic(err)
		}
	}()

	return nil
}

// GetAddr returns the listen address of the server
func (s *Server) GetAddr() string {
	return s.listenAddr
}

// Stop stops the Diameter server
func (s *Server) Stop() error {
	s.logger.Info("Stopping Diameter S13 server")
	return s.diamServer.Stop()
}

// handleMEIdentityCheck processes ME-Identity-Check-Request using the diam-gw handler pattern
func (s *Server) handleMEIdentityCheck(msg *connection.Message, conn connection.Conn) {
	startTime := time.Now()
	commandName := "ME-Identity-Check"

	// Increment active connections
	diameterActiveConnections.Inc()
	defer diameterActiveConnections.Dec()

	// Reconstruct full message from header and body
	fullMsg := append(msg.Header, msg.Body...)

	// Parse request
	req := s13.NewMEIdentityCheckRequest()
	if err := req.Unmarshal(fullMsg); err != nil {
		s.logger.Errorw("Failed to unmarshal ME-Identity-Check-Request", "error", err)
		diameterErrors.WithLabelValues("unmarshal_error").Inc()
		diameterRequestsTotal.WithLabelValues(commandName, "unmarshal_error").Inc()
		return
	}

	// Process request
	ctx := context.Background()
	answer, err := s.handler.HandleMEIdentityCheckRequest(ctx, req)
	if err != nil {
		s.logger.Errorw("Error processing ME-Identity-Check", "error", err)
		diameterErrors.WithLabelValues("processing_error").Inc()
		// Still send the answer (it contains error information)
	}

	// Marshal answer
	response, err := answer.Marshal()
	if err != nil {
		s.logger.Errorw("Failed to marshal ME-Identity-Check-Answer", "error", err)
		diameterErrors.WithLabelValues("marshal_error").Inc()
		diameterRequestsTotal.WithLabelValues(commandName, "marshal_error").Inc()
		return
	}

	// Send response
	resultLabel := "success"
	if _, err := conn.Write(response); err != nil {
		s.logger.Errorw("Failed to send ME-Identity-Check-Answer", "error", err)
		diameterErrors.WithLabelValues("send_error").Inc()
		resultLabel = "send_error"
	} else {
		s.logger.Infow("Sent ME-Identity-Check-Answer",
			"imei", string(*req.TerminalInformation.Imei),
			"result", answer.ResultCode)
	}

	// Record metrics
	duration := time.Since(startTime).Seconds()
	diameterRequestDuration.WithLabelValues(commandName).Observe(duration)
	diameterRequestsTotal.WithLabelValues(commandName, resultLabel).Inc()
}
