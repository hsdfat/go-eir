package testutil

import (
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

// PCAPWriter captures network traffic and writes to PCAP file
type PCAPWriter struct {
	filename string
	file     *os.File
	writer   *pcapgo.Writer
	mu       sync.Mutex
	closed   bool
}

// NewPCAPWriter creates a new PCAP writer
func NewPCAPWriter(filename string) (*PCAPWriter, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create PCAP file: %w", err)
	}

	writer := pcapgo.NewWriter(file)
	if err := writer.WriteFileHeader(65536, layers.LinkTypeEthernet); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to write PCAP header: %w", err)
	}

	return &PCAPWriter{
		filename: filename,
		file:     file,
		writer:   writer,
	}, nil
}

// WritePacket writes a packet to the PCAP file
func (p *PCAPWriter) WritePacket(data []byte, srcIP, dstIP net.IP, srcPort, dstPort uint16, isInbound bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return fmt.Errorf("PCAP writer is closed")
	}

	// Build Ethernet layer
	eth := &layers.Ethernet{
		SrcMAC:       net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		DstMAC:       net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x02},
		EthernetType: layers.EthernetTypeIPv4,
	}

	// Build IP layer
	ip := &layers.IPv4{
		Version:  4,
		TTL:      64,
		Protocol: layers.IPProtocolTCP,
		SrcIP:    srcIP,
		DstIP:    dstIP,
	}

	// Build TCP layer
	tcp := &layers.TCP{
		SrcPort: layers.TCPPort(srcPort),
		DstPort: layers.TCPPort(dstPort),
		Seq:     uint32(time.Now().Unix()),
		Ack:     0,
		PSH:     true,
		Window:  65535,
	}
	tcp.SetNetworkLayerForChecksum(ip)

	// Create packet buffer
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	// Serialize packet
	if err := gopacket.SerializeLayers(buf, opts, eth, ip, tcp, gopacket.Payload(data)); err != nil {
		return fmt.Errorf("failed to serialize packet: %w", err)
	}

	// Write packet to PCAP
	captureInfo := gopacket.CaptureInfo{
		Timestamp:     time.Now(),
		CaptureLength: len(buf.Bytes()),
		Length:        len(buf.Bytes()),
	}

	if err := p.writer.WritePacket(captureInfo, buf.Bytes()); err != nil {
		return fmt.Errorf("failed to write packet: %w", err)
	}

	return nil
}

// WriteHTTPPacket writes an HTTP packet to the PCAP file
func (p *PCAPWriter) WriteHTTPPacket(data []byte, srcIP, dstIP net.IP, srcPort, dstPort uint16, isRequest bool) error {
	return p.WritePacket(data, srcIP, dstIP, srcPort, dstPort, !isRequest)
}

// WriteDiameterPacket writes a Diameter packet to the PCAP file
func (p *PCAPWriter) WriteDiameterPacket(data []byte, srcIP, dstIP net.IP, srcPort, dstPort uint16) error {
	return p.WritePacket(data, srcIP, dstIP, srcPort, dstPort, false)
}

// Close closes the PCAP writer
func (p *PCAPWriter) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil
	}

	p.closed = true
	if err := p.file.Close(); err != nil {
		return fmt.Errorf("failed to close PCAP file: %w", err)
	}

	fmt.Printf("PCAP file written: %s\n", p.filename)
	return nil
}

// CaptureConnection captures traffic from a network connection
type CaptureConnection struct {
	net.Conn
	pcapWriter *PCAPWriter
	srcIP      net.IP
	dstIP      net.IP
	srcPort    uint16
	dstPort    uint16
}

// NewCaptureConnection wraps a connection to capture traffic
func NewCaptureConnection(conn net.Conn, pcapWriter *PCAPWriter) *CaptureConnection {
	srcAddr := conn.LocalAddr().(*net.TCPAddr)
	dstAddr := conn.RemoteAddr().(*net.TCPAddr)

	return &CaptureConnection{
		Conn:       conn,
		pcapWriter: pcapWriter,
		srcIP:      srcAddr.IP,
		dstIP:      dstAddr.IP,
		srcPort:    uint16(srcAddr.Port),
		dstPort:    uint16(dstAddr.Port),
	}
}

// Read captures read data
func (c *CaptureConnection) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if n > 0 && c.pcapWriter != nil {
		// Capture inbound packet (from dst to src)
		_ = c.pcapWriter.WritePacket(b[:n], c.dstIP, c.srcIP, c.dstPort, c.srcPort, true)
	}
	return n, err
}

// Write captures write data
func (c *CaptureConnection) Write(b []byte) (n int, err error) {
	if len(b) > 0 && c.pcapWriter != nil {
		// Capture outbound packet (from src to dst)
		_ = c.pcapWriter.WritePacket(b, c.srcIP, c.dstIP, c.srcPort, c.dstPort, false)
	}
	n, err = c.Conn.Write(b)
	return n, err
}
