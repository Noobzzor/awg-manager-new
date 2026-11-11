package singbox

import (
	"context"
	"fmt"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"sync"
)

type processLease struct {
	p      *Process
	mu     sync.Mutex
	closed bool
}

// AcquireProcessLease excludes all public process mutation calls, including
// Operator/watchdog callers that do not go through the orchestrator. Acquisition
// is cancellable; the held lease survives cancellation for bounded rollback.
// Lock order is the global heavy-operation gate, then this process gate, then
// startMu. A direct lease owner must use its leased mutation methods and must
// not acquire the global gate while holding this lease.
func (p *Process) AcquireProcessLease(ctx context.Context) (orchestrator.ProcessLease, error) {
	if err := p.mutationGate.LockContext(ctx); err != nil {
		return nil, err
	}
	return &processLease{p: p}, nil
}
func (p *Process) SetRuntimeRecoveryRequired(required bool) { p.recoveryRequired.Store(required) }
func (l *processLease) IsRunning() (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.p == nil {
		return false, 0
	}
	return l.p.IsRunning()
}
func (l *processLease) Start() error {
	_, err := l.StartSpawned()
	return err
}

func (l *processLease) StartSpawned() (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.p == nil {
		return false, fmt.Errorf("process lease is closed")
	}
	return l.p.startSpawnedLeased()
}
func (l *processLease) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.p == nil {
		return fmt.Errorf("process lease is closed")
	}
	return l.p.stopLeased()
}
func (l *processLease) Reload() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.p == nil {
		return fmt.Errorf("process lease is closed")
	}
	return l.p.reloadLeased()
}
func (l *processLease) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.p == nil {
		return
	}
	l.closed = true
	l.p.mutationGate.Unlock()
}
