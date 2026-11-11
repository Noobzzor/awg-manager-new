package routerpolicy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"
)

// ApplyState contains the original editor document and its derived runtime
// document. Nil bytes mean an absent file in a rollback snapshot.
type ApplyState struct {
	Editor  []byte `json:"editor"`
	Runtime []byte `json:"runtime"`
	Enabled bool   `json:"enabled"`
}

// ApplyBackend participates in the same coordinator as all config writers.
// Validate checks ownership and the complete merged configuration. Activate
// requests a runtime change; Confirm must verify the expected running state,
// not just a saved file or a successful signal. Backend errors must be safe to
// show without disclosing configuration contents.
type ApplyBackend interface {
	Validate(context.Context, ApplyState) error
	Snapshot(context.Context) (ApplyState, error)
	Commit(context.Context, ApplyState) error
	Activate(context.Context) error
	Confirm(context.Context, ApplyState) error
	Restore(context.Context, ApplyState) error
}

// ApplyLeaseBackend is required when a backend's engine can reload independently
// of the shared config-writer coordinator. Acquire after that coordinator and
// retain the lease through validation, journal recovery, commit and rollback.
// Activate must reload through the held lease, not reacquire its lifecycle lock.
// The returned release must be non-nil and must not depend on the request context.
type ApplyLeaseBackend interface {
	AcquireApplyLease(context.Context) (release func(), err error)
}

// ApplyRecoveryBackend retains a runtime reload barrier after an unresolved
// journal. Called while the writer coordinator and runtime lease are held.
// Only confirmed journal recovery/cleanup may clear it.
type ApplyRecoveryBackend interface {
	SetRecoveryRequired(bool)
}

type ApplyOutcome struct {
	// Applied means this attempt confirmed the candidate running. A non-nil
	// error still means acceptance/cleanup is incomplete: callers must retain
	// an explicit recovery state and must not acknowledge a complete apply.
	Applied        bool
	CleanupPending bool
}

type applyJournal struct {
	Version int        `json:"version"`
	Phase   string     `json:"phase"`
	Before  ApplyState `json:"before"`
	After   ApplyState `json:"after"`
}

type applyJournalStore interface {
	Load() (*applyJournal, error)
	Store(applyJournal) error
	Clear() error
}

type Applier struct {
	backend ApplyBackend
	guard   sync.Locker
	journal applyJournalStore
}

var ErrApplyRecoveryRequired = errors.New("router application requires recovery")

func NewApplier(path string, backend ApplyBackend, guard sync.Locker) (*Applier, error) {
	if !filepath.IsAbs(path) || backend == nil || guard == nil {
		return nil, fmt.Errorf("router applier requires an absolute journal path, backend and shared coordinator")
	}
	return &Applier{backend: backend, guard: guard, journal: fileApplyJournal{path: path}}, nil
}
func (a *Applier) Apply(ctx context.Context, state ApplyState) (ApplyOutcome, error) {
	return a.ApplyCurrent(ctx, func() (ApplyState, error) { return state, nil })
}

