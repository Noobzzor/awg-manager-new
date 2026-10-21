package main

import (
	"bytes"
	"encoding/base64"
	gatewaywarp "github.com/hoaxisr/awg-manager/internal/gateway/warp"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayWARPStartupRestoresSavedOrEmptyProvider(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty", true: "configured"}[configured], func(t *testing.T) {
			dir := t.TempDir()
			store := gatewaywarp.NewStore(dir)
			if configured {
				p := gatewaywarp.Profile{Endpoint: "162.159.193.10:2408", IPv4: "172.16.0.2", MTU: 1280, PrivateKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)), PublicKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)), Reserved: []int{0, 127, 255}}
				if err := store.Save(p); err != nil {
					t.Fatal(err)
				}
			}
			orch := orchestrator.NewWithAppliedPath(filepath.Join(dir, "config"), nil, filepath.Join(dir, "applied.json"))
			t.Cleanup(orch.Close)
			for _, m := range orchestrator.KnownSlots() {
				if err := orch.Register(m); err != nil {
					t.Fatal(err)
				}
			}
			if err := orch.Bootstrap(); err != nil {
				t.Fatal(err)
			}
			a := &app{dataDir: dir, sbOrch: orch}
			service, err := a.setupGatewayWARP()
			if err != nil || service == nil {
				t.Fatalf("startup: %v", err)
			}
			cfg, err := service.WARPConfig()
			if err != nil || (cfg != nil) != configured {
				t.Fatal("wrong startup projection")
			}
			data, enabled, err := orch.SnapshotSlot(orchestrator.SlotWARP)
			if err != nil || !enabled || len(data) == 0 {
				t.Fatal("startup slot missing")
			}
			if configured && !bytes.Contains(data, []byte(`"wireguard"`)) {
				t.Fatal("startup endpoint missing")
			}
			info, err := os.Stat(filepath.Join(dir, "config/16-z-warp.json"))
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("runtime config not private")
			}
		})
	}
}
func TestGatewayWARPStartupRejectsBrokenProfile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gateway-warp.json"), []byte(`{`), 0600); err != nil {
		t.Fatal(err)
	}
	orch := orchestrator.NewWithAppliedPath(filepath.Join(dir, "config"), nil, filepath.Join(dir, "applied.json"))
	t.Cleanup(orch.Close)
	a := &app{dataDir: dir, sbOrch: orch}
	if service, err := a.setupGatewayWARP(); err == nil || service != nil {
		t.Fatal("broken WARP profile exposed")
	}
}
