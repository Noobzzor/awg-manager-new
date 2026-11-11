package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
)

var ErrRuntimeRecoveryRequired = errors.New("runtime reload blocked pending transaction recovery")

// RuntimeRecoveryRequired exposes the journal owner's barrier to artifact GC.
// It does not replace reading/recovering the journal before startup producers.
func (o *Orchestrator) RuntimeRecoveryRequired() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.runtimeRecoveryRequired
}

// SetRuntimeRecoveryRequired is controlled by the journal owner while holding
// its shared coordinator and runtime lease. Normal reloads must wait until
// explicit journal recovery has confirmed the intended running state.
func (o *Orchestrator) SetRuntimeRecoveryRequired(required bool) {
	o.mu.Lock()
	o.runtimeRecoveryRequired = required
	proc := o.proc
	o.mu.Unlock()
	if blocker, ok := proc.(interface{ SetRuntimeRecoveryRequired(bool) }); ok {
		blocker.SetRuntimeRecoveryRequired(required)
	}
}

// RuntimeLease serializes an application transaction against engine lifecycle
// operations. The caller must also coordinate all participating config writers.
// If an outer writer coordinator is configured, the caller owns it before
// acquiring this lease; explicit activation does not re-acquire that gate.
type RuntimeLease struct {
	o           *Orchestrator
	mu          sync.Mutex
	releaseHold func()
	closed      bool
	process     ProcessLease
}

// AcquireRuntimeLease waits for in-flight lifecycle work and prevents queued
// manual/debounced reloads from observing intermediate slot writes. Always
// defer Close. ctx controls acquisition only: rollback may need to reload
// after the initiating request has been cancelled.
func (o *Orchestrator) AcquireRuntimeLease(ctx context.Context) (*RuntimeLease, error) {
	if err := heavyop.Default.LockContext(ctx); err != nil {
		return nil, err
	}
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		heavyop.Default.Unlock()
		return nil, fmt.Errorf("orchestrator is closed")
	}
	release := o.holdReloadsLocked()
	proc := o.proc
	o.mu.Unlock()
	var process ProcessLease
	if provider, ok := proc.(ProcessLeaseProvider); ok {
		var err error
		process, err = provider.AcquireProcessLease(ctx)
		if err != nil || process == nil {
			if process != nil {
				process.Close()
			}
			release()
			heavyop.Default.Unlock()
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("process controller returned an empty lease")
		}
	}
	o.mu.Lock()
	closed := o.closed
	o.mu.Unlock()
	if closed {
		if process != nil {
			process.Close()
		}
		release()
		heavyop.Default.Unlock()
		return nil, fmt.Errorf("orchestrator is closed")
	}
	return &RuntimeLease{o: o, releaseHold: release, process: process}, nil
}

// ReloadNow applies final (or restored) state while retaining the lease.
// Call the lease method, not Orchestrator.ReloadNow, to avoid self-deadlock.
func (l *RuntimeLease) ReloadNow() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.o == nil {
		return fmt.Errorf("runtime lease is closed")
	}
	l.o.mu.Lock()
	closed := l.o.closed
	l.o.mu.Unlock()
	if closed {
		return fmt.Errorf("orchestrator is closed")
	}
	l.o.cancelPendingReload()
	return l.o.reloadWithRuntimeProcess(l.process)
}

func (l *RuntimeLease) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.o == nil {
		return
	}
	l.closed = true
	l.releaseHold()
	if l.process != nil {
		l.process.Close()
	}
	heavyop.Default.Unlock()
}
