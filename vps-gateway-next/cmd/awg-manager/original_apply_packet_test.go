package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const originalPacketProbe = "/artifacts/policy-client-probe"
const originalPacketTarget = "203.0.113.2:51980"

type originalPacketFixture struct {
	t       *testing.T
	ctx     context.Context
	mu      sync.Mutex
	hits    map[string]int
	private string
	public  string
	server  string
}

func startOriginalPacketFixture(t *testing.T, ctx context.Context) *originalPacketFixture {
	t.Helper()
	if os.Getpid() != 1 {
		t.Fatal("packet fixture must be the container entry process")
	}
	ifaces, err := net.Interfaces()
	if err != nil || len(ifaces) != 1 || ifaces[0].Name != "lo" {
		t.Fatal("packet fixture requires a fresh isolated network")
	}
	f := &originalPacketFixture{t: t, ctx: ctx, hits: make(map[string]int)}
	sink := exec.CommandContext(ctx, originalPacketProbe, "sink")
	out, err := sink.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = sink.Start(); err != nil {
		t.Fatal("start controlled sink", err)
	}
	ready := make(chan struct{}, 1)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		scanner := bufio.NewScanner(out)
		for scanner.Scan() {
			var receipt struct {
				Ready  bool
				Marker string `json:"marker_sha256"`
			}
			if json.Unmarshal(scanner.Bytes(), &receipt) != nil {
				continue
			}
			if receipt.Ready {
				select {
				case ready <- struct{}{}:
				default:
				}
			}
			if receipt.Marker != "" {
				f.mu.Lock()
				f.hits[receipt.Marker]++
				f.mu.Unlock()
			}
		}
	}()
	t.Cleanup(func() {
		_ = sink.Process.Signal(syscall.SIGTERM)
		_ = sink.Wait()
		<-drained
	})
	select {
	case <-ready:
	case <-time.After(3 * time.Second):
		t.Fatal("controlled sink did not start")
	}
	for _, args := range [][]string{
		{"address", "add", "203.0.113.1/32", "dev", "fixture0"},
		{"link", "set", "fixture0", "up"},
		{"route", "add", "203.0.113.2/32", "dev", "fixture0"},
		{"route", "add", "default", "dev", "fixture0"},
	} {
		if err := exec.CommandContext(ctx, "/usr/sbin/ip", args...).Run(); err != nil {
			t.Fatal("configure controlled sink route", err)
		}
	}
	c, err := net.DialTimeout("tcp", originalPacketTarget, time.Second)
	if err != nil {
		t.Fatal("controlled sink baseline dial failed", err)
	}
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(time.Second))
	marker := make([]byte, 32)
	if _, err := rand.Read(marker); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Write(marker); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, len(marker))
	if _, err := io.ReadFull(c, reply); err != nil || !bytes.Equal(reply, marker) {
		t.Fatal("controlled sink baseline echo failed", err)
	}
	return f
}

func (f *originalPacketFixture) awg(input string, args ...string) string {
	f.t.Helper()
	cmd := exec.CommandContext(f.ctx, "/artifacts/awg", args...)
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if err != nil {
		f.t.Fatal("controlled AWG operation failed", args[0], err)
	}
	return strings.TrimSpace(string(out))
}

func (f *originalPacketFixture) provision() {
	f.t.Helper()
	// Provision only the synthetic test peer in the owned network namespace.
	// This fixture exercises routing, independently of client management.
	f.private = f.awg("", "genkey")
	f.public = f.awg(f.private+"\n", "pubkey")
	f.server = f.awg("", "show", "m2oa0", "public-key")
	f.awg("", "set", "m2oa0", "peer", f.public, "allowed-ips", "10.77.0.2/32")
}

func (f *originalPacketFixture) peerCounter(field string) uint64 {
	f.t.Helper()
	for _, line := range strings.Split(f.awg("", "show", "m2oa0", field), "\n") {
		columns := strings.Fields(line)
		if len(columns) >= 2 && columns[0] == f.public {
			n, err := strconv.ParseUint(columns[1], 10, 64)
			if err != nil {
				f.t.Fatal("invalid synthetic peer counter", field)
			}
			return n
		}
	}
	f.t.Fatal("synthetic peer missing", field)
	return 0
}

func (f *originalPacketFixture) probe(action string) {
	f.t.Helper()
	marker := make([]byte, 32)
	if _, err := rand.Read(marker); err != nil {
		f.t.Fatal(err)
	}
	f.probePayload(action, marker)
}

func (f *originalPacketFixture) probePayload(action string, marker []byte) {
	f.t.Helper()
	if len(marker) != 32 {
		f.t.Fatal("packet fixture expects a 32-byte payload")
	}
	private, err := base64.StdEncoding.DecodeString(f.private)
	if err != nil || len(private) != 32 {
		f.t.Fatal("invalid synthetic client identity")
	}
	server, err := base64.StdEncoding.DecodeString(f.server)
	if err != nil || len(server) != 32 {
		f.t.Fatal("invalid synthetic server identity")
	}
	beforeRx := f.peerCounter("transfer")
	beforeHandshake := f.peerCounter("latest-handshakes")
	for uint64(time.Now().Unix()) <= beforeHandshake {
		time.Sleep(20 * time.Millisecond)
	}
	markerSHA := fmt.Sprintf("%x", sha256.Sum256(marker))
	payload, err := json.Marshal(map[string]any{
		"PrivateKey": hex.EncodeToString(private), "ServerPublicKey": hex.EncodeToString(server),
		"Endpoint": "127.0.0.1:51820", "Marker": hex.EncodeToString(marker), "AWG": true,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(f.ctx, 9*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, originalPacketProbe, "outline-client")
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.Output()
	if err != nil {
		f.t.Fatal("controlled AWG client failed", action, err)
	}
	var result struct {
		Delivered bool
		Marker    string `json:"marker_sha256"`
	}
	if err := json.Unmarshal(out, &result); err != nil || result.Marker != markerSHA {
		f.t.Fatal("invalid controlled AWG client receipt", action)
	}
	f.mu.Lock()
	hits := f.hits[markerSHA]
	f.mu.Unlock()
	expectedHits := 1
	if action == "block" {
		expectedHits = 0
	}
	afterRx := f.peerCounter("transfer")
	afterHandshake := f.peerCounter("latest-handshakes")
	if afterRx <= beforeRx || afterHandshake <= beforeHandshake || result.Delivered != (action == "direct") || hits != expectedHits {
		f.t.Fatalf("AWG packet outcome %s: delivered=%t sink_hits=%d receive_increased=%t fresh_handshake=%t",
			action, result.Delivered, hits, afterRx > beforeRx, afterHandshake > beforeHandshake)
	}
	f.t.Logf("AWG packet %s: fresh handshake, receive counter increased, sink hits=%d", action, hits)
}
