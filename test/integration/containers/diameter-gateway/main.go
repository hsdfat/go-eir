package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hsdfat/diam-gw/commands/base"
	"github.com/hsdfat/diam-gw/commands/s13"
	"github.com/hsdfat/diam-gw/models_base"
)

// DiameterGateway is a pure forwarding gateway without business logic
type DiameterGateway struct {
	config        GatewayConfig
	draListener   net.Listener
	eirConn       net.Conn
	eirConnMutex  sync.Mutex
	shutdown      chan struct{}
	draConns      map[net.Conn]bool
	draConnsMutex sync.Mutex
}

// GatewayConfig holds gateway configuration
type GatewayConfig struct {
	DRAListenAddr string // Address to listen for DRA connections
	EIRServerAddr string // Address of EIR Core Application
	OriginHost    string
	OriginRealm   string
	ProductName   string
	VendorID      uint32
}

func main() {
	config := GatewayConfig{
		DRAListenAddr: getEnv("DRA_LISTEN_ADDR", "0.0.0.0:3868"),
		EIRServerAddr: getEnv("EIR_SERVER_ADDR", "eir-core:8080"),
		OriginHost:    getEnv("ORIGIN_HOST", "diameter-gw.epc.mnc001.mcc001.3gppnetwork.org"),
		OriginRealm:   getEnv("ORIGIN_REALM", "epc.mnc001.mcc001.3gppnetwork.org"),
		ProductName:   getEnv("PRODUCT_NAME", "Diameter-Gateway/1.0"),
		VendorID:      10415,
	}

	gateway := NewDiameterGateway(config)

	if err := gateway.Start(); err != nil {
		log.Fatalf("Failed to start Diameter Gateway: %v", err)
	}

	log.Println("✓ Diameter Gateway started successfully")
	log.Printf("  - Listening for DRA connections on: %s", config.DRAListenAddr)
	log.Printf("  - Forwarding to EIR Core at: %s", config.EIRServerAddr)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Diameter Gateway...")
	if err := gateway.Stop(); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	log.Println("Diameter Gateway stopped")
}

// NewDiameterGateway creates a new Diameter Gateway
func NewDiameterGateway(config GatewayConfig) *DiameterGateway {
	return &DiameterGateway{
		config:   config,
		shutdown: make(chan struct{}),
		draConns: make(map[net.Conn]bool),
	}
}

// Start starts the gateway
func (gw *DiameterGateway) Start() error {
	listener, err := net.Listen("tcp", gw.config.DRAListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", gw.config.DRAListenAddr, err)
	}

	gw.draListener = listener

	go gw.acceptDRAConnections()

	return nil
}

// Stop stops the gateway
func (gw *DiameterGateway) Stop() error {
	close(gw.shutdown)

	if gw.draListener != nil {
		gw.draListener.Close()
	}

	gw.draConnsMutex.Lock()
	for conn := range gw.draConns {
		conn.Close()
	}
	gw.draConnsMutex.Unlock()

	gw.eirConnMutex.Lock()
	if gw.eirConn != nil {
		gw.eirConn.Close()
	}
	gw.eirConnMutex.Unlock()

	return nil
}

