//go:build linux

package ingress

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// This gated fixture uses real kernel IP operations; it never substitutes m.run.
// It is not a full application, forwarding, policy, or client-device acceptance.
func TestM2ActualAWGAddressing(t *testing.T) {
	if os.Getenv("AWGM_REAL_ADDRESSING") != "1" {
		t.Skip("explicit owned network-none TUN fixture required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := "m2addr0"
	m := NewUserspaceAWGInterface(UserspaceAWGConfig{RuntimeBinary: "/input/amneziawg-go", ToolBinary: "/artifacts/awg", IPBinary: "/input/ip", InterfaceName: name, ListenPort: 51820, Address: "10.77.0.1/30", KeyDirectory: filepath.Join(t.TempDir(), "keys"), Parameters: testedAWGParameters()})
	t.Cleanup(func() {
		if err := m.Close(context.Background()); err != nil {
			t.Error("actual addressing cleanup incomplete")
		}
	})
	if err := m.Preflight(ctx); err != nil {
		t.Fatal(err)
	}
	privateKey, _, err := GenerateKeyPairWithBinary(ctx, "/artifacts/awg")
	if err != nil {
		t.Fatal("actual key generation failed")
	}
	if err := m.SetPrivateKey(privateKey); err != nil {
		t.Fatal("actual key installation failed")
	}
	if err := m.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	iface, err := net.InterfaceByName(name)
	if err != nil || iface.Flags&net.FlagUp == 0 {
		t.Fatal("actual TUN not up")
	}
	addrs, err := iface.Addrs()
	if err != nil {
		t.Fatal("actual address readback failed")
	}
	found := false
	for _, addr := range addrs {
		if addr.String() == "10.77.0.1/30" {
			found = true
		}
	}
	if !found {
		t.Fatal("manager did not assign expected actual prefix")
	}
	route, err := exec.CommandContext(ctx, "/input/ip", "route", "get", "10.77.0.2").Output()
	if err != nil || !strings.Contains(string(route), "dev "+name) || !strings.Contains(string(route), "src 10.77.0.1") {
		t.Fatal("actual connected return route missing")
	}
	keyFiles, err := os.ReadDir(m.config.KeyDirectory)
	if err != nil || len(keyFiles) != 0 {
		t.Fatal("temporary identity retained")
	}
	if err := m.Close(context.Background()); err != nil {
		t.Fatal("actual owned child join failed")
	}
	if _, err := os.Lstat(m.socketPath); !os.IsNotExist(err) {
		t.Fatal("actual socket remains")
	}
	if _, err := net.InterfaceByName(name); err == nil {
		t.Fatal("actual TUN remains")
	}
	proof := map[string]any{"status": "PASS_MANAGER_OWNED_AWG_REAL_ADDRESSING", "ip_commands_injected": 0, "actual_address": "10.77.0.1/30", "up": true, "connected_return_route": true, "child_joined": true, "socket_removed": true, "tun_removed": true, "packets": "NOT_TESTED"}
	encoded, _ := json.Marshal(proof)
	t.Log("M2_ADDRESS_JSON", string(encoded))
}
