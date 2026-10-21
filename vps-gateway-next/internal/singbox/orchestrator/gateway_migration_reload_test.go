package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/configmerge"
)

// This is a deterministic reload/skip regression, not manager OS-process or
// packet evidence. The separately owned real-engine fixture covers those.
func TestGatewayMigrationDisabledCurrentMustApplyToSurvivingProcess(t *testing.T) {
	dir := t.TempDir()
	applied := filepath.Join(t.TempDir(), "applied.json")
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("20-router.json", `{"route":{"rules":[]}}`)
	write("disabled/"+gatewayPolicyFilename, `{"route":{"rules":[]}}`)
	write(legacyGatewayPolicyFilename, `{"route":{"rules":[{"port":18082,"action":"reject"}]}}`)

	fp := &fakeProc{running: true}
	// Emulate the pre-migration hash contract. A disabled Gateway contributes
	// nothing to that hash, just as an unregistered Gateway contributes nothing.
	// No manual breadcrumb/hash injection: baseline Reload persists it.
	before := NewWithAppliedPath(dir, fp, applied)
	t.Cleanup(before.Close)
	if err := before.Register(SlotMeta{Slot: SlotRouter, Filename: "20-router.json"}); err != nil {
		t.Fatal(err)
	}
	if err := before.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	merged, err := configmerge.MergeDir(dir)
	if err != nil || !strings.Contains(merged, "18082") {
		t.Fatalf("baseline actual merge must include legacy BLOCK: err=%v", err)
	}
	if err := before.Reload(); err != nil {
		t.Fatal(err)
	}
	if fp.reloadsN() != 1 {
		t.Fatalf("baseline must actually apply once, got %d", fp.reloadsN())
	}
	before.Close()

	after := NewWithAppliedPath(dir, fp, applied)
	t.Cleanup(after.Close)
	for _, meta := range []SlotMeta{
		{Slot: SlotRouter, Filename: "20-router.json"},
		{Slot: SlotGatewayPolicy, Filename: gatewayPolicyFilename},
	} {
		if err := after.Register(meta); err != nil {
			t.Fatal(err)
		}
	}
	if err := after.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	merged, err = configmerge.MergeDir(dir)
	if err != nil || strings.Contains(merged, "18082") {
		t.Fatalf("desired actual merge must exclude archived legacy BLOCK: err=%v", err)
	}
	if err := after.Reload(); err != nil {
		t.Fatal(err)
	}
	if fp.reloadsN() != 2 {
		t.Fatalf("migration removed runtime-visible legacy BLOCK, but unchanged-slot hash skipped apply: reloads=%d, want 2", fp.reloadsN())
	}
	if fp.startsN() != 0 || fp.stopsN() != 0 {
		t.Fatalf("no-TUN convergence must not require stop/start: calls=%v", fp.calls())
	}
	if err := after.Reload(); err != nil {
		t.Fatal(err)
	}
	if fp.reloadsN() != 2 {
		t.Fatalf("unchanged post-migration reload must still be skipped, got %d", fp.reloadsN())
	}
}
