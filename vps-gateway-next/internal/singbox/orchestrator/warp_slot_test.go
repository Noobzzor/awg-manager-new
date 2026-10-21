package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWARPInfrastructureSlotRegisteredBeforeGateway(t *testing.T) {
	var warpName, gatewayName string
	for _, m := range KnownSlots() {
		if m.Slot == SlotWARP {
			warpName = m.Filename
			if !m.AlwaysOn || m.HasContent != nil {
				t.Fatal("WARP infrastructure activates engine")
			}
		}
		if m.Slot == SlotGatewayPolicy {
			gatewayName = m.Filename
		}
	}
	if warpName != "16-z-warp.json" || warpName >= gatewayName {
		t.Fatal("WARP slot missing or wrong order")
	}
}
func TestSecretConfigurationWritesArePrivate(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"16-z-warp.json", "pending-warp.json"} {
		path := filepath.Join(dir, name)
		if err := writeAtomic(path, []byte(`{"private_key":"SYNTHETIC"}`)); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("secret config readable by other users")
		}
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
		if err := writeAtomic(path, []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
		info, _ = os.Stat(path)
		if info.Mode().Perm() != 0600 {
			t.Fatal("replacement retained insecure mode")
		}
	}
}
