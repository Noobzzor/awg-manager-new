//go:build linux

package ingress

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Real AWG/tool/Registry control. ONLY IP address/up operations are injected:
// this proves peer management, NOT addressing, routing, packets or full app.
func TestM2ActualAWGRegistryControl(t *testing.T) {
	if os.Getenv("AWGM_REAL_REGISTRY_CONTROL") != "1" {
		t.Skip("explicit owned TUN fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := "m2ctl0"
	params := testedAWGParameters()
	dir := t.TempDir()
	tool := "/artifacts/awg"
	m := NewUserspaceAWGInterface(UserspaceAWGConfig{RuntimeBinary: "/input/amneziawg-go", ToolBinary: tool, IPBinary: "/bin/true", InterfaceName: name, ListenPort: 51820, Address: "10.66.0.1/24", KeyDirectory: filepath.Join(dir, "keys"), Parameters: params})
	t.Cleanup(func() {
		if err := m.Close(context.Background()); err != nil {
			t.Error("owned actual AWG cleanup incomplete")
		}
	})
	if err := m.Preflight(ctx); err != nil {
		t.Fatal(err)
	}
	privateKey, publicKey, err := GenerateKeyPairWithBinary(ctx, tool)
	if err != nil {
		t.Fatal("actual server key generation failed")
	}
	if err := m.SetPrivateKey(privateKey); err != nil {
		t.Fatal("actual identity installation refused")
	}
	actualRun := m.run
	ipCalls := 0
	m.run = func(ctx context.Context, binary string, args ...string) (string, error) {
		if binary == "/bin/true" {
			address := []string{"address", "add", "10.66.0.1/24", "dev", name}
			up := []string{"link", "set", "up", "dev", name}
			if !reflect.DeepEqual(args, address) && !reflect.DeepEqual(args, up) {
				t.Fatal("unexpected injected IP command")
			}
			ipCalls++
			return "", nil
		}
		if binary != tool {
			t.Fatal("unexpected non-AWG executable")
		}
		return actualRun(ctx, binary, args...)
	}
	if err := m.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	if ipCalls != 2 {
		t.Fatal("control-only IP injection contract changed")
	}
	keyFiles, err := os.ReadDir(m.config.KeyDirectory)
	if err != nil || len(keyFiles) != 0 {
		t.Fatal("temporary key file retained")
	}
	observed, err := actualRun(ctx, tool, "show", name, "public-key")
	if err != nil || strings.TrimSpace(observed) != publicKey {
		t.Fatal("actual server identity readback mismatch")
	}
	allocator, err := NewAllocator(netip.MustParsePrefix("10.66.0.0/24"))
	if err != nil {
		t.Fatal(err)
	}
	if err := allocator.ReserveAddress(netip.MustParseAddr("10.66.0.1")); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry(allocator, func(string) (string, string, error) { return GenerateKeyPairWithBinary(ctx, tool) })
	if err := registry.UseAWGParameters(params); err != nil {
		t.Fatal(err)
	}
	client, err := registry.Create("control-client", "Owned control fixture")
	if err != nil {
		t.Fatal("actual client creation failed")
	}
	profile, err := registry.RenderClientConfig(client.ID, publicKey, "127.0.0.1:51820")
	if err != nil {
		t.Fatal("actual client export failed")
	}
	for _, field := range params.fields() {
		if !strings.Contains(profile, fmt.Sprintf("%s = %d\n", field.name, field.value)) {
			t.Fatal("export/runtime profile mismatch")
		}
	}
	presence := []bool{}
	verify := func(want bool) {
		if err := registry.SyncClient(ctx, client.ID, "", m); err != nil {
			t.Fatal(err)
		}
		status, err := m.ReadPeerStatus(ctx, client.PublicKey)
		if want {
			if err != nil || status.PublicKey != client.PublicKey || status.Handshaken || status.RXBytes != 0 || status.TXBytes != 0 {
				t.Fatal("real new peer status readback mismatch")
			}
		} else if !errors.Is(err, ErrPeerNotFound) {
			t.Fatal("real removed peer still present or unreadable")
		}
		presence = append(presence, want)
	}
	verify(true)
	if err := registry.Disable(client.ID); err != nil {
		t.Fatal(err)
	}
	verify(false)
	if err := registry.Enable(client.ID); err != nil {
		t.Fatal(err)
	}
	verify(true)
	if err := registry.Revoke(client.ID); err != nil {
		t.Fatal(err)
	}
	verify(false)
	if _, err := registry.RenderClientConfig(client.ID, publicKey, "127.0.0.1:51820"); err == nil {
		t.Fatal("revoked client exported")
	}
	if err := m.Close(context.Background()); err != nil {
		t.Fatal("actual owned child join failed")
	}
	if _, err := os.Lstat(m.socketPath); !os.IsNotExist(err) {
		t.Fatal("actual owned socket remains")
	}
	if _, err := os.Stat(filepath.Join("/sys/class/net", name)); !os.IsNotExist(err) {
		t.Fatal("actual owned TUN remains")
	}
	toolBytes, err := os.ReadFile(tool)
	if err != nil {
		t.Fatal(err)
	}
	toolHash := sha256.Sum256(toolBytes)
	proof := map[string]any{"status": "PASS_ACTUAL_AWG_REGISTRY_CONTROL_ONLY", "presence": presence, "awg_parameters_verified": 11, "temporary_key_removed": true, "child_joined": true, "socket_removed": true, "tun_removed": true, "injected_ip_commands": ipCalls, "addressing_routing_packets": "NOT_TESTED", "tool_sha256": hex.EncodeToString(toolHash[:])}
	encoded, _ := json.Marshal(proof)
	t.Log("M2_CONTROL_JSON", string(encoded))
}
