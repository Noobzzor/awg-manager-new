package main

import (
	"bytes"
	"encoding/binary"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestDecodeIPv4FrameForHardwareAcceptsRawL3(t *testing.T) {
	packet := sampleIPv4TCPPacket()
	got, ok, err := decodeIPv4FrameForHardware(packet, syscall.ARPHRD_NONE)
	if err != nil {
		t.Fatalf("decodeIPv4FrameForHardware returned error: %v", err)
	}
	if !ok || !bytes.Equal(got, packet) {
		t.Fatal("raw L3 frame was not decoded as the original IPv4 packet")
	}
}

func TestDecodeIPv4FrameForHardwareAcceptsEthernet(t *testing.T) {
	packet := sampleIPv4TCPPacket()
	frame := make([]byte, 14+len(packet))
	copy(frame[0:6], []byte{0x02, 0, 0, 0, 0, 1})
	copy(frame[6:12], []byte{0x02, 0, 0, 0, 0, 2})
	frame[12] = 0x08
	frame[13] = 0x00
	copy(frame[14:], packet)
	got, ok, err := decodeIPv4FrameForHardware(frame, syscall.ARPHRD_ETHER)
	if err != nil {
		t.Fatalf("decodeIPv4FrameForHardware returned error: %v", err)
	}
	if !ok || !bytes.Equal(got, packet) {
		t.Fatal("Ethernet IPv4 frame was not decoded from its 14-byte header")
	}
}

func TestDecodeIPv4FrameForHardwareDoesNotConfuseMacWithIPv4(t *testing.T) {
	packet := sampleIPv4TCPPacket()
	frame := make([]byte, 14+len(packet))
	frame[0] = 0x45
	frame[2] = 0
	frame[3] = byte(len(packet))
	frame[8] = 64
	frame[9] = 6
	frame[12] = 0x08
	frame[13] = 0x00
	copy(frame[14:], packet)
	got, ok, err := decodeIPv4FrameForHardware(frame, syscall.ARPHRD_ETHER)
	if err != nil {
		t.Fatalf("decodeIPv4FrameForHardware returned error: %v", err)
	}
	if !ok || !bytes.Equal(got, packet) {
		t.Fatal("Ethernet frame with IPv4-like MAC was misread as raw IPv4")
	}
}

func TestDecodeIPv4FrameForHardwareRejectsUnknownLinkType(t *testing.T) {
	_, _, err := decodeIPv4FrameForHardware(sampleIPv4TCPPacket(), 9999)
	if err == nil {
		t.Fatal("unknown hardware type was guessed instead of rejected")
	}
}

func sampleTCPPacketWithPayload() []byte {
	packet := make([]byte, 43)
	copy(packet, sampleIPv4TCPPacket())
	binary.BigEndian.PutUint16(packet[2:4], uint16(len(packet)))
	binary.BigEndian.PutUint16(packet[20:22], 42424)
	binary.BigEndian.PutUint16(packet[22:24], 8080)
	binary.BigEndian.PutUint32(packet[24:28], 0x01020304)
	binary.BigEndian.PutUint32(packet[28:32], 0x11223344)
	packet[33] = 0x12 // SYN|ACK
	copy(packet[40:], []byte{0xaa, 0xbb, 0xcc})
	return packet
}

func TestSummarizeTCPPacketIncludesDirectionAndSequence(t *testing.T) {
	packet := sampleTCPPacketWithPayload()
	timestamp := time.Unix(1_700_000_000, 123_456_789).UTC()
	got, ok, err := summarizeTCPPacket(packet, syscall.ARPHRD_NONE, 4, "awg0", 7, timestamp)
	if err != nil {
		t.Fatalf("summarizeTCPPacket returned error: %v", err)
	}
	if !ok {
		t.Fatal("summarizeTCPPacket rejected a valid TCP packet")
	}
	if !got.Timestamp.Equal(timestamp) || got.Interface != "awg0" || got.Ifindex != 7 || got.HardwareType != syscall.ARPHRD_NONE {
		t.Fatalf("packet identity/timestamp fields = %+v", got)
	}
	if got.PacketType != 4 || got.PacketTypeName != "OUTGOING" {
		t.Fatalf("packet type = %d/%s, want outgoing (4)", got.PacketType, got.PacketTypeName)
	}
	if got.SourceIP != "10.66.0.3" || got.DestinationIP != "203.0.113.11" || got.SourcePort != 42424 || got.DestinationPort != 8080 {
		t.Fatalf("tuple = %+v", got)
	}
	if got.Flags != "SYN,ACK" || got.Sequence != 0x01020304 || got.Acknowledgement != 0x11223344 {
		t.Fatalf("TCP fields = %+v", got)
	}
	if got.IPv4HeaderLength != 20 || got.IPv4TotalLength != len(packet) || got.TCPHeaderLength != 20 || got.TCPDataLength != 3 || got.CapturedLength != len(packet) {
		t.Fatalf("packet lengths = %+v", got)
	}
}

func TestSummarizeTCPPacketIncludesEthernetMACs(t *testing.T) {
	ipPacket := sampleTCPPacketWithPayload()
	frame := append([]byte{
		0x02, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, // destination MAC
		0x02, 0x11, 0x22, 0x33, 0x44, 0x55, // source MAC
		0x08, 0x00,
	}, ipPacket...)
	got, ok, err := summarizeTCPPacket(frame, syscall.ARPHRD_ETHER, 4, "eth0", 2, time.Now())
	if err != nil || !ok {
		t.Fatalf("summarizeTCPPacket Ethernet frame = ok:%v err:%v", ok, err)
	}
	if got.SourceMAC != "02:11:22:33:44:55" || got.DestinationMAC != "02:aa:bb:cc:dd:ee" {
		t.Fatalf("Ethernet MACs = %q -> %q", got.SourceMAC, got.DestinationMAC)
	}
}

func TestSummarizeARPPacketDecodesRequestAndReply(t *testing.T) {
	makeFrame := func(opcode uint16, ethSource, ethDestination, senderMAC, senderIP, targetMAC, targetIP []byte) []byte {
		frame := make([]byte, 14+28)
		copy(frame[0:6], ethDestination)
		copy(frame[6:12], ethSource)
		binary.BigEndian.PutUint16(frame[12:14], 0x0806)
		arp := frame[14:]
		binary.BigEndian.PutUint16(arp[0:2], 1)
		binary.BigEndian.PutUint16(arp[2:4], 0x0800)
		arp[4], arp[5] = 6, 4
		binary.BigEndian.PutUint16(arp[6:8], opcode)
		copy(arp[8:14], senderMAC)
		copy(arp[14:18], senderIP)
		copy(arp[18:24], targetMAC)
		copy(arp[24:28], targetIP)
		return frame
	}

	gwMAC := []byte{0x02, 0, 0, 0, 0, 1}
	sinkMAC := []byte{0x02, 0, 0, 0, 0, 2}
	broadcastMAC := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	zeroMAC := []byte{0, 0, 0, 0, 0, 0}
	gwIP := []byte{172, 23, 0, 4}
	sinkIP := []byte{172, 23, 0, 2}

	tests := []struct {
		name            string
		frame           []byte
		wantOperation   string
		wantSourceMAC   string
		wantSourceIP    string
		wantTargetMAC   string
		wantTargetIP    string
		wantEtherSource string
	}{
		{
			name:            "request",
			frame:           makeFrame(1, gwMAC, broadcastMAC, gwMAC, gwIP, zeroMAC, sinkIP),
			wantOperation:   "REQUEST",
			wantSourceMAC:   "02:00:00:00:00:01",
			wantSourceIP:    "172.23.0.4",
			wantTargetMAC:   "00:00:00:00:00:00",
			wantTargetIP:    "172.23.0.2",
			wantEtherSource: "02:00:00:00:00:01",
		},
		{
			name:            "reply",
			frame:           makeFrame(2, sinkMAC, gwMAC, sinkMAC, sinkIP, gwMAC, gwIP),
			wantOperation:   "REPLY",
			wantSourceMAC:   "02:00:00:00:00:02",
			wantSourceIP:    "172.23.0.2",
			wantTargetMAC:   "02:00:00:00:00:01",
			wantTargetIP:    "172.23.0.4",
			wantEtherSource: "02:00:00:00:00:02",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok, err := summarizeARPPacket(tt.frame, syscall.ARPHRD_ETHER, 0, "eth0", 2, time.Unix(1, 2))
			if err != nil || !ok {
				t.Fatalf("summarizeARPPacket() = ok:%v err:%v", ok, err)
			}
			if got.Operation != tt.wantOperation || got.Interface != "eth0" || got.Ifindex != 2 {
				t.Fatalf("ARP identity = %+v", got)
			}
			if got.SourceMAC != tt.wantSourceMAC || got.SourceIP != tt.wantSourceIP || got.TargetMAC != tt.wantTargetMAC || got.TargetIP != tt.wantTargetIP {
				t.Fatalf("ARP addresses = %+v", got)
			}
			if got.EthernetSourceMAC != tt.wantEtherSource || got.CapturedLength != len(tt.frame) {
				t.Fatalf("Ethernet fields = %+v", got)
			}
		})
	}
}