// acceptDRAConnections accepts incoming DRA connections
func (gw *DiameterGateway) acceptDRAConnections() {
	for {
		select {
		case <-gw.shutdown:
			return
		default:
		}

		conn, err := gw.draListener.Accept()
		if err != nil {
			select {
			case <-gw.shutdown:
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}

		gw.draConnsMutex.Lock()
		gw.draConns[conn] = true
		gw.draConnsMutex.Unlock()

		log.Printf("New DRA connection from %s", conn.RemoteAddr())

		go gw.handleDRAConnection(conn)
	}
}

// handleDRAConnection handles a single DRA connection
func (gw *DiameterGateway) handleDRAConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		gw.draConnsMutex.Lock()
		delete(gw.draConns, conn)
		gw.draConnsMutex.Unlock()
		log.Printf("DRA connection closed: %s", conn.RemoteAddr())
	}()

	for {
		select {
		case <-gw.shutdown:
			return
		default:
		}

		// Read Diameter message
		message, err := gw.readDiameterMessage(conn)
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error from DRA: %v", err)
			}
			return
		}

		// Parse command code
		commandCode := gw.getCommandCode(message)
		isRequest := gw.isRequest(message)

		log.Printf("Received Diameter message: CommandCode=%d, IsRequest=%v, Size=%d bytes",
			commandCode, isRequest, len(message))

		// Handle message based on command code
		response, err := gw.routeMessage(commandCode, isRequest, message)
		if err != nil {
			log.Printf("Route error: %v", err)
			continue
		}

		// Send response back to DRA
		if response != nil {
			if err := gw.writeDiameterMessage(conn, response); err != nil {
				log.Printf("Write error to DRA: %v", err)
				return
			}
		}
	}
}

// routeMessage routes messages appropriately
func (gw *DiameterGateway) routeMessage(commandCode uint32, isRequest bool, message []byte) ([]byte, error) {
	switch commandCode {
	case 257: // CER
		if isRequest {
			return gw.handleCER(message)
		}
	case 280: // DWR
		if isRequest {
			return gw.handleDWR(message)
		}
	case 324: // ME-Identity-Check-Request (S13)
		if isRequest {
			return gw.forwardToEIRCore(message)
		}
	default:
		log.Printf("Unsupported command code: %d", commandCode)
	}

	return nil, nil
}

// handleCER handles Capabilities-Exchange-Request
func (gw *DiameterGateway) handleCER(message []byte) ([]byte, error) {
	req := &base.CapabilitiesExchangeRequest{}
	if err := req.Unmarshal(message); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CER: %w", err)
	}

	log.Printf("Processing CER from %s", req.OriginHost)

	// Build CEA response
	cea := base.NewCapabilitiesExchangeAnswer()
	cea.ResultCode = models_base.Unsigned32(2001) // DIAMETER_SUCCESS
	cea.OriginHost = models_base.DiameterIdentity(gw.config.OriginHost)
	cea.OriginRealm = models_base.DiameterIdentity(gw.config.OriginRealm)
	cea.HostIpAddress = []models_base.Address{
		models_base.Address(net.ParseIP("127.0.0.1")),
	}
	cea.VendorId = models_base.Unsigned32(gw.config.VendorID)
	cea.ProductName = models_base.UTF8String(gw.config.ProductName)

	// Copy application IDs from request
	if len(req.AuthApplicationId) > 0 {
		cea.AuthApplicationId = req.AuthApplicationId
	}

	// Preserve Hop-by-Hop and End-to-End IDs
	cea.Header.HopByHopID = req.Header.HopByHopID
	cea.Header.EndToEndID = req.Header.EndToEndID

	response, err := cea.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal CEA: %w", err)
	}

	log.Printf("Sent CEA to %s", req.OriginHost)

	return response, nil
}

// handleDWR handles Device-Watchdog-Request
func (gw *DiameterGateway) handleDWR(message []byte) ([]byte, error) {
	req := &base.DeviceWatchdogRequest{}
	if err := req.Unmarshal(message); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DWR: %w", err)
	}

	log.Println("Processing DWR")

	// Build DWA response
	dwa := base.NewDeviceWatchdogAnswer()
	dwa.ResultCode = models_base.Unsigned32(2001) // DIAMETER_SUCCESS
	dwa.OriginHost = models_base.DiameterIdentity(gw.config.OriginHost)
	dwa.OriginRealm = models_base.DiameterIdentity(gw.config.OriginRealm)

	// Preserve Hop-by-Hop and End-to-End IDs
	dwa.Header.HopByHopID = req.Header.HopByHopID
	dwa.Header.EndToEndID = req.Header.EndToEndID

	response, err := dwa.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DWA: %w", err)
	}

	log.Println("Sent DWA")

	return response, nil
}

