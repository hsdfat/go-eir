package testutil

import (
	"net"
	"os"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// TestPCAPWriteRead tests writing and reading PCAP files
func TestPCAPWriteRead(t *testing.T) {
	testFile := "test_pcap_verification.pcap"
	defer os.Remove(testFile)

	// Create PCAP writer
	writer, err := NewPCAPWriter(testFile)
	if err != nil {
		t.Fatalf("Failed to create PCAP writer: %v", err)
	}

	// Test data
	srcIP := net.IPv4(127, 0, 0, 1)
	dstIP := net.IPv4(127, 0, 0, 1)
	srcPort := uint16(12345)
	dstPort := uint16(3868)

	// Write HTTP/1.1 request
	httpRequest := []byte("GET /health HTTP/1.1\r\nHost: localhost\r\n\r\n")
	if err := writer.WritePacket(httpRequest, srcIP, dstIP, srcPort, uint16(8080), false); err != nil {
		t.Fatalf("Failed to write HTTP packet: %v", err)
	}

	// Write Diameter CER message (simplified)
	diameterCER := make([]byte, 136)
	diameterCER[0] = 0x01 // Version
	diameterCER[1] = 0x00 // Length (3 bytes)
	diameterCER[2] = 0x00
	diameterCER[3] = 0x88 // 136 bytes
	diameterCER[4] = 0x80 // Flags: Request
	diameterCER[5] = 0x00 // Command Code (3 bytes)
	diameterCER[6] = 0x01 // CER = 257
	diameterCER[7] = 0x01
	// Application ID = 0
	// Hop-by-Hop ID and End-to-End ID would be here
	copy(diameterCER[8:], "test data")

	if err := writer.WritePacket(diameterCER, srcIP, dstIP, srcPort, dstPort, false); err != nil {
		t.Fatalf("Failed to write Diameter packet: %v", err)
	}

	// Close writer
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close PCAP writer: %v", err)
	}

	// Read and verify PCAP file
	handle, err := pcap.OpenOffline(testFile)
	if err != nil {
		t.Fatalf("Failed to open PCAP file: %v", err)
	}
	defer handle.Close()

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packetCount := 0
	httpFound := false
	diameterFound := false
	tcpHandshakeFound := false

	for packet := range packetSource.Packets() {
		packetCount++

		// Check for TCP layer
		tcpLayer := packet.Layer(layers.LayerTypeTCP)
		if tcpLayer != nil {
			tcp, _ := tcpLayer.(*layers.TCP)

			// Check for TCP handshake (SYN, SYN-ACK, ACK)
			if tcp.SYN && !tcp.ACK {
				tcpHandshakeFound = true
				t.Logf("Found SYN packet: %s:%d -> %s:%d",
					packet.NetworkLayer().NetworkFlow().Src(),
					tcp.SrcPort,
					packet.NetworkLayer().NetworkFlow().Dst(),
					tcp.DstPort)
			}

			// Check application data
			if appLayer := packet.ApplicationLayer(); appLayer != nil {
				payload := appLayer.Payload()

				// Check for HTTP
				if len(payload) > 0 && string(payload[:min(4, len(payload))]) == "GET " {
					httpFound = true
					t.Logf("Found HTTP GET request: %s", string(payload[:min(50, len(payload))]))
				}

				// Check for Diameter (version byte = 0x01)
				if len(payload) > 0 && payload[0] == 0x01 {
					diameterFound = true
					t.Logf("Found Diameter message: version=0x%02x, length=%d bytes",
						payload[0], len(payload))

					// Verify Diameter header structure
					if len(payload) >= 20 {
						version := payload[0]
						length := uint32(payload[1])<<16 | uint32(payload[2])<<8 | uint32(payload[3])
						flags := payload[4]
						commandCode := uint32(payload[5])<<16 | uint32(payload[6])<<8 | uint32(payload[7])

						t.Logf("  Diameter: version=%d, length=%d, flags=0x%02x, commandCode=%d",
							version, length, flags, commandCode)

						if version != 1 {
							t.Errorf("Expected Diameter version 1, got %d", version)
						}
						if flags&0x80 == 0 {
							t.Errorf("Expected Request flag to be set (0x80)")
						}
					}
				}
			}
		}
	}

	t.Logf("Total packets captured: %d", packetCount)

	if !tcpHandshakeFound {
		t.Error("TCP handshake (SYN) not found in PCAP")
	}
	if !httpFound {
		t.Error("HTTP request not found in PCAP")
	}
	if !diameterFound {
		t.Error("Diameter message not found in PCAP")
	}

	// Verify minimum packet count (3 TCP handshake + 2 data packets)
	if packetCount < 5 {
		t.Errorf("Expected at least 5 packets, got %d", packetCount)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
