package routerpolicy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

type slotValidatorFixture struct{ calls int }

func (v *slotValidatorFixture) Validate(context.Context, string) error { v.calls++; return nil }

type slotProcessFixture struct {
	running     bool
	activations int
}

func (p *slotProcessFixture) IsRunning() (bool, int) { return p.running, 1 }
func (p *slotProcessFixture) Start() error           { p.running = true; p.activations++; return nil }
func (p *slotProcessFixture) Reload() error          { p.activations++; return nil }
func (p *slotProcessFixture) Stop() error            { p.running = false; return nil }

func slotBackendFixture(t *testing.T) (*SlotApplyBackend, *orchestrator.Orchestrator, *router.FileEditorDocuments, *slotValidatorFixture, *slotProcessFixture) {
	t.Helper()
	dir := t.TempDir()
	process := &slotProcessFixture{}
	o := orchestrator.NewWithAppliedPath(dir, process, filepath.Join(t.TempDir(), "applied.json"))
	t.Cleanup(o.Close)
	for _, m := range orchestrator.KnownSlots() {
		if err := o.Register(m); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "00-base.json"), []byte(`{"outbounds":[{"type":"direct","tag":"direct"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := o.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	v := &slotValidatorFixture{}
	o.SetValidator(v)
	docs, err := router.OpenEditorDocuments(filepath.Join(t.TempDir(), "editor"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewSlotApplyBackend(SlotApplyDeps{Orchestrator: o, Documents: docs, IngressInterface: "awg0", VerifyOwnership: func(context.Context) error { return nil }, ConfirmRunning: func(context.Context, ApplyState) error {
		if !process.running {
			return fixtureApplyError
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	return b, o, docs, v, process
}
func slotEditor(t *testing.T, final string) []byte {
	t.Helper()
	cfg := router.NewEmptyConfig()
	cfg.Route.Final = final
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func slotApplier(t *testing.T, b *SlotApplyBackend) *Applier {
	t.Helper()
	a, err := NewApplier(filepath.Join(t.TempDir(), "journal.json"), b, &sync.Mutex{})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestSlotBackendCommitsCanonicalAndBothDerivedSlots(t *testing.T) {
	b, o, docs, v, process := slotBackendFixture(t)
	editor := slotEditor(t, "block")
	candidate, err := b.Candidate(editor)
	if err != nil {
		t.Fatal(err)
	}
	if err := docs.SaveDraft(slotEditor(t, "direct")); err != nil {
		t.Fatal(err)
	}
	out, err := slotApplier(t, b).Apply(context.Background(), candidate)
	if err != nil || !out.Applied || process.activations != 1 || v.calls != 1 {
		t.Fatalf("apply failed: %+v %v", out, err)
	}
	got, err := docs.LoadApplied()
	if err != nil || !bytes.Equal(got, editor) {
		t.Fatal("canonical editor not committed", err)
	}
	effective, _ := docs.LoadEffective()
	if bytes.Equal(effective, got) {
		t.Fatal("apply silently discarded a later draft")
	}
	var cfg router.RouterConfig
	_ = json.Unmarshal(editor, &cfg)
	slots, err := RenderSlots("awg0", &cfg)
	if err != nil {
		t.Fatal(err)
	}
	for slot, want := range map[orchestrator.Slot][]byte{orchestrator.SlotGatewayPolicy: slots.Ingress, orchestrator.SlotRouter: slots.Routing} {
		raw, enabled, err := o.SnapshotSlot(slot)
		if err != nil || !enabled || !bytes.Equal(raw, want) {
			t.Fatal("derived slot mismatch", slot, err)
		}
	}
}

func TestSlotBackendRequiresDurableSlotWritesBeforeAcceptance(t *testing.T) {
	b, _, _, _, _ := slotBackendFixture(t)
	b.syncSlotState = func() error { return fixtureApplyError }
	candidate, err := b.Candidate(slotEditor(t, "direct"))
	if err != nil {
		t.Fatal(err)
	}
	a := slotApplier(t, b)
	out, err := a.Apply(context.Background(), candidate)
	if out.Applied || !errors.Is(err, fixtureApplyError) || !errors.Is(err, ErrApplyRecoveryRequired) {
		t.Fatalf("non-durable slots accepted: %+v %v", out, err)
	}
	if j, err := a.journal.Load(); err != nil || j == nil {
		t.Fatal("failed durability lost recovery journal", err)
	}
}

func TestSlotBackendRollbackRestoresExactSplitSnapshot(t *testing.T) {
	b, o, docs, _, _ := slotBackendFixture(t)
	ctx := context.Background()
	// A pre-existing parked router and absent ingress must survive rollback.
	parked := []byte("{\"route\":{\"rules\":[]}}\n")
	release := o.HoldReloads()
	defer release()
	if err := o.RestoreSlot(orchestrator.SlotRouter, parked, false); err != nil {
		t.Fatal(err)
	}
	oldEditor := slotEditor(t, "direct")
	if err := docs.CommitApplied(oldEditor); err != nil {
		t.Fatal(err)
	}
	checks := 0
	b.deps.ConfirmRunning = func(context.Context, ApplyState) error {
		checks++
		if checks == 1 {
			return fixtureApplyError
		}
		return nil
	}
	next, err := b.Candidate(slotEditor(t, "block"))
	if err != nil {
		t.Fatal(err)
	}
	if out, err := slotApplier(t, b).Apply(ctx, next); !errors.Is(err, fixtureApplyError) || out.Applied {
		t.Fatal("confirmation failure accepted", err)
	}
	ingress, enabled, err := o.SnapshotSlot(orchestrator.SlotGatewayPolicy)
	if err != nil || ingress != nil || enabled {
		t.Fatal("absent ingress not restored", err)
	}
	raw, enabled, err := o.SnapshotSlot(orchestrator.SlotRouter)
	if err != nil || enabled || !bytes.Equal(raw, parked) {
		t.Fatal("parked router changed", err)
	}
	got, _ := docs.LoadApplied()
	if !bytes.Equal(got, oldEditor) || checks != 2 {
		t.Fatal("canonical snapshot or rollback confirmation lost")
	}
}

func TestSlotBackendRefusesMissingNativeValidatorAndForeignOwner(t *testing.T) {
	for _, mode := range []string{"no_native", "foreign_owner"} {
		t.Run(mode, func(t *testing.T) {
			b, o, docs, _, process := slotBackendFixture(t)
			next, err := b.Candidate(slotEditor(t, "direct"))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "no_native" {
				o.SetValidator(nil)
			} else {
				b.deps.VerifyOwnership = func(context.Context) error { return fixtureApplyError }
			}
			if out, err := slotApplier(t, b).Apply(context.Background(), next); err == nil || out.Applied {
				t.Fatal("unsafe candidate accepted")
			}
			applied, _ := docs.LoadApplied()
			if applied != nil || process.activations != 0 {
				t.Fatal("refusal mutated state")
			}
		})
	}
}

func TestSlotBackendRefusesCandidateTampering(t *testing.T) {
	b, _, _, _, process := slotBackendFixture(t)
	next, err := b.Candidate(slotEditor(t, "direct"))
	if err != nil {
		t.Fatal(err)
	}
	next.Editor = slotEditor(t, "block")
	if out, err := slotApplier(t, b).Apply(context.Background(), next); err == nil || out.Applied {
		t.Fatal("mismatched editor/runtime accepted")
	}
	if process.activations != 0 {
		t.Fatal("tampered candidate touched engine")
	}
}

func TestSlotBackendRequiresLeaseForStateOperations(t *testing.T) {
	b, _, _, _, _ := slotBackendFixture(t)
	if _, err := b.Snapshot(context.Background()); err == nil {
		t.Fatal("snapshot outside transaction accepted")
	}
	if err := b.Activate(context.Background()); err == nil {
		t.Fatal("activation outside transaction accepted")
	}
}

func TestSlotBackendRequiresExplicitOwnershipAndLiveVerifier(t *testing.T) {
	b, _, _, _, _ := slotBackendFixture(t)
	for _, field := range []string{"owner", "live"} {
		t.Run(field, func(t *testing.T) {
			d := b.deps
			if field == "owner" {
				d.VerifyOwnership = nil
			} else {
				d.ConfirmRunning = nil
			}
			if _, err := NewSlotApplyBackend(d); err == nil {
				t.Fatal("incomplete product dependencies accepted")
			}
		})
	}
}

func TestSlotBackendPreparedRecoveryRestoresBothSlotsAndAbsence(t *testing.T) {
	b, _, docs, _, _ := slotBackendFixture(t)
	ctx := context.Background()
	release, err := b.AcquireApplyLease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	old, err := b.Snapshot(ctx)
	release()
	if err != nil {
		t.Fatal(err)
	}
	next, err := b.Candidate(slotEditor(t, "direct"))
	if err != nil {
		t.Fatal(err)
	}
	release, err = b.AcquireApplyLease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Commit(ctx, next); err != nil {
		release()
		t.Fatal(err)
	}
	release()
	a := slotApplier(t, b)
	if err := a.journal.Store(applyJournal{Version: 1, Phase: "prepared", Before: old, After: next}); err != nil {
		t.Fatal(err)
	}
	b.deps.ConfirmRunning = func(context.Context, ApplyState) error { return nil } // expected absent runtime, deliberately stopped
	if err := a.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	release, err = b.AcquireApplyLease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	got, err := b.Snapshot(ctx)
	if err != nil || !reflect.DeepEqual(got, old) {
		t.Fatal("prepared split recovery differs from snapshot", err)
	}
	applied, _ := docs.LoadApplied()
	if applied != nil {
		t.Fatal("absent canonical document materialized")
	}
}
