package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

type captureSocket struct {
	iface   string
	fd      int
	ifindex int
}

type tcpPacketSummary struct {
	Timestamp        time.Time
	Interface        string
	Ifindex          int
	HardwareType     uint16
	PacketType       uint8
	PacketTypeName   string
	SourceMAC        string
	DestinationMAC   string
	SourceIP         string
	DestinationIP    string
	SourcePort       uint16
	DestinationPort  uint16
	Flags            string
	Sequence         uint32
	Acknowledgement  uint32
	IPv4HeaderLength int
	IPv4TotalLength  int
	TCPHeaderLength  int
	TCPDataLength    int
	CapturedLength   int
}

type arpPacketSummary struct {
	Timestamp         time.Time
	Interface         string
	Ifindex           int
	HardwareType      uint16
	PacketType        uint8
	PacketTypeName    string
	EthernetSourceMAC string
	EthernetDestMAC   string
	Operation         string
	SourceMAC         string
	SourceIP          string
	TargetMAC         string
	TargetIP          string
	CapturedLength    int
}

type packetStatistics struct {
	Packets uint32
	Drops   uint32
}

const (
	solPacket           = 263
	packetStatisticsOpt = 6
)

func openPacketSocket(iface string) (int, int, error) {
	ifi, err := net.InterfaceByName(iface)
	if err != nil {
		return -1, 0, err
	}
	fd, err := syscall.Socket(syscall.AF_PACKET, syscall.SOCK_RAW, int(htons(syscall.ETH_P_ALL)))
	if err != nil {
		return -1, 0, err
	}
	if err := syscall.Bind(fd, &syscall.SockaddrLinklayer{Protocol: htons(syscall.ETH_P_ALL), Ifindex: ifi.Index}); err != nil {
		_ = syscall.Close(fd)
		return -1, 0, err
	}
	if err := setReceiveTimeout(fd); err != nil {
		_ = syscall.Close(fd)
		return -1, 0, fmt.Errorf("set receive timeout: %w", err)
	}
	return fd, ifi.Index, nil
}

func setReceiveTimeout(fd int) error {
	return syscall.SetsockoptTimeval(fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &syscall.Timeval{Sec: 0, Usec: 200000})
}

func openPacketSockets(ifaces []string) ([]captureSocket, error) {
	sockets := make([]captureSocket, 0, len(ifaces))
	for _, raw := range ifaces {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		fd, index, err := openPacketSocket(name)
		if err != nil {
			for _, socket := range sockets {
				_ = syscall.Close(socket.fd)
			}
			return nil, fmt.Errorf("open packet socket on %s: %w", name, err)
		}
		sockets = append(sockets, captureSocket{iface: name, fd: fd, ifindex: index})
	}
	if len(sockets) == 0 {
		return nil, errors.New("no packet interfaces were requested")
	}
	return sockets, nil
}

func htons(v uint16) uint16 { return v<<8 | v>>8 }

func decodeIPv4Packet(packet []byte) ([]byte, bool) {
	if len(packet) < 20 || packet[0]>>4 != 4 {
		return nil, false
	}
	headerLen := int(packet[0]&0x0f) * 4
	totalLen := int(binary.BigEndian.Uint16(packet[2:4]))
	if headerLen < 20 || totalLen < headerLen || totalLen > len(packet) {
		return nil, false
	}
	return packet[:totalLen], true
}

func decodeIPv4Frame(frame []byte) ([]byte, bool) {
	if len(frame) >= 14 && binary.BigEndian.Uint16(frame[12:14]) == syscall.ETH_P_IP {
		packet, ok, err := decodeIPv4FrameForHardware(frame, syscall.ARPHRD_ETHER)
		return packet, ok && err == nil
	}
	packet, ok, err := decodeIPv4FrameForHardware(frame, syscall.ARPHRD_NONE)
	return packet, ok && err == nil
}

