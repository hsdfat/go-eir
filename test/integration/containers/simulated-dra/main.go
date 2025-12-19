package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hsdfat/diam-gw/commands/base"
	"github.com/hsdfat/diam-gw/commands/s13"
	"github.com/hsdfat/diam-gw/models_base"
)

// SimulatedDRA represents a simulated Diameter Routing Agent
// This component establishes Diameter connection to the Gateway and forwards S13 messages
type SimulatedDRA struct {
	config   DRAConfig
	listener net.Listener
	shutdown chan struct{}
}

// DRAConfig holds DRA configuration
type DRAConfig struct {
	ListenAddr  string // Address to listen for client connections (e.g., from MME simulator)
	GatewayAddr string // Address of Diameter Gateway
	OriginHost  string
	OriginRealm string
	ProductName string
	VendorID    uint32
	AuthAppID   uint32
}

func main() {
	config := DRAConfig{
		ListenAddr:  getEnv("DRA_LISTEN_ADDR", "0.0.0.0:3869"),
		GatewayAddr: getEnv("GATEWAY_ADDR", "diameter-gateway:3868"),
		OriginHost:  getEnv("ORIGIN_HOST", "dra.epc.mnc001.mcc001.3gppnetwork.org"),
		OriginRealm: getEnv("ORIGIN_REALM", "epc.mnc001.mcc001.3gppnetwork.org"),
		ProductName: getEnv("PRODUCT_NAME", "DRA-Simulator/1.0"),
		VendorID:    10415,
		AuthAppID:   16777252, // S13 Application ID
	}

	dra := NewSimulatedDRA(config)

	if err := dra.Start(); err != nil {
		log.Fatalf("Failed to start Simulated DRA: %v", err)
	}

	log.Println("✓ Simulated DRA started successfully")
	log.Printf("  - Listening for client connections on: %s", config.ListenAddr)
	log.Printf("  - Routing to Diameter Gateway at: %s", config.GatewayAddr)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Simulated DRA...")
	if err := dra.Stop(); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
	log.Println("Simulated DRA stopped")
}

// NewSimulatedDRA creates a new Simulated DRA
func NewSimulatedDRA(config DRAConfig) *SimulatedDRA {
	return &SimulatedDRA{
		config:   config,
		shutdown: make(chan struct{}),
	}
}

// Start starts the DRA
func (dra *SimulatedDRA) Start() error {
	listener, err := net.Listen("tcp", dra.config.ListenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", dra.config.ListenAddr, err)
	}

	dra.listener = listener

	go dra.acceptConnections()

	return nil
}

// Stop stops the DRA
func (dra *SimulatedDRA) Stop() error {
	close(dra.shutdown)

	if dra.listener != nil {
		dra.listener.Close()
	}

	return nil
}