// forwardToEIRCore forwards S13 messages to EIR Core Application
func (gw *DiameterGateway) forwardToEIRCore(message []byte) ([]byte, error) {
	// Parse request to extract IMEI for logging
	req := &s13.MEIdentityCheckRequest{}
	if err := req.Unmarshal(message); err == nil {
		var imei string
		if req.TerminalInformation != nil && req.TerminalInformation.Imei != nil {
			imei = string(*req.TerminalInformation.Imei)
		}
		log.Printf("Forwarding ME-Identity-Check-Request for IMEI: %s", imei)
	}

	// Get or establish connection to EIR Core
	eirConn, err := gw.getEIRConnection()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to EIR Core: %w", err)
	}

	// Forward request to EIR Core
	if err := gw.writeDiameterMessage(eirConn, message); err != nil {
		return nil, fmt.Errorf("failed to forward request to EIR Core: %w", err)
	}

	log.Println("Request forwarded to EIR Core")

	// Read response from EIR Core
	response, err := gw.readDiameterMessage(eirConn)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from EIR Core: %w", err)
	}

	log.Printf("Received response from EIR Core, forwarding to DRA (%d bytes)", len(response))

	return response, nil
}

// getEIRConnection gets or creates connection to EIR Core
func (gw *DiameterGateway) getEIRConnection() (net.Conn, error) {
	gw.eirConnMutex.Lock()
	defer gw.eirConnMutex.Unlock()

	// Check if existing connection is alive
	if gw.eirConn != nil {
		// Test connection with a short deadline
		gw.eirConn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
		one := make([]byte, 1)
		_, err := gw.eirConn.Read(one)
		gw.eirConn.SetReadDeadline(time.Time{})

		if err == nil || err == io.EOF {
			// Connection is alive
			return gw.eirConn, nil
		}

		// Connection is dead, close it
		gw.eirConn.Close()
		gw.eirConn = nil
	}

	// Create new connection
	log.Printf("Establishing new connection to EIR Core at %s", gw.config.EIRServerAddr)

	conn, err := net.DialTimeout("tcp", gw.config.EIRServerAddr, 5*time.Second)
	if err != nil {
		return nil, err
	}

	gw.eirConn = conn

	log.Println("✓ Connected to EIR Core")

	return conn, nil
}

// readDiameterMessage reads a complete Diameter message
func (gw *DiameterGateway) readDiameterMessage(conn net.Conn) ([]byte, error) {
	// Read header (20 bytes)
	header := make([]byte, 20)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	// Parse message length from header (bytes 1-3)
	messageLength := uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])

	if messageLength < 20 || messageLength > 1024*1024 { // Max 1MB
		return nil, fmt.Errorf("invalid message length: %d", messageLength)
	}

	// Read remaining message
	fullMessage := make([]byte, messageLength)
	copy(fullMessage[:20], header)

	if messageLength > 20 {
		if _, err := io.ReadFull(conn, fullMessage[20:]); err != nil {
			return nil, err
		}
	}

	return fullMessage, nil
}

// writeDiameterMessage writes a Diameter message
func (gw *DiameterGateway) writeDiameterMessage(conn net.Conn, message []byte) error {
	_, err := conn.Write(message)
	return err
}

// getCommandCode extracts command code from Diameter message
func (gw *DiameterGateway) getCommandCode(message []byte) uint32 {
	if len(message) < 8 {
		return 0
	}
	return uint32(message[5])<<16 | uint32(message[6])<<8 | uint32(message[7])
}

// isRequest checks if message is a request (R-bit set)
func (gw *DiameterGateway) isRequest(message []byte) bool {
	if len(message) < 5 {
		return false
	}
	return (message[4] & 0x80) != 0
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
