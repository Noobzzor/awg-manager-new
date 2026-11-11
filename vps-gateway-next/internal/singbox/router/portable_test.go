package router

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/presets"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

var errPortableUnavailable = errors.New("gateway runtime unavailable fixture")

type portableRuntimeFixture struct {
	calls       []string
	statusGuard sync.Locker
}

func (p *portableRuntimeFixture) StagingStatus(context.Context) StagingStatus {
	p.calls = append(p.calls, "staging")
	return StagingStatus{HasDraft: true, Validation: &orchestrator.ValidationResult{Errors: []orchestrator.ValidationError{{Kind: "gateway-projection-fixture", Message: "projected candidate is invalid"}}}}
}

func (p *portableRuntimeFixture) called(name string) error {
	p.calls = append(p.calls, name)
	return errPortableUnavailable
}
func (p *portableRuntimeFixture) GetStatus(context.Context) (Status, error) {
	if p.statusGuard != nil {
		p.statusGuard.Lock()
		defer p.statusGuard.Unlock()
	}
	return Status{}, p.called("status")
}
func (p *portableRuntimeFixture) GetSettings(context.Context) (storage.SingboxRouterSettings, error) {
	return storage.SingboxRouterSettings{}, p.called("settings")
}
func (p *portableRuntimeFixture) UpdateSettings(context.Context, storage.SingboxRouterSettings) error {
	return p.called("update")
}
func (p *portableRuntimeFixture) Reconcile(context.Context) error { return p.called("reconcile") }
func (p *portableRuntimeFixture) SwitchRoutingMode(context.Context, string) error {
	return p.called("mode")
}
func (p *portableRuntimeFixture) ApplyStaging(context.Context) (orchestrator.ValidationResult, error) {
	return orchestrator.ValidationResult{}, p.called("apply")
}
func (p *portableRuntimeFixture) RenameExternalOutboundTag(context.Context, string, string) error {
	return p.called("rename")
}

func portableEditorFixture(t *testing.T) (Service, *portableRuntimeFixture, *orchestrator.Orchestrator) {
	return portableEditorWithGuard(t, &sync.Mutex{})
}

func portableEditorWithGuard(t *testing.T, guard sync.Locker) (Service, *portableRuntimeFixture, *orchestrator.Orchestrator) {
	t.Helper()
	sb := newTestSingbox(t)
	settings := newTestSettingsStore(t, storage.SingboxRouterSettings{})
	orch := orchestrator.NewWithAppliedPath(sb.dir, nil, filepath.Join(t.TempDir(), "applied.json"))
	if err := orch.Register(orchestrator.SlotMeta{Slot: orchestrator.SlotRouter, Filename: "20-router.json"}); err != nil {
		t.Fatal(err)
	}
	if err := orch.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(orch.Close)
	runtime := &portableRuntimeFixture{statusGuard: guard}
	documents, err := OpenEditorDocuments(filepath.Join(t.TempDir(), "canonical-editor"))
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewPortableService(PortableDeps{Settings: settings, Catalog: presets.NewCatalog(presets.NewStore(t.TempDir())), Singbox: sb, Orch: orch, Runtime: runtime, Documents: documents, Coordinator: guard, Events: events.NewBus()})
	if err != nil {
		t.Fatal(err)
	}
	return svc, runtime, orch
}

func TestPortableEditorNeverReadsOrWritesDerivedRuntimeSlot(t *testing.T) {
	svc, _, orch := portableEditorFixture(t)
	impl := svc.(*ServiceImpl)
	documents := impl.portableDocuments.(*FileEditorDocuments)
	canonical := []byte(`{"route":{"final":"direct","rules":[{"domain_suffix":["authored.example"],"outbound":"direct"}]}}`)
	if err := documents.CommitApplied(canonical); err != nil {
		t.Fatal(err)
	}
	runtime := []byte(`{"route":{"rules":[{"domain_suffix":["runtime-only.example"],"action":"reject"}]}}`)
	if err := orch.RestoreSlot(orchestrator.SlotRouter, runtime, false); err != nil {
		t.Fatal(err)
	}
	rules, err := svc.ListRules(context.Background())
	if err != nil || len(rules) != 1 || rules[0].DomainSuffix[0] != "authored.example" {
		t.Fatal("editor loaded derived runtime instead of authored document", err)
	}
	if err := svc.AddRule(context.Background(), Rule{DomainSuffix: []string{"new.example"}, Outbound: "direct"}); err != nil {
		t.Fatal(err)
	}
	rules, err = svc.ListRules(context.Background())
	if err != nil || len(rules) != 2 {
		t.Fatal("authored edit did not remain in canonical draft", err)
	}
	if orch.DraftInfo(orchestrator.SlotRouter).HasDraft {
		t.Fatal("canonical edit staged an engine runtime slot")
	}
	if err := svc.DiscardStaging(context.Background()); err != nil {
		t.Fatal(err)
	}
	rules, err = svc.ListRules(context.Background())
	if err != nil || len(rules) != 1 || rules[0].DomainSuffix[0] != "authored.example" {
		t.Fatal("discard did not restore authored applied document", err)
	}
	got, _, err := orch.SnapshotSlot(orchestrator.SlotRouter)
	if err != nil || string(got) != string(runtime) {
		t.Fatal("edit or discard changed derived runtime bytes", err)
	}
}