// ApplyCurrent reads authoring state while holding the writer and runtime leases,
// after recovering any earlier transaction. The reader must not reacquire them.
func (a *Applier) ApplyCurrent(ctx context.Context, read func() (ApplyState, error)) (outcome ApplyOutcome, applyErr error) {
	if read == nil {
		return ApplyOutcome{}, fmt.Errorf("router candidate reader required")
	}
	a.guard.Lock()
	defer a.guard.Unlock()
	if err := ctx.Err(); err != nil {
		return ApplyOutcome{}, err
	}
	release, err := a.acquireLease(ctx)
	if err != nil {
		return ApplyOutcome{}, err
	}
	defer func() { a.finishLease(release, applyErr) }()
	if err := a.recoverLocked(ctx); err != nil {
		return ApplyOutcome{}, err
	}
	state, err := read()
	if err != nil {
		return ApplyOutcome{}, err
	}
	state = cloneApplyState(state)
	if err := validApplyState(state, true); err != nil {
		return ApplyOutcome{}, err
	}
	if err := a.backend.Validate(ctx, cloneApplyState(state)); err != nil {
		return ApplyOutcome{}, fmt.Errorf("validate router candidate: %w", err)
	}
	before, err := a.backend.Snapshot(ctx)
	if err != nil {
		return ApplyOutcome{}, fmt.Errorf("snapshot router state: %w", err)
	}
	j := applyJournal{Version: 1, Phase: "prepared", Before: cloneApplyState(before), After: state}
	if err := ctx.Err(); err != nil {
		return ApplyOutcome{}, err
	}
	if err := a.journal.Store(j); err != nil {
		return ApplyOutcome{}, fmt.Errorf("prepare router recovery journal: %w", err)
	}
	fail := func(cause error) (ApplyOutcome, error) {
		// Cancellation of the caller must not cancel restoration of working
		// state. Recovery is bounded and leaves the journal on any failure.
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		return ApplyOutcome{}, errors.Join(cause, a.restoreLocked(rctx, j))
	}
	if err := a.backend.Commit(ctx, cloneApplyState(state)); err != nil {
		return fail(fmt.Errorf("commit router candidate: %w", err))
	}
	if err := a.backend.Activate(ctx); err != nil {
		return fail(fmt.Errorf("activate router candidate: %w", err))
	}
	if err := a.backend.Confirm(ctx, cloneApplyState(state)); err != nil {
		return fail(fmt.Errorf("confirm router candidate: %w", err))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	j.Phase = "accepted"
	if err := a.journal.Store(j); err != nil {
		// The runtime is confirmed, but the journal write may have failed
		// before OR after replacement. Do not guess its durable phase or
		// erase it. Recovery resolves prepared versus accepted explicitly.
		return ApplyOutcome{Applied: true, CleanupPending: true}, errors.Join(ErrApplyRecoveryRequired, fmt.Errorf("record router acceptance: %w", err))
	}
	if err := a.journal.Clear(); err != nil {
		return ApplyOutcome{Applied: true, CleanupPending: true}, errors.Join(ErrApplyRecoveryRequired, fmt.Errorf("finish router journal: %w", err))
	}
	return ApplyOutcome{Applied: true}, nil
}

func (a *Applier) Recover(ctx context.Context) (recoveryErr error) {
	a.guard.Lock()
	defer a.guard.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	release, err := a.acquireLease(ctx)
	if err != nil {
		return err
	}
	defer func() { a.finishLease(release, recoveryErr) }()
	return a.recoverLocked(ctx)
}

func (a *Applier) finishLease(release func(), operationErr error) {
	defer release()
	if backend, ok := a.backend.(ApplyRecoveryBackend); ok {
		journal, err := a.journal.Load()
		// Even a removed file may have an uncertain durable deletion. Keep
		// the barrier on explicit recovery errors, not merely file presence.
		backend.SetRecoveryRequired(err != nil || journal != nil || errors.Is(operationErr, ErrApplyRecoveryRequired))
	}
}

func (a *Applier) acquireLease(ctx context.Context) (func(), error) {
	backend, ok := a.backend.(ApplyLeaseBackend)
	if !ok {
		// Backends without independent lifecycle operations need only the
		// shared writer coordinator (e.g. an isolated synchronous engine).
		return func() {}, nil
	}
	release, err := backend.AcquireApplyLease(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire router runtime lease: %w", err)
	}
	if release == nil {
		return nil, fmt.Errorf("router backend returned an empty runtime lease")
	}
	return release, nil
}

func (a *Applier) recoverLocked(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	j, err := a.journal.Load()
	if err != nil {
		return errors.Join(ErrApplyRecoveryRequired, err)
	}
	if j == nil {
		return nil
	}
	if j.Phase == "prepared" {
		return a.restoreLocked(ctx, *j)
	}
	// The previous attempt durably accepted this state. Finish it rather
	// than rolling an acknowledged configuration back after a restart.
	if err := a.backend.Validate(ctx, cloneApplyState(j.After)); err != nil {
		return errors.Join(ErrApplyRecoveryRequired, err)
	}
	if err := a.backend.Commit(ctx, cloneApplyState(j.After)); err != nil {
		return errors.Join(ErrApplyRecoveryRequired, err)
	}
	if err := a.backend.Activate(ctx); err != nil {
		return errors.Join(ErrApplyRecoveryRequired, err)
	}
	if err := a.backend.Confirm(ctx, cloneApplyState(j.After)); err != nil {
		return errors.Join(ErrApplyRecoveryRequired, err)
	}
	if err := a.journal.Clear(); err != nil {
		return errors.Join(ErrApplyRecoveryRequired, err)
	}
	return nil
}

func (a *Applier) restoreLocked(ctx context.Context, j applyJournal) error {
	if err := a.backend.Restore(ctx, cloneApplyState(j.Before)); err != nil {
		return errors.Join(ErrApplyRecoveryRequired, err)
	}
	if err := a.backend.Activate(ctx); err != nil {
		return errors.Join(ErrApplyRecoveryRequired, err)
	}
	if err := a.backend.Confirm(ctx, cloneApplyState(j.Before)); err != nil {
		return errors.Join(ErrApplyRecoveryRequired, err)
	}
	if err := a.journal.Clear(); err != nil {
		return errors.Join(ErrApplyRecoveryRequired, err)
	}
	return nil
}

func cloneApplyState(v ApplyState) ApplyState {
	if v.Editor != nil {
		v.Editor = append([]byte{}, v.Editor...)
	}
	if v.Runtime != nil {
		v.Runtime = append([]byte{}, v.Runtime...)
	}
	return v
}

func validApplyState(v ApplyState, candidate bool) error {
	if candidate && (v.Editor == nil || v.Runtime == nil) {
		return fmt.Errorf("router candidate documents are missing")
	}
	if v.Enabled && v.Runtime == nil {
		return fmt.Errorf("enabled router runtime document is missing")
	}
	for _, data := range [][]byte{v.Editor, v.Runtime} {
		if data != nil && !json.Valid(data) {
			return fmt.Errorf("router state contains invalid JSON")
		}
	}
	return nil
}
