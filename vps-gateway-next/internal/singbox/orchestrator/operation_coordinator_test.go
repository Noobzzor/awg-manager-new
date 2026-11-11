package orchestrator

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
)

type observedWriterCoordinator struct {
	heavyop.Gate
	entered chan struct{}
	once    sync.Once
}

func (g *observedWriterCoordinator) LockContext(ctx context.Context) error {
	g.once.Do(func() { close(g.entered) })
	return g.Gate.LockContext(ctx)
}
func writerCoordinatedOrchestrator(t *testing.T) (*Orchestrator, *leaseProcessFixture, *observedWriterCoordinator) {
	o, p := leaseOrchestrator(t)
	g := &observedWriterCoordinator{entered: make(chan struct{})}
	o.SetOperationCoordinator(g)
	return o, p, g
}

func TestWriterCoordinatedReloadCancelledBeforeActivation(t *testing.T) {
	o, p, g := writerCoordinatedOrchestrator(t)
	g.Lock()
	defer g.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if err := o.ReloadContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("reload escaped writer admission:%v", err)
	}
	if p.running.Load() {
		t.Fatal("cancelled admission activated engine")
	}
}

func TestWriterCoordinatedReloadWaitsBeforeHeavyOperationGate(t *testing.T) {
	o, p, g := writerCoordinatedOrchestrator(t)
	g.Lock()
	defer g.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- o.ReloadContext(ctx) }()
	select {
	case <-g.entered:
	case <-p.started:
		t.Fatal("reload skipped writer gate")
	case <-time.After(time.Second):
		t.Fatal("reload never attempted writer admission")
	}
	if !heavyop.Default.TryLock() {
		t.Fatal("queued writer already monopolized heavy-operation gate")
	}
	heavyop.Default.Unlock()
	if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued cancellation:%v", err)
	}
}

func TestWriterCoordinatedLegacyReloadResumesAfterCommit(t *testing.T) {
	o, p, g := writerCoordinatedOrchestrator(t)
	g.Lock()
	held := true
	defer func() {
		if held {
			g.Unlock()
		}
	}()
	done := make(chan error, 1)
	go func() { done <- o.Reload() }()
	select {
	case <-g.entered:
	case <-p.started:
		t.Fatal("ordinary reload activated during producer operation")
	case <-time.After(time.Second):
		t.Fatal("ordinary reload did not queue")
	}
	select {
	case <-p.started:
		t.Fatal("engine started while producer gate held")
	case <-time.After(30 * time.Millisecond):
	}
	g.Unlock()
	held = false
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("ordinary reload did not resume")
	}
	if !p.running.Load() {
		t.Fatal("committed reload did not start engine")
	}
}

func TestWriterCoordinatedExplicitRuntimeLeaseDoesNotRelock(t *testing.T) {
	o, p, g := writerCoordinatedOrchestrator(t)
	g.Lock()
	defer g.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	lease, err := o.AcquireRuntimeLease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err := lease.ReloadNow(); err != nil {
		t.Fatal(err)
	}
	if !p.running.Load() {
		t.Fatal("transaction-owned activation did not proceed under writer lease")
	}
	select {
	case <-g.entered:
		t.Fatal("owned runtime activation re-acquired writer gate")
	default:
	}
}

func TestWriterCoordinatedCloseCancelsDebouncedAdmission(t *testing.T) {
	o, p, g := writerCoordinatedOrchestrator(t)
	g.Lock()
	held := true
	defer func() {
		if held {
			g.Unlock()
		}
	}()
	var errorLogs atomic.Int32
	o.SetLogger(func(level, message string) {
		if level == "error" {
			errorLogs.Add(1)
		}
	})
	o.mu.Lock()
	o.scheduleReload()
	o.mu.Unlock()
	select {
	case <-g.entered:
	case <-p.started:
		t.Fatal("debounced reload bypassed writer coordinator")
	case <-time.After(2 * time.Second):
		t.Fatal("debounced reload did not reach writer queue")
	}
	closed := make(chan struct{})
	go func() { o.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		g.Unlock()
		held = false
		<-closed
		t.Fatal("Close waited on a timer callback blocked by writer admission")
	}
	if p.running.Load() {
		t.Fatal("closing orchestrator activated queued engine")
	}
	if errorLogs.Load() != 0 {
		t.Fatal("intentional close admission cancellation emitted an engine error")
	}
	if err := o.ReloadContext(context.Background()); err == nil {
		t.Fatal("closed orchestrator accepted reload")
	}
}