type portableGuardFixture struct {
	mu      sync.Mutex
	attempt chan struct{}
}

func (g *portableGuardFixture) Lock()   { g.attempt <- struct{}{}; g.mu.Lock() }
func (g *portableGuardFixture) Unlock() { g.mu.Unlock() }

func TestPortableEditsWaitForApplyCoordinator(t *testing.T) {
	g := &portableGuardFixture{attempt: make(chan struct{}, 4)}
	svc, _, orch := portableEditorWithGuard(t, g)
	g.mu.Lock()
	done := make(chan error, 1)
	go func() {
		done <- svc.AddRule(context.Background(), Rule{DomainSuffix: []string{"fixture.example"}, Outbound: "direct"})
	}()
	select {
	case <-g.attempt:
	case <-time.After(time.Second):
		g.mu.Unlock()
		t.Fatal("original edit bypassed apply coordinator")
	}
	if raw, err := svc.(*ServiceImpl).portableDocuments.LoadEffective(); err != nil || raw != nil {
		g.mu.Unlock()
		t.Fatal("edit wrote during application transaction")
	}
	g.mu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("edit/status callback deadlocked on coordinator")
	}
	if raw, err := svc.(*ServiceImpl).portableDocuments.LoadEffective(); err != nil || raw == nil {
		t.Fatal("edit did not resume after application transaction")
	}
	if orch.DraftInfo(orchestrator.SlotRouter).HasDraft {
		t.Fatal("editor staged a derived runtime slot")
	}
}

func TestPortableInlineDraftCannotOverwriteAppliedArtifacts(t *testing.T) {
	svc, _, orch := portableEditorFixture(t)
	// Production registers both routing slots. GC intentionally aborts if a
	// sibling slot cannot be read, so retain that completeness in the fixture.
	if err := orch.Register(orchestrator.SlotMeta{Slot: orchestrator.SlotFakeIP, Filename: "21-fakeip.json"}); err != nil {
		t.Fatal(err)
	}
	impl := svc.(*ServiceImpl)
	documents := impl.portableDocuments.(*FileEditorDocuments)
	impl.deps.Singbox.(*fakeSingbox).binary = "fixture-compiler"
	withFakeRuleSetCompiler(t, func(_ string, args []string) (string, string, error) {
		source, err := os.ReadFile(args[len(args)-1])
		if err != nil {
			return "", "", err
		}
		writeCompiledOutput(t, args, string(source))
		return "", "", nil
	})
	ctx := context.Background()
	if err := svc.AddRuleSet(ctx, RuleSet{Tag: "fixture-inline", Type: "inline", Rules: []map[string]any{{"domain_suffix": []string{"before.example"}}}}); err != nil {
		t.Fatal(err)
	}
	applied, err := documents.LoadEffective()
	if err != nil {
		t.Fatal(err)
	}
	if err := documents.CommitApplied(applied); err != nil {
		t.Fatal(err)
	}
	if err := documents.DiscardDraft(); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseRouterConfigBytes(applied)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := cfg.Route.RuleSet[0].Path
	oldBinary, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatal(err)
	}
	oldSourcePath := oldPath[:len(oldPath)-len(".srs")] + ".json"
	oldSource, err := os.ReadFile(oldSourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.UpdateRuleSet(ctx, "fixture-inline", RuleSet{Tag: "fixture-inline", Type: "inline", Rules: []map[string]any{{"domain_suffix": []string{"after.example"}}}}); err != nil {
		t.Fatal(err)
	}
	stillBinary, err := os.ReadFile(oldPath)
	if err != nil || string(stillBinary) != string(oldBinary) {
		t.Fatal("editing a draft changed the applied SRS before Apply")
	}
	stillSource, err := os.ReadFile(oldSourcePath)
	if err != nil || string(stillSource) != string(oldSource) {
		t.Fatal("editing a draft changed applied rule-set source")
	}
	pending, err := documents.LoadEffective()
	if err != nil {
		t.Fatal(err)
	}
	changed, err := parseRouterConfigBytes(pending)
	if err != nil {
		t.Fatal(err)
	}
	newPath := changed.Route.RuleSet[0].Path
	if newPath == oldPath {
		t.Fatal("draft and applied rule-set share a mutable artifact")
	}
	if err := svc.DiscardStaging(ctx); err != nil {
		t.Fatal(err)
	}
	impl.GCRuleSetArtifacts()
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatal("discard/GC removed applied version", err)
	}
	if _, err := os.Stat(newPath); !os.IsNotExist(err) {
		t.Fatal("discard/GC retained unreferenced draft version", err)
	}
}

