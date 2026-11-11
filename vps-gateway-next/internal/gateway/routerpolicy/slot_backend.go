package routerpolicy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

type AppliedEditorDocuments interface {
	LoadApplied() ([]byte, error)
	CommitApplied([]byte) error
}

type SlotApplyDeps struct {
	// CheckProjectedRouting admits file inputs before the native checker.
	CheckProjectedRouting func([]byte) error
	Orchestrator          *orchestrator.Orchestrator
	Documents             AppliedEditorDocuments
	IngressInterface      string
	// Called with the shared writer coordinator and runtime lease held. Must
	// reject another policy owner; must not acquire those locks recursively.
	VerifyOwnership func(context.Context) error
	// Must verify the actual expected running configuration, not only files or
	// ReloadNow success. Deployment-specific live verification is mandatory.
	ConfirmRunning func(context.Context, ApplyState) error
}

// SlotApplyBackend journals canonical applied authoring plus exact18/20 slot
// bytes and enabled state. It never owns19 or discards a draft. Startup must
// wire the common writer coordinator, exclusive owner and recovery before
// enabling the original editor or running artifact GC.
type SlotApplyBackend struct {
	deps          SlotApplyDeps
	syncSlotState func() error
	mu            sync.Mutex
	acquiring     bool
	lease         *orchestrator.RuntimeLease
}

type appliedSlot struct {
	Data    []byte `json:"data"`
	Enabled bool   `json:"enabled"`
}
type appliedSlots struct {
	Version int         `json:"version"`
	Ingress appliedSlot `json:"ingress"`
	Routing appliedSlot `json:"routing"`
}

func NewSlotApplyBackend(d SlotApplyDeps) (*SlotApplyBackend, error) {
	if d.Orchestrator == nil || d.Documents == nil || d.VerifyOwnership == nil || d.ConfirmRunning == nil {
		return nil, fmt.Errorf("slot apply requires orchestrator, documents, ownership and live verification")
	}
	if _, err := RenderSlots(d.IngressInterface, router.NewEmptyConfig()); err != nil {
		return nil, fmt.Errorf("invalid slot ingress")
	}
	return &SlotApplyBackend{deps: d, syncSlotState: d.Orchestrator.SyncRuntimeState}, nil
}

// Candidate is read-only; it does not imply native validation or activation.
func (b *SlotApplyBackend) Candidate(editor []byte) (ApplyState, error) {
	var cfg *router.RouterConfig
	if err := json.Unmarshal(editor, &cfg); err != nil || cfg == nil {
		return ApplyState{}, fmt.Errorf("invalid canonical router document")
	}
	slots, err := RenderSlots(b.deps.IngressInterface, cfg)
	if err != nil {
		return ApplyState{}, err
	}
	raw, err := json.Marshal(appliedSlots{Version: 1, Ingress: appliedSlot{Data: slots.Ingress, Enabled: true}, Routing: appliedSlot{Data: slots.Routing, Enabled: true}})
	if err != nil {
		return ApplyState{}, err
	}
	return ApplyState{Editor: append([]byte{}, editor...), Runtime: raw, Enabled: true}, nil
}

func (b *SlotApplyBackend) AcquireApplyLease(ctx context.Context) (func(), error) {
	b.mu.Lock()
	if b.acquiring || b.lease != nil {
		b.mu.Unlock()
		return nil, fmt.Errorf("slot transaction already active")
	}
	b.acquiring = true
	b.mu.Unlock()
	lease, err := b.deps.Orchestrator.AcquireRuntimeLease(ctx)
	b.mu.Lock()
	b.acquiring = false
	b.lease = lease
	b.mu.Unlock()
	if err != nil {
		return nil, err
	}
	var once sync.Once
	release := func() { once.Do(func() { b.mu.Lock(); b.lease = nil; b.mu.Unlock(); lease.Close() }) }
	if err := b.deps.VerifyOwnership(ctx); err != nil {
		release()
		return nil, err
	}
	return release, nil
}

func (b *SlotApplyBackend) requireLease() (*orchestrator.RuntimeLease, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.lease == nil {
		return nil, fmt.Errorf("slot operation requires runtime lease")
	}
	return b.lease, nil
}

func (b *SlotApplyBackend) SetRecoveryRequired(required bool) {
	b.deps.Orchestrator.SetRuntimeRecoveryRequired(required)
}

