package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

var captureWithError func(captureSocket, []net.IP, uint16, time.Time) error = capture

func TestCaptureReturnsFatalReadError(t *testing.T) {
	err := captureWithError(
		captureSocket{iface: "lo", fd: -1},
		[]net.IP{net.ParseIP("10.66.0.3").To4()},
		0,
		time.Now().Add(time.Second),
	)
	if !errors.Is(err, syscall.EBADF) {
		t.Fatalf("capture error = %v, want syscall.EBADF", err)
	}
}

func TestSetReceiveTimeoutPropagatesFailure(t *testing.T) {
	if err := setReceiveTimeout(-1); err == nil {
		t.Fatal("setReceiveTimeout(-1) returned nil")
	}
}

func TestOpenPacketSocketsFailsAtomically(t *testing.T) {
	fd, _, err := openPacketSocket("lo")
	if err != nil {
		t.Fatalf("open loopback prerequisite: %v", err)
	}
	_ = syscall.Close(fd)
	sockets, err := openPacketSockets([]string{"lo", "awg-no-such-interface"})
	if err == nil {
		t.Fatal("openPacketSockets unexpectedly accepted a missing requested interface")
	}
	if len(sockets) != 0 {
		t.Fatalf("openPacketSockets returned %d partial sockets after failure", len(sockets))
	}
}

func TestRunCapturesCollectsBeforeCompleting(t *testing.T) {
	if os.Getenv("AWG_CAPTURE_RACE_CHILD") == "1" {
		published := make(chan struct{})
		release := make(chan struct{})
		hooks := captureHooks{
			beforePublish: func() {
				close(published)
				<-release
			},
			beforeCollect: func() {
				<-published
				close(release)
			},
		}
		err := runCapturesWithHooks(
			[]captureSocket{{iface: "lo", fd: -1}},
			[]net.IP{net.ParseIP("10.66.0.3").To4()},
			0,
			time.Second,
			hooks,
		)
		if !errors.Is(err, syscall.EBADF) {
			t.Fatalf("runCapturesWithHooks returned %v, want syscall.EBADF", err)
		}
		fmt.Println("RACE_CHILD_PASS")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunCapturesCollectsBeforeCompleting$")
	cmd.Env = append(os.Environ(), "AWG_CAPTURE_RACE_CHILD=1")
	output, err := cmd.CombinedOutput()
	if err == nil && bytes.Contains(output, []byte("RACE_CHILD_PASS")) {
		return
	}
	if bytes.Contains(output, []byte("send on closed channel")) || bytes.Contains(output, []byte("runCapturesWithHooks returned")) {
		fmt.Printf("RACE_RED_EXPECTED: %s\n", output)
		t.Fatal("collector completed before the worker result was safely published")
	}
	t.Fatalf("race child failed unexpectedly: err=%v output=%s", err, strings.TrimSpace(string(output)))
}

func sampleIPv4TCPPacket() []byte {
	packet := make([]byte, 40)
	packet[0] = 0x45
	packet[2] = 0
	packet[3] = byte(len(packet))
	packet[8] = 64
	packet[9] = 6
	copy(packet[12:16], []byte{10, 66, 0, 3})
	copy(packet[16:20], []byte{203, 0, 113, 11})
	packet[20] = 0x30
	packet[32] = 0x50
	packet[33] = 0x02
	return packet
}

func TestDecodeIPv4FrameAcceptsRawL3Packet(t *testing.T) {
	packet := sampleIPv4TCPPacket()
	got, ok := decodeIPv4Frame(packet)
	if !ok {
		t.Fatal("decodeIPv4Frame rejected a valid raw IPv4/TCP packet")
	}
	if !bytes.Equal(got, packet) {
		t.Fatalf("decoded raw packet differs: got %d bytes, want %d", len(got), len(packet))
	}
}

func TestDecodeIPv4FrameAcceptsEthernetPacket(t *testing.T) {
	packet := sampleIPv4TCPPacket()
	frame := make([]byte, 14+len(packet))
	copy(frame[0:6], []byte{0x02, 0, 0, 0, 0, 1})
	copy(frame[6:12], []byte{0x02, 0, 0, 0, 0, 2})
	frame[12] = 0x08
	frame[13] = 0x00
	copy(frame[14:], packet)
	got, ok := decodeIPv4Frame(frame)
	if !ok {
		t.Fatal("decodeIPv4Frame rejected an Ethernet-framed IPv4/TCP packet")
	}
	if !bytes.Equal(got, packet) {
		t.Fatalf("decoded Ethernet payload differs: got %d bytes, want %d", len(got), len(packet))
	}
}

func TestDecodeIPv4FrameDoesNotTreatIPv4LikeMACAsRawPacket(t *testing.T) {
	packet := sampleIPv4TCPPacket()
	frame := make([]byte, 14+len(packet))
	frame[0] = 0x45 // Deliberately looks like an IPv4 version/IHL byte.
	frame[2] = 0
	frame[3] = byte(len(packet))
	frame[8] = 64
	frame[9] = 6
	frame[12] = 0x08
	frame[13] = 0x00
	copy(frame[14:], packet)
	got, ok := decodeIPv4Frame(frame)
	if !ok {
		t.Fatal("decodeIPv4Frame rejected the Ethernet IPv4 frame")
	}
	if !bytes.Equal(got, packet) {
		t.Fatal("decodeIPv4Frame treated an IPv4-like Ethernet MAC as a raw IPv4 header")
	}
}
