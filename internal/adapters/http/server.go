package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hsdfat8/eir/internal/domain/ports"
	"github.com/hsdfat8/eir/internal/observability"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// ServerConfig holds HTTP/2 server configuration
type ServerConfig struct {
	ListenAddr      string        // Listen address (e.g., "0.0.0.0:8080")
	ReadTimeout     time.Duration // Read timeout
	WriteTimeout    time.Duration // Write timeout
	IdleTimeout     time.Duration // Idle timeout
	EnableTLS       bool          // Enable TLS (required for proper HTTP/2)
	TLSCertFile     string        // TLS certificate file path
	TLSKeyFile      string        // TLS key file path
	EnableH2C       bool          // Enable H2C (HTTP/2 Cleartext) for testing
	MaxHeaderBytes  int           // Max header size
	ShutdownTimeout time.Duration // Graceful shutdown timeout
}

// Server represents the HTTP/2 server
type Server struct {
	config     ServerConfig
	httpServer *http.Server
	listener   net.Listener
	eirService ports.EIRService
	router     *gin.Engine
	logger     observability.Logger
}

// NewServer creates a new HTTP/2 server instance
func NewServer(config ServerConfig, eirService ports.EIRService) *Server {
	// Set defaults
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 30 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 30 * time.Second
	}
	if config.IdleTimeout == 0 {
		config.IdleTimeout = 120 * time.Second
	}
	if config.MaxHeaderBytes == 0 {
		config.MaxHeaderBytes = 1 << 20 // 1MB
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 10 * time.Second
	}

	router := SetupRouter(eirService)

	// Initialize logger
	log := observability.New("http-server", "info")

	return &Server{
		config:     config,
		eirService: eirService,
		router:     router,
		logger:     log,
	}
}

// Start starts the HTTP/2 server
func (s *Server) Start() error {
	// Create listener first (supports port 0 for testing)
	listener, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}
	s.listener = listener

	// Update config with actual address
	s.config.ListenAddr = listener.Addr().String()

	var handler http.Handler = s.router

	// Configure HTTP/2 server
	s.httpServer = &http.Server{
		Addr:           s.config.ListenAddr,
		Handler:        handler,
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		IdleTimeout:    s.config.IdleTimeout,
		MaxHeaderBytes: s.config.MaxHeaderBytes,
	}

	// Start server based on configuration
	if s.config.EnableTLS {
		return s.startTLS()
	} else if s.config.EnableH2C {
		return s.startH2C()
	} else {
		return s.startHTTP1()
	}
}

// startTLS starts the server with TLS (proper HTTP/2)
func (s *Server) startTLS() error {
	// Configure TLS with HTTP/2
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2", "http/1.1"}, // Prefer HTTP/2
	}
	s.httpServer.TLSConfig = tlsConfig

	// Configure HTTP/2
	http2Server := &http2.Server{
		MaxHandlers:                  0,
		MaxConcurrentStreams:         250,
		MaxReadFrameSize:             1 << 20, // 1MB
		PermitProhibitedCipherSuites: false,
	}

	if err := http2.ConfigureServer(s.httpServer, http2Server); err != nil {
		return fmt.Errorf("failed to configure HTTP/2: %w", err)
	}

	s.logger.Infow("Starting HTTP/2 server with TLS", "address", s.config.ListenAddr)

	// Start server in goroutine
	go func() {
		if err := s.httpServer.ServeTLS(s.listener, s.config.TLSCertFile, s.config.TLSKeyFile); err != nil && err != http.ErrServerClosed {
			s.logger.Errorw("HTTP/2 TLS server error", "error", err)
		}
	}()

	return nil
}

// startH2C starts the server with H2C (HTTP/2 Cleartext) for testing
func (s *Server) startH2C() error {
	h2cHandler := h2c.NewHandler(s.router, &http2.Server{
		MaxHandlers:          0,
		MaxConcurrentStreams: 250,
		MaxReadFrameSize:     1 << 20, // 1MB
	})

	s.httpServer.Handler = h2cHandler

	s.logger.Infow("Starting HTTP/2 (H2C) server", "address", s.config.ListenAddr)

	// Start server in goroutine
	go func() {
		if err := s.httpServer.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			s.logger.Errorw("HTTP/2 H2C server error", "error", err)
		}
	}()

	return nil
}

// startHTTP1 starts a standard HTTP/1.1 server
func (s *Server) startHTTP1() error {
	s.logger.Infow("Starting HTTP/1.1 server", "address", s.config.ListenAddr)

	// Start server in goroutine
	go func() {
		if err := s.httpServer.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			s.logger.Errorw("HTTP/1.1 server error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully stops the HTTP/2 server
func (s *Server) Stop() error {
	if s.httpServer == nil {
		return nil
	}

	s.logger.Infow("Stopping HTTP server", "address", s.config.ListenAddr)

	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	s.logger.Info("HTTP server stopped successfully")
	return nil
}

// GetAddr returns the server's listen address
func (s *Server) GetAddr() string {
	return s.config.ListenAddr
}

// IsRunning checks if the server is running
func (s *Server) IsRunning() bool {
	return s.httpServer != nil && s.listener != nil
}
