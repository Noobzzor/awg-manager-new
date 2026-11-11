package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type leaseProcessFixture struct {
	running atomic.Bool
	started chan struct{}
}

func (p *leaseProcessFixture) IsRunning() (bool, int) { return p.running.Load(), 123 }
func (p *leaseProcessFixture) Start() error {
	p.running.Store(true)
	p.started <- struct{}{}
	return nil
}
func (p *leaseProcessFixture) Stop() error   { p.running.Store(false); return nil }
func (p *leaseProcessFixture) Reload() error { return nil }
func leaseOrchestrator(t *testing.T) (*Orchestrator, *leaseProcessFixture) {
	t.Helper()
	dir := t.TempDir()
	p := &leaseProcessFixture{started: make(chan struct{}, 4)}
	o := NewWithAppliedPath(dir, p, filepath.Join(t.TempDir(), "applied.json"))
	t.Cleanup(o.Close)
	if err := o.Register(SlotMeta{Slot: SlotRouter, Filename: "20-router.json"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "20-router.json"), []byte(`{"route":{"rules":[{"network":"tcp","outbound":"direct"}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := o.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	return o, p
}

func TestRuntimeLeaseExcludesConcurrentReload(t *testing.T) {
	o, p := leaseOrchestrator(t)
	lease, err := o.AcquireRuntimeLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	entered, done := make(chan struct{}), make(chan error, 1)
	go func() { close(entered); done <- o.ReloadNow() }()
	<-entered
	select {
	case <-p.started:
		t.Fatal("engine reloaded inside application lease")
	case <-time.After(50 * time.Millisecond):
	}
	lease.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued reload did not resume after lease release")
	}
	if !p.running.Load() {
		t.Fatal("queued engine reload was lost")
	}
}

func TestRuntimeLeaseExplicitReloadAndExpiry(t *testing.T) {
	o, p := leaseOrchestrator(t)
	lease, err := o.AcquireRuntimeLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	done := make(chan error, 1)
	go func() { done <- lease.ReloadNow() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("explicit reload deadlocked on its own runtime lease")
	}
	if !p.running.Load() {
		t.Fatal("explicit apply did not start engine")
	}
	lease.Close()
	lease.Close()
	if err := lease.ReloadNow(); err == nil {
		t.Fatal("expired runtime lease still authorizes reload")
	}
}

func TestRuntimeLeaseCancellationDoesNotRetainLifecycleLock(t *testing.T) {
	o, _ := leaseOrchestrator(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if lease, err := o.AcquireRuntimeLease(ctx); err == nil || lease != nil {
		if lease != nil {
			lease.Close()
		}
		t.Fatal("cancelled transaction acquired runtime lease")
	}
	done := make(chan error, 1)
	go func() { done <- o.ReloadNow() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled acquisition retained lifecycle lock")
	}
}

func TestRuntimeLeaseQueuedAcquisitionCanBeCancelled(t *testing.T) {
	o, _ := leaseOrchestrator(t)
	first, err := o.AcquireRuntimeLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		lease, err := o.AcquireRuntimeLease(ctx)
		if lease != nil {
			lease.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatal("second transaction bypassed held lifecycle lease", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatal("queued cancellation ignored", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled transaction still waiting for lifecycle lock")
	}
}

func TestRuntimeLeaseSuppressesAutomaticReloadDuringWrites(t *testing.T) {
	o, p := leaseOrchestrator(t)
	lease, err := o.AcquireRuntimeLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err := o.Save(SlotRouter, []byte(`{"route":{"rules":[{"network":"udp","outbound":"direct"}]}}`)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.started:
		t.Fatal("debounced reload ran during staged runtime writes")
	case <-time.After(350 * time.Millisecond):
	}
	if err := lease.ReloadNow(); err != nil {
		t.Fatal(err)
	}
	if !p.running.Load() {
		t.Fatal("explicit final reload did not start engine")
	}
}

func TestRuntimeLeaseRefusesClosedOwner(t *testing.T) {
	o, _ := leaseOrchestrator(t)
	o.Close()
	if lease, err := o.AcquireRuntimeLease(context.Background()); err == nil || lease != nil {
		if lease != nil {
			lease.Close()
		}
		t.Fatal("closed owner authorized runtime transaction")
	}
}

func TestRuntimeLeaseRecoveryBarrierBlocksNormalReload(t *testing.T) {
	o, p := leaseOrchestrator(t)
	lease, err := o.AcquireRuntimeLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	o.SetRuntimeRecoveryRequired(true)
	lease.Close()
	if err := o.ReloadNow(); err != ErrRuntimeRecoveryRequired || p.running.Load() {
		t.Fatal("normal reload bypassed incomplete recovery", err)
	}
	lease, err = o.AcquireRuntimeLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err := lease.ReloadNow(); err != nil {
		t.Fatal("explicit recovery blocked by its own barrier", err)
	}
	o.SetRuntimeRecoveryRequired(false)
	lease.Close()
	if err := o.ReloadNow(); err != nil {
		t.Fatal("confirmed recovery did not unblock reload", err)
	}
}