func TestPortableRuntimeSettingsBoundary(t *testing.T) {
	svc, runtime, _ := portableEditorFixture(t)
	ctx := context.Background()
	if _, err := svc.GetSettings(ctx); !errors.Is(err, errPortableUnavailable) {
		t.Errorf("settings must come from explicit gateway runtime; got %v", err)
	}
	if err := svc.UpdateSettings(ctx, storage.SingboxRouterSettings{RoutingMode: "invalid-fixture"}); !errors.Is(err, errPortableUnavailable) {
		t.Errorf("settings must not enter legacy normalization/side effects; got %v", err)
	}
	if len(runtime.calls) != 2 {
		t.Errorf("runtime calls=%v, want settings/update", runtime.calls)
	}
}

func TestPortableStagingUsesGatewayProjectionValidation(t *testing.T) {
	svc, runtime, _ := portableEditorFixture(t)
	got := svc.StagingStatus(context.Background())
	if !got.HasDraft || got.Validation == nil || len(got.Validation.Errors) != 1 || got.Validation.Errors[0].Kind != "gateway-projection-fixture" {
		t.Fatal("portable staging must validate the projected gateway configuration, not the legacy router slot")
	}
	if len(runtime.calls) != 1 || runtime.calls[0] != "staging" {
		t.Fatal("projected staging runtime was bypassed")
	}
}

func TestPortableRuntimeLifecycleCannotFallBackToRouter(t *testing.T) {
	svc, _, _ := portableEditorFixture(t)
	ctx := context.Background()
	if _, err := svc.GetStatus(ctx); !errors.Is(err, errPortableUnavailable) {
		t.Fatal(err)
	}
	if err := svc.Reconcile(ctx); !errors.Is(err, errPortableUnavailable) {
		t.Fatal(err)
	}
	if err := svc.SwitchRoutingMode(ctx, "tproxy"); !errors.Is(err, errPortableUnavailable) {
		t.Fatal(err)
	}
	if _, err := svc.ApplyStaging(ctx); !errors.Is(err, errPortableUnavailable) {
		t.Fatal(err)
	}
	impl := svc.(*ServiceImpl)
	if impl.deps.IPTables != nil || impl.deps.WANIPCollector != nil || impl.deps.OpkgTun != nil || impl.deps.Policies != nil {
		t.Fatal("portable editor constructed router platform dependencies")
	}
}

