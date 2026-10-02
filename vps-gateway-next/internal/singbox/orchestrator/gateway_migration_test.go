package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/configmerge"
)

const oldGatewayFilename = "19-gateway-policy.json"
const newGatewayFilename = "18-z-gateway-policy.json"

func gatewayMigrationOrch(t *testing.T) (*Orchestrator, string) {
	t.Helper()
	o, dir := newTestOrch(t)
	for _, meta := range KnownSlots() {
		if meta.Slot == SlotGatewayPolicy || meta.Slot == SlotBase {
			if err := o.Register(meta); err != nil {
				t.Fatal(err)
			}
		}
	}
	// These tests exercise persistence/merge, not process reloads.
	o.HoldReloads()
	return o, dir
}

func migrationWrite(t *testing.T, dir, location, name, data string) {
	t.Helper()
	path := filepath.Join(dir, location, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

type migrationMergeValidator struct {
	merged string
}

func (v *migrationMergeValidator) Validate(_ context.Context, dir string) error {
	var err error
	v.merged, err = configmerge.MergeDir(dir)
	return err
}

func TestGatewayMigration_ApplyMatchesRuntimeMerge(t *testing.T) {
	o, dir := gatewayMigrationOrch(t)
	migrationWrite(t, dir, "", "00-base.json", `{"outbounds":[{"type":"direct","tag":"direct"}]}`)
	legacy := `{"inbounds":[{"type":"mixed","tag":"gateway"}],"route":{"final":"direct"}}`
	migrationWrite(t, dir, "", oldGatewayFilename, legacy)
	if err := o.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	got, enabled, err := o.SnapshotSlot(SlotGatewayPolicy)
	if err != nil || !enabled || string(got) != legacy {
		t.Fatalf("legacy active policy not adopted: enabled=%v data=%s err=%v", enabled, got, err)
	}
	validator := &migrationMergeValidator{}
	o.SetValidator(validator)
	replacement := `{"inbounds":[{"type":"mixed","tag":"gateway"}],"route":{"rules":[{"inbound":["gateway"],"outbound":"direct"}]}}`
	if err := o.SaveDraft(SlotGatewayPolicy, []byte(replacement)); err != nil {
		t.Fatal(err)
	}
	res, err := o.ApplyDraft(SlotGatewayPolicy)
	if err != nil || !res.Ok() {
		t.Fatalf("apply: result=%+v err=%v", res, err)
	}
	merged, err := configmerge.MergeDir(dir)
	if err != nil {
		t.Fatalf("runtime merge still sees legacy inbound: %v", err)
	}
	if merged != validator.merged {
		t.Fatalf("validator/runtime mismatch:\nvalidator=%s\nruntime=%s", validator.merged, merged)
	}
	var cfg struct {
		Inbounds []json.RawMessage          `json:"inbounds"`
		Route    map[string]json.RawMessage `json:"route"`
	}
	if err := json.Unmarshal([]byte(merged), &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Inbounds) != 1 || cfg.Route["final"] != nil {
		t.Fatalf("stale policy survived replacement: %s", merged)
	}
	if _, err := os.Stat(filepath.Join(dir, oldGatewayFilename)); !os.IsNotExist(err) {
		t.Fatalf("legacy active path remains: %v", err)
	}
}