func TestSummarizeTCPPacketRejectsMalformedTCPHeaderAndFragments(t *testing.T) {
	badTCP := sampleIPv4TCPPacket()
	badTCP[32] = 0x40 // TCP data offset below the minimum header length.
	if _, _, err := summarizeTCPPacket(badTCP, syscall.ARPHRD_NONE, 0, "awg0", 7, time.Now()); err == nil {
		t.Fatal("malformed TCP data offset was accepted")
	}
	fragment := sampleIPv4TCPPacket()
	fragment[6] = 0x20 // IPv4 more-fragments flag.
	if _, _, err := summarizeTCPPacket(fragment, syscall.ARPHRD_NONE, 0, "awg0", 7, time.Now()); err == nil {
		t.Fatal("fragmented TCP packet was silently treated as a complete segment")
	}
}

func TestDecodeKernelTimestampData(t *testing.T) {
	data := make([]byte, 16)
	binary.NativeEndian.PutUint64(data[:8], 1_700_000_000)
	binary.NativeEndian.PutUint64(data[8:], 123_456_789)
	got, err := decodeKernelTimestampData(data)
	if err != nil {
		t.Fatalf("decodeKernelTimestampData returned error: %v", err)
	}
	if got.Unix() != 1_700_000_000 || got.Nanosecond() != 123_456_789 {
		t.Fatalf("decoded timestamp = %s", got.Format(time.RFC3339Nano))
	}
	if _, err := decodeKernelTimestampData([]byte{1, 2, 3}); err == nil {
		t.Fatal("short kernel timestamp payload was accepted")
	}
}