func TestPortableOriginalRuleOrderStaysDrafted(t *testing.T) {
	svc, runtime, orch := portableEditorFixture(t)
	ctx := context.Background()
	for _, domain := range []string{"one.example", "two.example"} {
		if err := svc.AddRule(ctx, Rule{DomainSuffix: []string{domain}, Outbound: "direct"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.MoveRule(ctx, 0, 1); err != nil {
		t.Fatal(err)
	}
	rules, err := svc.ListRules(ctx)
	if err != nil || len(rules) != 2 || rules[0].DomainSuffix[0] != "two.example" {
		t.Fatalf("original order CRUD failed: %v %v", rules, err)
	}
	if raw, err := svc.(*ServiceImpl).portableDocuments.LoadEffective(); err != nil || raw == nil {
		t.Fatal("original editor did not stage rules")
	}
	if orch.DraftInfo(orchestrator.SlotRouter).HasDraft {
		t.Fatal("editor staged engine-owned runtime")
	}
	if applied, err := orch.LoadApplied(orchestrator.SlotRouter); err != nil || len(applied) != 0 {
		t.Fatal("editing must not activate a router slot", err)
	}
	for _, call := range runtime.calls {
		if call != "status" {
			t.Fatalf("editing activated platform operation %s", call)
		}
	}
	if len(runtime.calls) == 0 {
		t.Fatal("edit events did not read gateway status")
	}
	if err := svc.DiscardStaging(ctx); err != nil {
		t.Fatal(err)
	}
	rules, err = svc.ListRules(ctx)
	if err != nil || len(rules) != 0 {
		t.Fatal("discard did not restore original applied rules", err)
	}
}

func TestPortableExternalRenameCannotRewriteRuntimeDirectly(t *testing.T) {
	svc, runtime, orch := portableEditorFixture(t)
	before := []byte(`{"route":{"rules":[{"domain_suffix":["fixture.example"],"outbound":"old-exit"}]}}`)
	if err := orch.RestoreSlot(orchestrator.SlotRouter, before, false); err != nil {
		t.Fatal(err)
	}
	if err := svc.(*ServiceImpl).RenameExternalOutboundTag(context.Background(), "old-exit", "new-exit"); !errors.Is(err, errPortableUnavailable) {
		t.Fatal("external rename bypassed explicit runtime transaction", err)
	}
	after, _, err := orch.SnapshotSlot(orchestrator.SlotRouter)
	if err != nil || string(after) != string(before) {
		t.Fatal("rename wrote derived runtime outside transaction", err)
	}
	if len(runtime.calls) != 1 || runtime.calls[0] != "rename" {
		t.Fatal("rename transaction was not delegated")
	}
}

func TestPortableServiceRequiresExplicitRuntime(t *testing.T) {
	if svc, err := NewPortableService(PortableDeps{}); err == nil || svc != nil {
		t.Fatal("missing portable dependencies must not select the router platform")
	}
}

func TestPortableOriginalPresetsAndDNSRemainEditable(t *testing.T) {
	svc, _, orch := portableEditorFixture(t)
	ctx := context.Background()
	catalog, err := svc.ListPresets()
	if err != nil || len(catalog) < 10 {
		t.Fatal("original service catalog unavailable", err)
	}
	want := NewEmptyConfig()
	for _, id := range []string{"youtube", "telegram"} {
		var found *Preset
		for i := range catalog {
			if catalog[i].ID == id {
				found = &catalog[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("original catalog lost %s", id)
		}
		if err := ApplyPresetToConfig(want, *found, "direct"); err != nil {
			t.Fatal(err)
		}
		if err := svc.ApplyPreset(ctx, id, "direct"); err != nil {
			t.Fatal(err)
		}
	}
	rules, err := svc.ListRules(ctx)
	if err != nil || !reflect.DeepEqual(rules, want.Route.Rules) {
		t.Fatal("original preset rule projection changed", err)
	}
	sets, err := svc.ListRuleSets(ctx)
	if err != nil || !reflect.DeepEqual(sets, want.Route.RuleSet) {
		t.Fatal("original preset rule sets changed", err)
	}
	for _, tag := range []string{"fixture-dns-a", "fixture-dns-b"} {
		if err := svc.AddDNSServer(ctx, DNSServer{Tag: tag, Type: "udp", Server: "192.0.2.53"}); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.MoveDNSServer(ctx, 0, 1); err != nil {
		t.Fatal(err)
	}
	servers, err := svc.ListDNSServers(ctx)
	if err != nil || len(servers) != 2 || servers[0].Tag != "fixture-dns-b" {
		t.Fatal("original DNS order changed", err)
	}
	dnsRule := DNSRule{DomainSuffix: []string{"fixture.example"}, Server: "fixture-dns-b"}
	if err := svc.AddDNSRule(ctx, dnsRule); err != nil {
		t.Fatal(err)
	}
	dnsRules, err := svc.ListDNSRules(ctx)
	if err != nil || len(dnsRules) != 1 || !reflect.DeepEqual(dnsRules[0], dnsRule) {
		t.Fatal("original DNS rule changed", err)
	}
	if applied, err := orch.LoadApplied(orchestrator.SlotRouter); err != nil || len(applied) != 0 {
		t.Fatal("presets/DNS edit activated a legacy router slot", err)
	}
}