// acceptConnections accepts incoming client connections
func (dra *SimulatedDRA) acceptConnections() {
	for {
		select {
		case <-dra.shutdown:
			return
		default:
		}

		conn, err := dra.listener.Accept()
		if err != nil {
			select {
			case <-dra.shutdown:
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}

		log.Printf("New client connection from %s", conn.RemoteAddr())

		go dra.handleClientConnection(conn)
	}
}

// handleClientConnection handles a single client connection
func (dra *SimulatedDRA) handleClientConnection(clientConn net.Conn) {
	defer func() {
		clientConn.Close()
		log.Printf("Client connection closed: %s", clientConn.RemoteAddr())
	}()

	// Establish connection to Diameter Gateway
	gatewayConn, err := dra.connectToGateway()
	if err != nil {
		log.Printf("Failed to connect to gateway: %v", err)
		return
	}
	defer gatewayConn.Close()

	log.Printf("✓ Established connection to Diameter Gateway at %s", dra.config.GatewayAddr)

	// Bidirectional proxy: forward messages between client and gateway
	errChan := make(chan error, 2)

	// Client -> Gateway
	go func() {
		errChan <- dra.forwardMessages(clientConn, gatewayConn, "Client->Gateway")
	}()

	// Gateway -> Client
	go func() {
		errChan <- dra.forwardMessages(gatewayConn, clientConn, "Gateway->Client")
	}()

	// Wait for either direction to fail
	err = <-errChan
	if err != nil && err != io.EOF {
		log.Printf("Forwarding error: %v", err)
	}
}

// connectToGateway establishes connection to Diameter Gateway
func (dra *SimulatedDRA) connectToGateway() (net.Conn, error) {
	// Retry logic for gateway connection
	var conn net.Conn
	var err error

	for i := 0; i < 5; i++ {
		conn, err = net.DialTimeout("tcp", dra.config.GatewayAddr, 5*time.Second)
		if err == nil {
			break
		}

		log.Printf("Failed to connect to gateway (attempt %d/5): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to gateway after retries: %w", err)
	}

	return conn, nil
}

// forwardMessages forwards Diameter messages from source to destination
func (dra *SimulatedDRA) forwardMessages(src, dst net.Conn, direction string) error {
	for {
		select {
		case <-dra.shutdown:
			return nil
		default:
		}

		// Read Diameter message from source
		message, err := dra.readDiameterMessage(src)
		if err != nil {
			if err != io.EOF {
				log.Printf("[%s] Read error: %v", direction, err)
			}
			return err
		}

		// Parse and log message details
		commandCode := dra.getCommandCode(message)
		isRequest := dra.isRequest(message)

		log.Printf("[%s] Forwarding Diameter message: CommandCode=%d, IsRequest=%v, Size=%d bytes",
			direction, commandCode, isRequest, len(message))

		// Log specific details for S13 messages
		if commandCode == 324 {
			dra.logS13Message(message, isRequest, direction)
		}

		// Forward message to destination
		if err := dra.writeDiameterMessage(dst, message); err != nil {
			log.Printf("[%s] Write error: %v", direction, err)
			return err
		}
	}
}

// logS13Message logs S13 message details
func (dra *SimulatedDRA) logS13Message(message []byte, isRequest bool, direction string) {
	if isRequest {
		req := &s13.MEIdentityCheckRequest{}
		if err := req.Unmarshal(message); err == nil {
			var imei string
			if req.TerminalInformation != nil && req.TerminalInformation.Imei != nil {
				imei = string(*req.TerminalInformation.Imei)
			}
			log.Printf("[%s] S13 Request: IMEI=%s, SessionID=%s, HopByHop=%d, EndToEnd=%d",
				direction, imei, req.SessionId, req.Header.HopByHopID, req.Header.EndToEndID)
		}
	} else {
		ans := &s13.MEIdentityCheckAnswer{}
		if err := ans.Unmarshal(message); err == nil {
			var status, resultCode string
			if ans.EquipmentStatus != nil {
				status = fmt.Sprintf("%d", *ans.EquipmentStatus)
			}
			if ans.ResultCode != nil {
				resultCode = fmt.Sprintf("%d", *ans.ResultCode)
			}
			log.Printf("[%s] S13 Answer: Status=%s, ResultCode=%s, HopByHop=%d, EndToEnd=%d",
				direction, status, resultCode, ans.Header.HopByHopID, ans.Header.EndToEndID)
		}
	}
}

// readDiameterMessage reads a complete Diameter message
func (dra *SimulatedDRA) readDiameterMessage(conn net.Conn) ([]byte, error) {
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
func (dra *SimulatedDRA) writeDiameterMessage(conn net.Conn, message []byte) error {
	_, err := conn.Write(message)
	return err
}

// getCommandCode extracts command code from Diameter message
func (dra *SimulatedDRA) getCommandCode(message []byte) uint32 {
	if len(message) < 8 {
		return 0
	}
	return uint32(message[5])<<16 | uint32(message[6])<<8 | uint32(message[7])
}

// isRequest checks if message is a request (R-bit set)
func (dra *SimulatedDRA) isRequest(message []byte) bool {
	if len(message) < 5 {
		return false
	}
	return (message[4] & 0x80) != 0
}

// handleCER handles Capabilities-Exchange-Request (if needed locally)
func (dra *SimulatedDRA) handleCER(message []byte) ([]byte, error) {
	req := &base.CapabilitiesExchangeRequest{}
	if err := req.Unmarshal(message); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CER: %w", err)
	}

	log.Printf("Processing CER from %s", req.OriginHost)

	// Build CEA response
	cea := base.NewCapabilitiesExchangeAnswer()
	cea.ResultCode = models_base.Unsigned32(2001) // DIAMETER_SUCCESS
	cea.OriginHost = models_base.DiameterIdentity(dra.config.OriginHost)
	cea.OriginRealm = models_base.DiameterIdentity(dra.config.OriginRealm)
	cea.HostIpAddress = []models_base.Address{
		models_base.Address(net.ParseIP("127.0.0.1")),
	}
	cea.VendorId = models_base.Unsigned32(dra.config.VendorID)
	cea.ProductName = models_base.UTF8String(dra.config.ProductName)

	// Set S13 application ID
	cea.AuthApplicationId = []models_base.Unsigned32{models_base.Unsigned32(dra.config.AuthAppID)}

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

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