func decodeIPv4FrameForHardware(frame []byte, hardwareType uint16) ([]byte, bool, error) {
	switch hardwareType {
	case syscall.ARPHRD_ETHER, syscall.ARPHRD_LOOPBACK:
		if len(frame) < 14 || binary.BigEndian.Uint16(frame[12:14]) != syscall.ETH_P_IP {
			return nil, false, nil
		}
		packet, ok := decodeIPv4Packet(frame[14:])
		return packet, ok, nil
	case syscall.ARPHRD_NONE:
		packet, ok := decodeIPv4Packet(frame)
		return packet, ok, nil
	default:
		return nil, false, fmt.Errorf("unsupported AF_PACKET hardware type %d", hardwareType)
	}
}

func summarizeTCPPacket(frame []byte, hardwareType uint16, packetType uint8, iface string, ifindex int, timestamp time.Time) (tcpPacketSummary, bool, error) {
	packet, ok, err := decodeIPv4FrameForHardware(frame, hardwareType)
	if err != nil || !ok {
		return tcpPacketSummary{}, ok, err
	}
	if len(packet) < 20 || packet[0]>>4 != 4 || packet[9] != 6 {
		return tcpPacketSummary{}, false, nil
	}
	ipHeaderLength := int(packet[0]&0x0f) * 4
	if ipHeaderLength < 20 || len(packet) < ipHeaderLength {
		return tcpPacketSummary{}, false, errors.New("invalid IPv4 header length")
	}
	fragment := binary.BigEndian.Uint16(packet[6:8])
	if fragment&0x1fff != 0 || fragment&0x2000 != 0 {
		return tcpPacketSummary{}, false, errors.New("fragmented IPv4 packet cannot be summarized as TCP")
	}
	if len(packet) < ipHeaderLength+20 {
		return tcpPacketSummary{}, false, errors.New("truncated TCP header")
	}
	sourceMAC, destinationMAC := ethernetMACs(frame, hardwareType)
	tcp := packet[ipHeaderLength:]
	tcpHeaderLength := int(tcp[12]>>4) * 4
	if tcpHeaderLength < 20 || len(tcp) < tcpHeaderLength {
		return tcpPacketSummary{}, false, errors.New("invalid TCP header length")
	}
	return tcpPacketSummary{
		Timestamp:        timestamp,
		Interface:        iface,
		Ifindex:          ifindex,
		HardwareType:     hardwareType,
		PacketType:       packetType,
		PacketTypeName:   packetTypeName(packetType),
		SourceMAC:        sourceMAC,
		DestinationMAC:   destinationMAC,
		SourceIP:         net.IP(packet[12:16]).String(),
		DestinationIP:    net.IP(packet[16:20]).String(),
		SourcePort:       binary.BigEndian.Uint16(tcp[0:2]),
		DestinationPort:  binary.BigEndian.Uint16(tcp[2:4]),
		Flags:            tcpFlags(tcp[13]),
		Sequence:         binary.BigEndian.Uint32(tcp[4:8]),
		Acknowledgement:  binary.BigEndian.Uint32(tcp[8:12]),
		IPv4HeaderLength: ipHeaderLength,
		IPv4TotalLength:  len(packet),
		TCPHeaderLength:  tcpHeaderLength,
		TCPDataLength:    len(tcp) - tcpHeaderLength,
		CapturedLength:   len(packet),
	}, true, nil
}

func ethernetMACs(frame []byte, hardwareType uint16) (string, string) {
	if hardwareType != syscall.ARPHRD_ETHER || len(frame) < 14 {
		return "-", "-"
	}
	return net.HardwareAddr(frame[6:12]).String(), net.HardwareAddr(frame[0:6]).String()
}

