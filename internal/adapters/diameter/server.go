package diameter

import (
	"context"
	"fmt"
	"net"

	"github.com/hsdfat/diam-gw/commands/s13"
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
	config   ServerConfig
	handler  *S13Handler
	listener net.Listener
}

// NewServer creates a new Diameter S13 server
func NewServer(config ServerConfig, eirService ports.EIRService) *Server {
	handler := NewS13Handler(eirService, config.OriginHost, config.OriginRealm)

	return &Server{
		config:  config,
		handler: handler,
	}
}

// Start starts the Diameter server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", s.config.ListenAddr, err)
	}

	s.listener = listener
	fmt.Printf("Diameter S13 server listening on %s\n", s.config.ListenAddr)

	go s.acceptConnections()

	return nil
}

// Stop stops the Diameter server
func (s *Server) Stop() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// acceptConnections accepts incoming Diameter connections
func (s *Server) acceptConnections() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			fmt.Printf("Error accepting connection: %v\n", err)
			continue
		}

		go s.handleConnection(conn)
	}
}

// handleConnection handles a Diameter connection
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Printf("New Diameter connection from %s\n", conn.RemoteAddr())

	// Read Diameter messages in a loop
	for {
		// Read Diameter message header (20 bytes)
		header := make([]byte, 20)
		n, err := conn.Read(header)
		if err != nil {
			fmt.Printf("Connection closed: %v\n", err)
			return
		}

		if n != 20 {
			fmt.Printf("Invalid Diameter header size: %d\n", n)
			return
		}

		// Parse message length from header (bytes 1-3)
		messageLength := uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])

		// Read full message
		message := make([]byte, messageLength)
		copy(message[:20], header)

		remaining := int(messageLength) - 20
		if remaining > 0 {
			n, err = conn.Read(message[20:])
			if err != nil || n != remaining {
				fmt.Printf("Failed to read message body: %v\n", err)
				return
			}
		}

		// Process message
		response, err := s.processMessage(message)
		if err != nil {
			fmt.Printf("Error processing message: %v\n", err)
			continue
		}

		// Send response
		if response != nil {
			_, err = conn.Write(response)
			if err != nil {
				fmt.Printf("Error sending response: %v\n", err)
				return
			}
		}
	}
}

// processMessage processes a Diameter message and returns a response
func (s *Server) processMessage(message []byte) ([]byte, error) {
	// Parse command code (bytes 5-7)
	commandCode := uint32(message[5])<<16 | uint32(message[6])<<8 | uint32(message[7])

	// Check if it's a request (R-bit in flags, byte 4)
	isRequest := (message[4] & 0x80) != 0

	// Handle ME-Identity-Check-Request (Command Code 324)
	if commandCode == 324 && isRequest {
		return s.handleMEIdentityCheck(message)
	}

	// Handle Capabilities-Exchange-Request (Command Code 257)
	if commandCode == 257 && isRequest {
		return s.handleCapabilitiesExchange(message)
	}

	// Handle Device-Watchdog-Request (Command Code 280)
	if commandCode == 280 && isRequest {
		return s.handleDeviceWatchdog(message)
	}

	fmt.Printf("Unsupported command code: %d\n", commandCode)
	return nil, nil
}

// handleMEIdentityCheck processes ME-Identity-Check-Request
func (s *Server) handleMEIdentityCheck(message []byte) ([]byte, error) {
	// Parse request
	req := s13.NewMEIdentityCheckRequest()
	if err := req.Unmarshal(message); err != nil {
		return nil, fmt.Errorf("failed to unmarshal request: %w", err)
	}

	// Process request
	ctx := context.Background()
	answer, err := s.handler.HandleMEIdentityCheckRequest(ctx, req)
	if err != nil {
		fmt.Printf("Error processing ME-Identity-Check: %v\n", err)
		// Still return the answer (it contains error information)
	}

	// Marshal answer
	response, err := answer.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal answer: %w", err)
	}

	return response, nil
}

// handleCapabilitiesExchange handles CER (simplified implementation)
func (s *Server) handleCapabilitiesExchange(message []byte) ([]byte, error) {
	// TODO: Implement full CER/CEA handling
	// For now, return a minimal CEA
	fmt.Println("Received Capabilities-Exchange-Request")
	return nil, nil
}

// handleDeviceWatchdog handles DWR (simplified implementation)
func (s *Server) handleDeviceWatchdog(message []byte) ([]byte, error) {
	// TODO: Implement full DWR/DWA handling
	// For now, return a minimal DWA
	fmt.Println("Received Device-Watchdog-Request")
	return nil, nil
}
