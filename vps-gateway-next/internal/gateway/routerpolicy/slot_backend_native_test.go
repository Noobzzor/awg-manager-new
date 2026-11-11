package routerpolicy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

// The production backend retains and natively checks the real TUN projection.
// Only the process fixture substitutes loopback mixed ingress in a separate
// engine directory. This proves the adapter/engine control, not AWG/TUN E2E.
type projectedNativeProcess struct {
	source  string
	backend *nativeApplyBackend
	ctx     context.Context
}

func (p projectedNativeProcess) IsRunning() (bool, int) {
	return nativeOwnerProcess{backend: p.backend}.IsRunning()
}
func (p projectedNativeProcess) Stop() error  { p.backend.stop(); return nil }
func (p projectedNativeProcess) Start() error { return p.Reload() }
func (p projectedNativeProcess) Reload() error {
	target := filepath.Join(p.backend.dir, "config.d")
	if err := os.MkdirAll(target, 0700); err != nil {
		return err
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			if err := os.Remove(filepath.Join(target, entry.Name())); err != nil {
				return err
			}
		}
	}
	entries, err = os.ReadDir(p.source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(p.source, entry.Name()))
		if err != nil {
			return err
		}
		if entry.Name() == "18-z-gateway-policy.json" {
			var doc map[string]json.RawMessage
			if err := json.Unmarshal(raw, &doc); err != nil {
				return err
			}
			doc["inbounds"] = json.RawMessage(`[{"type":"mixed","tag":"awg-gateway-in","listen":"127.0.0.1","listen_port":10880}]`)
			raw, err = json.Marshal(doc)
			if err != nil {
				return err
			}
		}
		if err := os.WriteFile(filepath.Join(target, entry.Name()), raw, 0600); err != nil {
			return err
		}
	}
	return p.backend.Activate(p.ctx)
}

type crashAppliedDocuments struct {
	*router.FileEditorDocuments
	crash bool
}

func (d *crashAppliedDocuments) CommitApplied(raw []byte) error {
	if err := d.FileEditorDocuments.CommitApplied(raw); err != nil {
		return err
	}
	if d.crash {
		d.crash = false
		panic("simulated loss after canonical and split slot writes")
	}
	return nil
}