func summarizeARPPacket(frame []byte, hardwareType uint16, packetType uint8, iface string, ifindex int, timestamp time.Time) (arpPacketSummary, bool, error) {
	if hardwareType != syscall.ARPHRD_ETHER || len(frame) < 14 || binary.BigEndian.Uint16(frame[12:14]) != 0x0806 {
		return arpPacketSummary{}, false, nil
	}
	if len(frame) < 14+28 {
		return arpPacketSummary{}, false, errors.New("truncated Ethernet/IPv4 ARP frame")
	}
	arp := frame[14:]
	if binary.BigEndian.Uint16(arp[0:2]) != syscall.ARPHRD_ETHER || binary.BigEndian.Uint16(arp[2:4]) != syscall.ETH_P_IP || arp[4] != 6 || arp[5] != 4 {
		return arpPacketSummary{}, false, errors.New("unsupported Ethernet ARP address format")
	}
	operation := ""
	switch binary.BigEndian.Uint16(arp[6:8]) {
	case 1:
		operation = "REQUEST"
	case 2:
		operation = "REPLY"
	default:
		return arpPacketSummary{}, false, errors.New("unsupported Ethernet ARP operation")
	}
	return arpPacketSummary{
		Timestamp:         timestamp,
		Interface:         iface,
		Ifindex:           ifindex,
		HardwareType:      hardwareType,
		PacketType:        packetType,
		PacketTypeName:    packetTypeName(packetType),
		EthernetSourceMAC: net.HardwareAddr(frame[6:12]).String(),
		EthernetDestMAC:   net.HardwareAddr(frame[0:6]).String(),
		Operation:         operation,
		SourceMAC:         net.HardwareAddr(arp[8:14]).String(),
		SourceIP:          net.IP(arp[14:18]).String(),
		TargetMAC:         net.HardwareAddr(arp[18:24]).String(),
		TargetIP:          net.IP(arp[24:28]).String(),
		CapturedLength:    len(frame),
	}, true, nil
}

func decodeKernelTimestampData(data []byte) (time.Time, error) {
	if len(data) < 16 {
		return time.Time{}, errors.New("kernel timestamp payload is too short")
	}
	seconds := int64(binary.NativeEndian.Uint64(data[:8]))
	nanoseconds := int64(binary.NativeEndian.Uint64(data[8:16]))
	if nanoseconds < 0 || nanoseconds >= int64(time.Second) {
		return time.Time{}, errors.New("kernel timestamp nanoseconds out of range")
	}
	return time.Unix(seconds, nanoseconds), nil
}

func parseKernelTimestamp(control []byte) (time.Time, error) {
	messages, err := syscall.ParseSocketControlMessage(control)
	if err != nil {
		return time.Time{}, err
	}
	for _, message := range messages {
		if message.Header.Level != syscall.SOL_SOCKET || message.Header.Type != syscall.SCM_TIMESTAMPNS {
			continue
		}
		return decodeKernelTimestampData(message.Data)
	}
	return time.Time{}, errors.New("SCM_TIMESTAMPNS control message not found")
}

func readPacketStatistics(fd int) (packetStatistics, error) {
	stats := packetStatistics{}
	length := uint32(binary.Size(stats))
	_, _, errno := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		uintptr(fd),
		uintptr(solPacket),
		uintptr(packetStatisticsOpt),
		uintptr(unsafe.Pointer(&stats)),
		uintptr(unsafe.Pointer(&length)),
		0,
	)
	if errno != 0 {
		return packetStatistics{}, errno
	}
	if length != uint32(binary.Size(stats)) {
		return packetStatistics{}, fmt.Errorf("unexpected packet statistics size %d", length)
	}
	return stats, nil
}

func packetTypeName(packetType uint8) string {
	switch packetType {
	case 0:
		return "HOST"
	case 1:
		return "BROADCAST"
	case 2:
		return "MULTICAST"
	case 3:
		return "OTHERHOST"
	case 4:
		return "OUTGOING"
	default:
		return fmt.Sprintf("UNKNOWN_%d", packetType)
	}
}

func hasIP(ip net.IP, allowed []net.IP) bool {
	for _, candidate := range allowed {
		if ip.Equal(candidate) {
			return true
		}
	}
	return false
}

