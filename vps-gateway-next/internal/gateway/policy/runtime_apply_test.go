package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

func TestRuntimeApplyUsesAWG3AsVPNAndWritesGatewaySlot(t *testing.T) {
	saver := new(recordingSlotSaver)
	runtime := Runtime{
		Catalog: fakeAWG3Catalog{tags: []awg3endpoint.TagInfo{{Tag: "ams", Kind: "awg3"}}},
		Saver:   saver,
	}
	profile := Profile{ID: "p", Name: "VPN", DefaultAction: ActionVPN}

	if err := runtime.Apply(profile, "ams"); err != nil {
		t.Fatal(err)
	}
	if saver.slot != orchestrator.SlotGatewayPolicy {
		t.Fatalf("slot = %q, want %q", saver.slot, orchestrator.SlotGatewayPolicy)
	}
	if len(saver.data) == 0 || !containsBytes(saver.data, []byte(`"outbound": "ams"`)) {
		t.Fatalf("gateway slot does not select AWG3 VPN: %s", saver.data)
	}
}

func TestRuntimeApplyActivatesFreshGatewayPolicySlot(t *testing.T) {
	dir := t.TempDir()
	orch := orchestrator.NewWithAppliedPath(dir, nil, filepath.Join(t.TempDir(), "applied.json"))
	t.Cleanup(orch.Close)
	for _, meta := range orchestrator.KnownSlots() {
		if err := orch.Register(meta); err != nil {
			t.Fatalf("register %s: %v", meta.Slot, err)
		}
	}
	if err := orch.Bootstrap(); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if err := orch.Save(orchestrator.SlotAwg3, []byte(`{"outbounds":[{"type":"direct","tag":"ams"}]}`)); err != nil {
		t.Fatalf("seed AWG3 outbound: %v", err)
	}

	runtime := Runtime{
		Catalog: fakeAWG3Catalog{tags: []awg3endpoint.TagInfo{{Tag: "ams", Kind: "awg3"}}},
		Saver:   orch,
	}
	profile := Profile{ID: "p", Name: "VPN", DefaultAction: ActionVPN}
	if err := runtime.Apply(profile, "ams"); err != nil {
		t.Fatalf("apply: %v", err)
	}

	data, enabled, err := orch.SnapshotSlot(orchestrator.SlotGatewayPolicy)
	if err != nil {
		t.Fatalf("snapshot gateway policy: %v", err)
	}
	if !enabled {
		t.Fatal("freshly applied gateway policy slot is disabled")
	}
	activePath := filepath.Join(dir, "18-z-gateway-policy.json")
	active, err := os.ReadFile(activePath)
	if err != nil {
		t.Fatalf("read active gateway policy file: %v", err)
	}
	if len(data) == 0 || string(active) != string(data) || !containsBytes(data, []byte(`"type": "tun"`)) {
		t.Fatalf("active gateway policy does not contain the TUN inbound: %s", data)
	}
}

func TestRuntimeApplyFailsWithoutAWG3ForVPNProfile(t *testing.T) {
	runtime := Runtime{
		Catalog: fakeAWG3Catalog{},
		Saver:   new(recordingSlotSaver),
	}
	profile := Profile{ID: "p", Name: "VPN", DefaultAction: ActionVPN}
	if err := runtime.Apply(profile, ""); err == nil {
		t.Fatal("expected VPN apply to fail without AWG3 endpoint")
	}
}

func containsBytes(haystack, needle []byte) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