func TestNativeSlotApplyBackendAndRecovery(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_SLOT_PROOF") != "1" {
		t.Skip("requires isolated real-engine slot adapter control")
	}
	binary := os.Getenv("SINGBOX_BIN")
	if binary == "" {
		t.Fatal("SINGBOX_BIN required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "owned-slot-fixture") }))
	defer fixture.Close()
	dir := t.TempDir()
	source := filepath.Join(dir, "config.d")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "00-base.json"), []byte(`{"log":{"level":"error"},"outbounds":[{"type":"direct","tag":"direct"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	docs, err := router.OpenEditorDocuments(filepath.Join(dir, "editor"))
	if err != nil {
		t.Fatal(err)
	}
	engine := &nativeApplyBackend{dir: filepath.Join(dir, "engine"), binary: binary}
	defer engine.stop()
	process := projectedNativeProcess{source: source, backend: engine, ctx: ctx}
	confirm := func(ctx context.Context, s ApplyState) error {
		if running, _ := process.IsRunning(); !running {
			return errors.New("native projected engine stopped")
		}
		var cfg router.RouterConfig
		if err := json.Unmarshal(s.Editor, &cfg); err != nil {
			return err
		}
		want := 200
		if cfg.Route.Final == "block" {
			want = 502
		}
		proxy, _ := url.Parse("http://127.0.0.1:10880")
		transport := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: time.Second}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fixture.URL, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(req)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 100))
		if err != nil {
			return err
		}
		if response.StatusCode != want || (want == 200 && string(body) != "owned-slot-fixture") {
			return fmt.Errorf("native confirmation HTTP%d wanted%d", response.StatusCode, want)
		}
		return nil
	}
	fresh := func() (*SlotApplyBackend, *orchestrator.Orchestrator) {
		t.Helper()
		o := orchestrator.NewWithAppliedPath(source, process, filepath.Join(dir, "orch-applied.json"))
		t.Cleanup(o.Close)
		for _, meta := range orchestrator.KnownSlots() {
			if err := o.Register(meta); err != nil {
				t.Fatal(err)
			}
		}
		if err := o.Bootstrap(); err != nil {
			t.Fatal(err)
		}
		o.SetValidator(nativeOwnerValidator{binary})
		b, err := NewSlotApplyBackend(SlotApplyDeps{Orchestrator: o, Documents: docs, IngressInterface: "awg0", VerifyOwnership: func(context.Context) error { return nil }, ConfirmRunning: confirm})
		if err != nil {
			t.Fatal(err)
		}
		return b, o
	}
	b, o := fresh()
	guard := &sync.Mutex{}
	path := filepath.Join(dir, "apply.json")
	applier := func() *Applier {
		a, err := NewApplier(path, b, guard)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	a := applier()
	direct, err := b.Candidate(slotEditor(t, "direct"))
	if err != nil {
		t.Fatal(err)
	}
	block, err := b.Candidate(slotEditor(t, "block"))
	if err != nil {
		t.Fatal(err)
	}
	if out, err := a.Apply(ctx, direct); err != nil || !out.Applied {
		t.Fatal("native split baseline failed", err)
	}
	t.Log("NATIVE_SLOT canonical and split DIRECT accepted by native validator and running engine")
	tampered := cloneApplyState(direct)
	tampered.Editor = block.Editor
	if out, err := a.Apply(ctx, tampered); err == nil || out.Applied {
		t.Fatal("tampered native candidate accepted")
	}
	if err := confirm(ctx, direct); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_SLOT mismatched candidate refused; DIRECT retained")
	engine.failActivation = true
	if out, err := a.Apply(ctx, block); !errors.Is(err, fixtureApplyError) || out.Applied {
		t.Fatal("native failed activation accepted", err)
	}
	if err := confirm(ctx, direct); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_SLOT failed activation restored canonical and split DIRECT")
	syncState := b.syncSlotState
	b.syncSlotState = func() error { return fixtureApplyError }
	if out, err := a.Apply(ctx, block); !errors.Is(err, ErrApplyRecoveryRequired) || out.Applied {
		t.Fatal("native non-durable commit accepted", err)
	}
	if err := o.ReloadNow(); !errors.Is(err, orchestrator.ErrRuntimeRecoveryRequired) {
		t.Fatal("unresolved journal allowed ordinary reload", err)
	}
	b.syncSlotState = syncState
	if err := a.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if err := confirm(ctx, direct); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_SLOT durability failure blocks ordinary reload until confirmed recovery")
	crashDocs := &crashAppliedDocuments{FileEditorDocuments: docs, crash: true}
	b.deps.Documents = crashDocs
	crashed := false
	func() { defer func() { crashed = recover() != nil }(); _, _ = a.Apply(ctx, block) }()
	if !crashed {
		t.Fatal("prepared slot interruption did not run")
	}
	if err := o.ReloadNow(); !errors.Is(err, orchestrator.ErrRuntimeRecoveryRequired) {
		t.Fatal("interrupted slot writes escaped barrier", err)
	}
	engine.stop()
	o.Close()
	b, o = fresh()
	a = applier()
	if err := a.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if err := confirm(ctx, direct); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_SLOT fresh backend recovered prepared journal to canonical and split DIRECT")
	a.journal = journalFixture{applyJournalStore: a.journal, afterStore: func(j applyJournal) {
		if j.Phase == "accepted" {
			panic("simulated accepted split loss")
		}
	}}
	crashed = false
	func() { defer func() { crashed = recover() != nil }(); _, _ = a.Apply(ctx, block) }()
	if !crashed {
		t.Fatal("accepted slot interruption did not run")
	}
	engine.stop()
	o.Close()
	b, o = fresh()
	a = applier()
	if err := a.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if err := confirm(ctx, block); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_SLOT fresh backend completed accepted canonical and split BLOCK")
	if out, err := a.Apply(ctx, direct); err != nil || !out.Applied {
		t.Fatal("native final DIRECT failed", err)
	}
	if err := confirm(ctx, direct); err != nil {
		t.Fatal(err)
	}
	t.Log("native production slot backend journal and engine controls passed")
}