func tcpFlags(v byte) string {
	var flags []string
	for _, f := range []struct {
		mask byte
		name string
	}{{0x02, "SYN"}, {0x10, "ACK"}, {0x04, "RST"}, {0x01, "FIN"}, {0x08, "PSH"}, {0x20, "URG"}, {0x40, "ECE"}, {0x80, "CWR"}} {
		if v&f.mask != 0 {
			flags = append(flags, f.name)
		}
	}
	if len(flags) == 0 {
		return "-"
	}
	return strings.Join(flags, ",")
}

func capture(s captureSocket, allowed []net.IP, port uint16, deadline time.Time) error {
	buf := make([]byte, 65535)
	for time.Now().Before(deadline) {
		n, from, err := syscall.Recvfrom(s.fd, buf, 0)
		if err != nil {
			if err == syscall.EAGAIN || err == syscall.EWOULDBLOCK || err == syscall.EINTR {
				continue
			}
			return fmt.Errorf("read packet socket on %s: %w", s.iface, err)
		}
		linkAddress, ok := from.(*syscall.SockaddrLinklayer)
		if !ok {
			return fmt.Errorf("read packet socket on %s: unexpected address type %T", s.iface, from)
		}
		if linkAddress.Ifindex != s.ifindex {
			return fmt.Errorf("read packet socket on %s: ifindex changed from %d to %d", s.iface, s.ifindex, linkAddress.Ifindex)
		}
		arp, isARP, err := summarizeARPPacket(buf[:n], linkAddress.Hatype, linkAddress.Pkttype, s.iface, s.ifindex, time.Now().UTC())
		if err != nil {
			return fmt.Errorf("decode ARP packet on %s: %w", s.iface, err)
		}
		if isARP {
			if hasIP(net.ParseIP(arp.SourceIP), allowed) || hasIP(net.ParseIP(arp.TargetIP), allowed) {
				fmt.Printf("ARP time=%s iface=%s ifindex=%d packet_type=%s ethernet_src_mac=%s ethernet_dst_mac=%s operation=%s sender_mac=%s sender_ip=%s target_mac=%s target_ip=%s len=%d\n",
					arp.Timestamp.Format(time.RFC3339Nano), arp.Interface, arp.Ifindex, arp.PacketTypeName, arp.EthernetSourceMAC, arp.EthernetDestMAC, arp.Operation, arp.SourceMAC, arp.SourceIP, arp.TargetMAC, arp.TargetIP, arp.CapturedLength)
			}
			continue
		}
		ip, ok, err := decodeIPv4FrameForHardware(buf[:n], linkAddress.Hatype)
		if err != nil {
			return fmt.Errorf("decode packet on %s: %w", s.iface, err)
		}
		if !ok {
			continue
		}
		ihl := int(ip[0]&0x0f) * 4
		if ip[0]>>4 != 4 || ihl < 20 || len(ip) < ihl+20 || ip[9] != 6 {
			continue
		}
		src := net.IP(ip[12:16])
		dst := net.IP(ip[16:20])
		if !hasIP(src, allowed) && !hasIP(dst, allowed) {
			continue
		}
		tcp := ip[ihl:]
		sport := binary.BigEndian.Uint16(tcp[0:2])
		dport := binary.BigEndian.Uint16(tcp[2:4])
		if port != 0 && sport != port && dport != port {
			continue
		}
		sourceMAC, destinationMAC := ethernetMACs(buf[:n], linkAddress.Hatype)
		fmt.Printf("PACKET time=%s iface=%s src_mac=%s dst_mac=%s src=%s:%d dst=%s:%d flags=%s len=%d\n",
			time.Now().UTC().Format(time.RFC3339Nano), s.iface, sourceMAC, destinationMAC, src, sport, dst, dport, tcpFlags(tcp[13]), n)
	}
	return nil
}

type captureHooks struct {
	beforePublish func()
	beforeCollect func()
}