func TestReadPacketStatisticsOnLoopbackSocket(t *testing.T) {
	fd, _, err := openPacketSocket("lo")
	if err != nil {
		t.Fatalf("open loopback packet socket: %v", err)
	}
	defer syscall.Close(fd)
	stats, err := readPacketStatistics(fd)
	if err != nil {
		t.Fatalf("read packet statistics: %v", err)
	}
	if got := binary.Size(stats); got != 8 {
		t.Fatalf("tpacket_stats layout size=%d, want 8", got)
	}
	if _, err := readPacketStatistics(-1); err == nil {
		t.Fatal("invalid packet socket descriptor returned packet statistics")
	}
}

func TestParseKernelTimestampControlMessage(t *testing.T) {
	data := make([]byte, 16)
	binary.NativeEndian.PutUint64(data[:8], 1_700_000_000)
	binary.NativeEndian.PutUint64(data[8:], 123_456_789)
	oob := make([]byte, syscall.CmsgSpace(len(data)))
	header := (*syscall.Cmsghdr)(unsafe.Pointer(&oob[0]))
	header.SetLen(syscall.CmsgLen(len(data)))
	header.Level = syscall.SOL_SOCKET
	header.Type = syscall.SCM_TIMESTAMPNS
	copy(oob[syscall.CmsgLen(0):], data)
	got, err := parseKernelTimestamp(oob)
	if err != nil {
		t.Fatalf("parseKernelTimestamp returned error: %v", err)
	}
	if got.Unix() != 1_700_000_000 || got.Nanosecond() != 123_456_789 {
		t.Fatalf("parsed kernel timestamp = %s", got.Format(time.RFC3339Nano))
	}
}

func TestPacketTypeNames(t *testing.T) {
	for packetType, want := range map[uint8]string{0: "HOST", 1: "BROADCAST", 2: "MULTICAST", 3: "OTHERHOST", 4: "OUTGOING"} {
		if got := packetTypeName(packetType); got != want {
			t.Errorf("packetTypeName(%d)=%q, want %q", packetType, got, want)
		}
	}
	if got := packetTypeName(255); got != "UNKNOWN_255" {
		t.Fatalf("unknown packet type label=%q", got)
	}
}