func decodeAppliedSlots(s ApplyState) (appliedSlots, error) {
	var slots appliedSlots
	decoder := json.NewDecoder(bytes.NewReader(s.Runtime))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&slots); err != nil {
		return slots, fmt.Errorf("invalid slot snapshot")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF || slots.Version != 1 {
		return slots, fmt.Errorf("invalid slot snapshot version or trailing data")
	}
	for _, slot := range []appliedSlot{slots.Ingress, slots.Routing} {
		if slot.Enabled && slot.Data == nil {
			return slots, fmt.Errorf("enabled snapshot slot is absent")
		}
	}
	if s.Enabled != (slots.Ingress.Enabled || slots.Routing.Enabled) {
		return slots, fmt.Errorf("slot snapshot enabled state mismatch")
	}
	return slots, nil
}

func (b *SlotApplyBackend) Validate(ctx context.Context, s ApplyState) error {
	if _, err := b.requireLease(); err != nil {
		return err
	}
	if err := b.deps.VerifyOwnership(ctx); err != nil {
		return err
	}
	slots, err := decodeAppliedSlots(s)
	if err != nil {
		return err
	}
	expected, err := b.Candidate(s.Editor)
	if err != nil {
		return err
	}
	if !bytes.Equal(s.Runtime, expected.Runtime) || s.Enabled != expected.Enabled {
		return fmt.Errorf("runtime slots do not match canonical authoring")
	}
	if b.deps.CheckProjectedRouting != nil {
		if err := b.deps.CheckProjectedRouting(slots.Routing.Data); err != nil {
			return err
		}
	}
	res, err := b.deps.Orchestrator.CheckMergedBatchNative(ctx, map[orchestrator.Slot]orchestrator.SlotCandidate{
		orchestrator.SlotGatewayPolicy: {Data: slots.Ingress.Data, Enabled: slots.Ingress.Enabled},
		orchestrator.SlotRouter:        {Data: slots.Routing.Data, Enabled: slots.Routing.Enabled},
	})
	if err != nil {
		return err
	}
	if !res.Ok() {
		return res
	}
	return nil
}

func (b *SlotApplyBackend) Snapshot(ctx context.Context) (ApplyState, error) {
	if _, err := b.requireLease(); err != nil {
		return ApplyState{}, err
	}
	if err := ctx.Err(); err != nil {
		return ApplyState{}, err
	}
	editor, err := b.deps.Documents.LoadApplied()
	if err != nil {
		return ApplyState{}, err
	}
	ingress, ie, err := b.deps.Orchestrator.SnapshotSlot(orchestrator.SlotGatewayPolicy)
	if err != nil {
		return ApplyState{}, err
	}
	routing, re, err := b.deps.Orchestrator.SnapshotSlot(orchestrator.SlotRouter)
	if err != nil {
		return ApplyState{}, err
	}
	raw, err := json.Marshal(appliedSlots{Version: 1, Ingress: appliedSlot{Data: ingress, Enabled: ie}, Routing: appliedSlot{Data: routing, Enabled: re}})
	return ApplyState{Editor: editor, Runtime: raw, Enabled: ie || re}, err
}

func (b *SlotApplyBackend) Commit(ctx context.Context, s ApplyState) error  { return b.write(ctx, s) }
func (b *SlotApplyBackend) Restore(ctx context.Context, s ApplyState) error { return b.write(ctx, s) }
func (b *SlotApplyBackend) write(ctx context.Context, s ApplyState) error {
	if _, err := b.requireLease(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := b.deps.VerifyOwnership(ctx); err != nil {
		return err
	}
	slots, err := decodeAppliedSlots(s)
	if err != nil {
		return err
	}
	if err := b.deps.Orchestrator.RestoreSlot(orchestrator.SlotGatewayPolicy, slots.Ingress.Data, slots.Ingress.Enabled); err != nil {
		return err
	}
	if err := b.deps.Orchestrator.RestoreSlot(orchestrator.SlotRouter, slots.Routing.Data, slots.Routing.Enabled); err != nil {
		return err
	}
	if err := b.syncSlotState(); err != nil {
		return err
	}
	return b.deps.Documents.CommitApplied(s.Editor)
}

func (b *SlotApplyBackend) Activate(ctx context.Context) error {
	lease, err := b.requireLease()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return lease.ReloadNow()
}

func (b *SlotApplyBackend) Confirm(ctx context.Context, want ApplyState) error {
	got, err := b.Snapshot(ctx)
	if err != nil {
		return err
	}
	if !bytes.Equal(got.Editor, want.Editor) || !bytes.Equal(got.Runtime, want.Runtime) || got.Enabled != want.Enabled {
		return fmt.Errorf("applied slot snapshot differs from expected state")
	}
	return b.deps.ConfirmRunning(ctx, cloneApplyState(want))
}