type captureResult struct {
	socket     captureSocket
	err        error
	statistics packetStatistics
	statsErr   error
}

func runCaptures(sockets []captureSocket, allowed []net.IP, port uint16, duration time.Duration) error {
	return runCapturesWithHooks(sockets, allowed, port, duration, captureHooks{})
}

func runCapturesWithHooks(sockets []captureSocket, allowed []net.IP, port uint16, duration time.Duration, hooks captureHooks) error {
	if len(sockets) == 0 {
		return errors.New("no packet sockets are ready")
	}
	deadline := time.Now().Add(duration)
	resultsBySocket := make(chan captureResult, len(sockets))
	for _, socket := range sockets {
		go func(s captureSocket) {
			result := captureResult{socket: s, err: capture(s, allowed, port, deadline)}
			result.statistics, result.statsErr = readPacketStatistics(s.fd)
			if hooks.beforePublish != nil {
				hooks.beforePublish()
			}
			resultsBySocket <- result
		}(socket)
	}
	if hooks.beforeCollect != nil {
		hooks.beforeCollect()
	}
	var captureErrors []error
	for range sockets {
		result := <-resultsBySocket
		if result.statsErr != nil {
			fmt.Printf("PACKET_STATISTICS iface=%s packets=unknown drops=unknown error=%q\n", result.socket.iface, result.statsErr.Error())
			captureErrors = append(captureErrors, fmt.Errorf("packet statistics on %s: %w", result.socket.iface, result.statsErr))
		} else {
			fmt.Printf("PACKET_STATISTICS iface=%s packets=%d drops=%d\n", result.socket.iface, result.statistics.Packets, result.statistics.Drops)
		}
		if result.err != nil {
			captureErrors = append(captureErrors, result.err)
		}
	}
	if len(captureErrors) != 0 {
		return fmt.Errorf("one or more packet captures failed: %w", errors.Join(captureErrors...))
	}
	return nil
}

func main() {
	ifacesArg := flag.String("ifaces", "", "comma-separated interfaces to capture")
	ipsArg := flag.String("ips", "", "comma-separated IPv4 endpoints to include")
	portArg := flag.Uint("port", 0, "TCP port filter; 0 captures all TCP ports for the selected IPs")
	durationArg := flag.Duration("duration", 15*time.Second, "capture duration")
	flag.Parse()

	var allowed []net.IP
	for _, raw := range strings.Split(*ipsArg, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		ip := net.ParseIP(raw).To4()
		if ip == nil {
			fmt.Fprintf(os.Stderr, "invalid IPv4 address %q\n", raw)
			os.Exit(2)
		}
		allowed = append(allowed, ip)
	}
	if len(allowed) == 0 {
		fmt.Fprintln(os.Stderr, "at least one IPv4 address is required")
		os.Exit(2)
	}
	if *portArg > 65535 || *durationArg <= 0 {
		fmt.Fprintln(os.Stderr, "invalid port or duration")
		os.Exit(2)
	}

	var ifaceNames []string
	for _, raw := range strings.Split(*ifacesArg, ",") {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		ifaceNames = append(ifaceNames, name)
	}
	sockets, err := openPacketSockets(ifaceNames)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CAPTURE_ERROR error=%q\n", err.Error())
		os.Exit(2)
	}
	for _, socket := range sockets {
		fmt.Printf("READY_BOUND iface=%s ifindex=%d\n", socket.iface, socket.ifindex)
	}
	fmt.Printf("CAPTURE_READY interfaces=%d duration=%s\n", len(sockets), *durationArg)

	captureErr := runCaptures(sockets, allowed, uint16(*portArg), *durationArg)
	for _, socket := range sockets {
		_ = syscall.Close(socket.fd)
	}
	if captureErr != nil {
		fmt.Fprintf(os.Stderr, "CAPTURE_FAILED error=%q\n", captureErr.Error())
		os.Exit(1)
	}
	fmt.Println("CAPTURE_DONE")
}
