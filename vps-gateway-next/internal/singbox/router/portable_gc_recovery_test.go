package router

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

func TestPortableArtifactGCRetainsRecoveryVersions(t *testing.T) {
	svc, _, orch := portableEditorFixture(t)
	if err := orch.Register(orchestrator.SlotMeta{Slot: orchestrator.SlotFakeIP, Filename: "21-fakeip.json"}); err != nil {
		t.Fatal(err)
	}
	impl := svc.(*ServiceImpl)
	dir := filepath.Join(impl.ruleSetMaterializer().configDir, "rule-sets", "inline")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(dir, "sha256-"+strings.Repeat("a", 64)+".srs")
	rollback := filepath.Join(dir, "sha256-"+strings.Repeat("b", 64)+".srs")
	for _, p := range []string{keep, rollback} {
		if err := os.WriteFile(p, []byte("owned GC fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := NewEmptyConfig()
	cfg.Route.RuleSet = []RuleSet{managedLocalRuleSet("keep-srs", keep)}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := impl.portableDocuments.(*FileEditorDocuments).CommitApplied(data); err != nil {
		t.Fatal(err)
	}
	orch.SetRuntimeRecoveryRequired(true)
	impl.GCRuleSetArtifacts()
	if _, err := os.Stat(rollback); err != nil {
		t.Fatal("GC removed a possible recovery version while recovery was required", err)
	}
	orch.SetRuntimeRecoveryRequired(false)
	impl.GCRuleSetArtifacts()
	if _, err := os.Stat(rollback); !os.IsNotExist(err) {
		t.Fatal("GC did not resume after recovery was cleared")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("GC removed the applied immutable artifact")
	}
}
