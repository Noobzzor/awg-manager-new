package singbox

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
)

func TestWriterCoordinatedControlCancelsBeforeIntent(t *testing.T) {
	for _, action := range []string{"start", "stop", "restart"} {
		t.Run(action, func(t *testing.T) {
			persisted := 0
			op := newTestOperator(t, func(bool) error { persisted++; return nil })
			op.proc.startCmd = func(string, ...string) (*exec.Cmd, error) { return nil, errors.New("fixture startup unavailable") }
			initial := action != "stop"
			op.manuallyStopped.Store(initial)
			gate := &heavyop.Gate{}
			op.SetOperationCoordinator(gate)
			gate.Lock()
			defer gate.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			if err := op.Control(ctx, action); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("control escaped writer queue:%v", err)
			}
			if persisted != 0 || op.manuallyStopped.Load() != initial {
				t.Fatal("cancelled control changed persisted or in-memory intent")
			}
		})
	}
}

func TestWriterCoordinatedControlHoldsGateThroughIntentPersistence(t *testing.T) {
	for _, reject := range []bool{false, true} {
		name := "commit"
		if reject {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			gate := &heavyop.Gate{}
			held := false
			op := newTestOperator(t, func(bool) error {
				if gate.TryLock() {
					gate.Unlock()
				} else {
					held = true
				}
				if reject {
					return errors.New("injected settings failure")
				}
				return nil
			})
			op.SetOperationCoordinator(gate)
			err := op.Control(context.Background(), "stop")
			if (err != nil) != reject || !held {
				t.Fatalf("intent persistence escaped operation: held=%t err=%v", held, err)
			}
			if op.manuallyStopped.Load() == reject {
				t.Fatal("manual intent did not preserve existing rollback semantics")
			}
			if !gate.TryLock() {
				t.Fatal("control retained writer coordinator")
			}
			gate.Unlock()
		})
	}
}

func TestWriterCoordinatedWatchdogCancelsBeforeRestart(t *testing.T) {
	op := newTestOperator(t, nil)
	op.activeWorkFn = func() bool { return true }
	started := false
	op.startFn = func(context.Context, bool) (bool, error) { started = true; return false, nil }
	gate := &heavyop.Gate{}
	op.SetOperationCoordinator(gate)
	gate.Lock()
	defer gate.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := op.Reconcile(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("watchdog escaped writer queue:%v", err)
	}
	if started {
		t.Fatal("queued watchdog reached process start")
	}
}

func TestWriterCoordinatedWatchdogHoldsGateThroughRestart(t *testing.T) {
	op := newTestOperator(t, nil)
	op.activeWorkFn = func() bool { return true }
	gate := &heavyop.Gate{}
	held := false
	op.startFn = func(context.Context, bool) (bool, error) {
		if gate.TryLock() {
			gate.Unlock()
		} else {
			held = true
		}
		return false, nil
	}
	op.SetOperationCoordinator(gate)
	if err := op.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Fatal("watchdog restart escaped writer coordinator")
	}
	if !gate.TryLock() {
		t.Fatal("watchdog retained writer coordinator")
	}
	gate.Unlock()
}
