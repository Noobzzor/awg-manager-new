//go:build linux

package ingress

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This is manager owner/Registry packet evidence, not full-app HTTP or policy E2E.
// The ChannelTUN fixture owns no OS client address, so replies must cross AWG.
func TestM2ActualAWGManagerPackets(t *testing.T) {
	if os.Getenv("AWGM_REAL_MANAGER_PACKETS") != "1" {
		t.Skip("explicit isolated packet fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
	defer cancel()
	name := "m2pkt0"
	tool := "/artifacts/awg"
	params := testedAWGParameters()
	m := NewUserspaceAWGInterface(UserspaceAWGConfig{RuntimeBinary: "/input/amneziawg-go", ToolBinary: tool, IPBinary: "/input/ip", InterfaceName: name, ListenPort: 51820, Address: "10.77.0.1/30", KeyDirectory: filepath.Join(t.TempDir(), "keys"), Parameters: params})
	t.Cleanup(func() {
		if err := m.Close(context.Background()); err != nil {
			t.Error("packet owner cleanup incomplete")
		}
	})
	if err := m.Preflight(ctx); err != nil {
		t.Fatal(err)
	}
	privateKey, serverPublic, err := GenerateKeyPairWithBinary(ctx, tool)
	if err != nil {
		t.Fatal("server key creation failed")
	}
	if err := m.SetPrivateKey(privateKey); err != nil {
		t.Fatal("server identity refused")
	}
	if err := m.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	iface, err := net.InterfaceByName(name)
	if err != nil || iface.Flags&net.FlagUp == 0 {
		t.Fatal("actual server TUN not up")
	}
	addrs, err := iface.Addrs()
	if err != nil {
		t.Fatal("address readback failed")
	}
	hasAddress := false
	for _, a := range addrs {
		if a.String() == "10.77.0.1/30" {
			hasAddress = true
		}
	}
	if !hasAddress {
		t.Fatal("actual server address missing")
	}
	allocator, err := NewAllocator(netip.MustParsePrefix("10.77.0.0/30"))
	if err != nil {
		t.Fatal(err)
	}
	if err := allocator.ReserveAddress(netip.MustParseAddr("10.77.0.1")); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(allocator, func(string) (string, string, error) { return GenerateKeyPairWithBinary(ctx, tool) })
	if err := registry.UseAWGParameters(params); err != nil {
		t.Fatal(err)
	}
	client, err := registry.Create("packet-client", "Synthetic personal device")
	if err != nil {
		t.Fatal("client creation failed")
	}
	if client.Address.String() != "10.77.0.2" {
		t.Fatal("client fixture address differs")
	}
	profile, err := registry.RenderClientConfig(client.ID, serverPublic, "127.0.0.1:51821")
	if err != nil {
		t.Fatal("actual export failed")
	}
	var exportedPrivate string
	for _, line := range strings.Split(profile, "\n") {
		if k, v, ok := strings.Cut(line, "="); ok && strings.TrimSpace(k) == "PrivateKey" {
			exportedPrivate = strings.TrimSpace(v)
		}
	}
	privBytes, err := base64.StdEncoding.DecodeString(exportedPrivate)
	if err != nil || len(privBytes) != 32 {
		t.Fatal("exported identity invalid")
	}
	serverBytes, err := base64.StdEncoding.DecodeString(serverPublic)
	if err != nil || len(serverBytes) != 32 {
		t.Fatal("server identity invalid")
	}
	type clientProof struct {
		Delivered bool  `json:"delivered"`
		Handshake int64 `json:"handshake"`
		Rx, Tx    int64
		MarkerSHA string `json:"marker_sha256"`
	}
	type wireProof struct {
		AWGInit, AWGResponse, ClientTransport, ServerTransport, StandardInit, ForwardErrors int
		PlaintextMarker                                                                     bool
	}
	phases := []map[string]any{}
	runPhase := func(label string, want bool) {
		if err := registry.SyncClient(ctx, client.ID, "", m); err != nil {
			t.Fatal(err)
		}
		before, err := m.ReadPeerStatus(ctx, client.PublicKey)
		if want {
			if err != nil || before.Handshaken || before.RXBytes != 0 || before.TXBytes != 0 {
				t.Fatal("fresh peer status mismatch")
			}
		} else if !errors.Is(err, ErrPeerNotFound) {
			t.Fatal("disabled/revoked peer still present")
		}
		marker := make([]byte, 32)
		if _, err := rand.Read(marker); err != nil {
			t.Fatal(err)
		}
		relay, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 51821})
		if err != nil {
			t.Fatal("relay setup failed")
		}
		done := make(chan wireProof, 1)
		go func() {
			stats := wireProof{}
			buf := make([]byte, 65536)
			var target *net.UDPAddr
			for {
				n, from, err := relay.ReadFromUDP(buf)
				if err != nil {
					done <- stats
					return
				}
				packet := buf[:n]
				server := from.Port == 51820
				if bytes.Contains(packet, marker) {
					stats.PlaintextMarker = true
				}
				if len(packet) >= 20 {
					h := binary.LittleEndian.Uint32(packet[16:20])
					if server {
						if h == params.H2 {
							stats.AWGResponse++
						}
						if h == params.H4 {
							stats.ServerTransport++
						}
					} else {
						if h == params.H1 {
							stats.AWGInit++
						}
						if h == params.H4 {
							stats.ClientTransport++
						}
					}
				}
				if !server && len(packet) == 148 && binary.LittleEndian.Uint32(packet[:4]) == 1 {
					stats.StandardInit++
				}
				destination := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 51820}
				if server {
					if target == nil {
						stats.ForwardErrors++
						continue
					}
					destination = target
				} else {
					target = from
				}
				if _, err := relay.WriteToUDP(packet, destination); err != nil {
					stats.ForwardErrors++
				}
			}
		}()
		encoded, _ := json.Marshal(map[string]any{"PrivateKey": hex.EncodeToString(privBytes), "ServerPublicKey": hex.EncodeToString(serverBytes), "Endpoint": "127.0.0.1:51821", "Marker": hex.EncodeToString(marker), "AWG": true})
		childCtx, childCancel := context.WithTimeout(ctx, 9*time.Second)
		child := exec.CommandContext(childCtx, "/artifacts/manager-client-probe", "client")
		child.Stdin = bytes.NewReader(encoded)
		output, childErr := child.Output()
		childCancel()
		relay.Close()
		wire := <-done
		if childErr != nil {
			t.Fatal("ephemeral client execution failed (stderr withheld)")
		}
		var observed clientProof
		if json.Unmarshal(output, &observed) != nil {
			t.Fatal("client proof invalid")
		}
		if observed.MarkerSHA != hex.EncodeToString(sha256Sum(marker)) {
			t.Fatal("fresh payload proof mismatch")
		}
		if wire.PlaintextMarker || wire.ForwardErrors != 0 || wire.StandardInit != 0 || wire.AWGInit < 1 {
			t.Fatal("encrypted wire proof invalid")
		}
		after, statusErr := m.ReadPeerStatus(ctx, client.PublicKey)
		if want {
			if !observed.Delivered || observed.Handshake <= 0 || observed.Rx <= 0 || observed.Tx <= 0 || statusErr != nil || !after.Handshaken || after.RXBytes <= 0 || after.TXBytes <= 0 || wire.AWGResponse < 1 || wire.ClientTransport < 1 || wire.ServerTransport < 1 {
				t.Fatal("actual AWG positive lacks handshake/packet/status proof")
			}
		} else if observed.Delivered || observed.Handshake != 0 || observed.Rx != 0 || wire.AWGResponse != 0 || wire.ServerTransport != 0 || !errors.Is(statusErr, ErrPeerNotFound) {
			t.Fatal("disabled/revoked device still passes packets")
		}
		phases = append(phases, map[string]any{"phase": label, "client": observed, "wire": wire, "server_handshaken": after.Handshaken, "server_rx": after.RXBytes, "server_tx": after.TXBytes})
	}
	runPhase("active", true)
	if err := registry.Disable(client.ID); err != nil {
		t.Fatal(err)
	}
	runPhase("disabled", false)
	if err := registry.Enable(client.ID); err != nil {
		t.Fatal(err)
	}
	runPhase("enabled", true)
	if err := registry.Revoke(client.ID); err != nil {
		t.Fatal(err)
	}
	runPhase("revoked", false)
	if _, err := registry.RenderClientConfig(client.ID, serverPublic, "127.0.0.1:51821"); err == nil {
		t.Fatal("revoked profile exported")
	}
	if err := m.Close(context.Background()); err != nil {
		t.Fatal("packet owner join failed")
	}
	if _, err := os.Lstat(m.socketPath); !os.IsNotExist(err) {
		t.Fatal("owned socket retained")
	}
	if _, err := net.InterfaceByName(name); err == nil {
		t.Fatal("owned TUN retained")
	}
	proof := map[string]any{"status": "PASS_MANAGER_OWNER_REGISTRY_AWG_PACKETS", "phases": phases, "ip_commands_injected": 0, "actual_address": true, "child_joined": true, "socket_removed": true, "tun_removed": true, "full_app_http_policy": "NOT_TESTED"}
	encoded, _ := json.Marshal(proof)
	t.Log("M2_MANAGER_PACKET_JSON", string(encoded))
}

func sha256Sum(b []byte) []byte { s := sha256.Sum256(b); return s[:] }
