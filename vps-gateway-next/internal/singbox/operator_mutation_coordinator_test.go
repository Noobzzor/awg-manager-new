package singbox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
)

func TestOperatorMutationCoordinatorCancelledQueue(t *testing.T) {
	for _, action := range []string{"add", "remove", "update", "rename", "apply", "select", "uninstall"} {
		t.Run(action, func(t *testing.T) {
			op := newTestOperator(t, nil)
			op.proc.logDir = t.TempDir()
			marker := filepath.Join(op.dir, "preserve")
			if err := os.WriteFile(marker, []byte("owned fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			gate := &heavyop.Gate{}
			op.SetOperationCoordinator(gate)
			gate.Lock()
			defer gate.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			var err error
			switch action {
			case "add":
				_, _, err = op.AddTunnels(ctx, "invalid fixture input")
			case "remove":
				err = op.RemoveTunnel(ctx, "missing")
			case "update":
				err = op.UpdateTunnel(ctx, "missing", json.RawMessage(`{}`))
			case "rename":
				err = op.RenameTunnel(ctx, "missing", "new")
			case "apply":
				err = op.ApplyConfig(ctx, NewConfig())
			case "select":
				err = op.SetSelectorDefault(ctx, "selector", "member")
			case "uninstall":
				err = op.Uninstall(ctx)
			}
			_, markerErr := os.Stat(marker)
			if !errors.Is(err, context.DeadlineExceeded) || markerErr != nil || op.installBusy.Load() {
				t.Fatalf("mutation escaped writer admission: err=%v markerErr=%v busy=%t", err, markerErr, op.installBusy.Load())
			}
		})
	}
}

func TestOperatorMutationCoordinatorNestedApplyDoesNotRelock(t *testing.T) {
	for _, action := range []string{"add", "update", "rename"} {
		t.Run(action, func(t *testing.T) {
			op, _ := newOrchedOperator(t)
			op.proxyMgr = &fakeProxies{}
			op.ndmsProxyEnabledFn = func() bool { return false }
			releaseReloads := op.orch.HoldReloads()
			defer releaseReloads()
			cfg := NewConfig()
			if err := cfg.AddTunnelWithListenPort("A", "vless", "old.example", 443, 0, json.RawMessage(`{"type":"vless","tag":"A","server":"old.example","server_port":443}`)); err != nil {
				t.Fatal(err)
			}
			if err := op.ApplyConfig(context.Background(), cfg); err != nil {
				t.Fatal(err)
			}
			gate := &heavyop.Gate{}
			op.SetOperationCoordinator(gate)
			op.orch.SetOperationCoordinator(gate)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var err error
			switch action {
			case "add":
				var added []TunnelInfo
				var parseErrors []BatchError
				added, parseErrors, err = op.AddTunnels(ctx, "vless://3a3b1c2e-9999-4321-aaaa-1234567890ab@new.example:443?security=tls#B")
				if err == nil && (len(added) != 1 || len(parseErrors) != 0) {
					t.Fatal("valid owned fixture was not added")
				}
			case "update":
				err = op.UpdateTunnel(ctx, "A", json.RawMessage(`{"type":"vless","tag":"A","server":"new.example","server_port":443}`))
			case "rename":
				err = op.RenameTunnel(ctx, "A", "B")
			}
			if err != nil {
				t.Fatalf("nested mutation re-entered writer gate or failed:%v", err)
			}
			if !gate.TryLock() {
				t.Fatal("completed tunnel mutation retained writer gate")
			}
			gate.Unlock()
			got, err := op.LoadCurrentConfig()
			if err != nil {
				t.Fatal(err)
			}
			want := 1
			if action == "add" {
				want = 2
			}
			if len(got.Tunnels()) != want {
				t.Fatal("mutation did not persist expected tunnels")
			}
		})
	}
}

type uninstallObservedGate struct {
	heavyop.Gate
	admitted chan struct{}
}

func (g *uninstallObservedGate) LockContext(ctx context.Context) error {
	if err := g.Gate.LockContext(ctx); err != nil {
		return err
	}
	close(g.admitted)
	return nil
}

func TestOperatorMutationCoordinatorUninstallSpansStopAndOwnedRemoval(t *testing.T) {
	op := newTestOperator(t, nil)
	op.proc.logDir = t.TempDir()
	protected := filepath.Join(op.proc.logDir, "unrelated")
	if err := os.WriteFile(protected, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	gate := &uninstallObservedGate{admitted: make(chan struct{})}
	op.SetOperationCoordinator(gate)
	lease, err := op.proc.AcquireProcessLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- op.Uninstall(context.Background()) }()
	joined := false
	defer func() {
		lease.Close()
		if !joined {
			<-done
		}
	}()
	select {
	case <-gate.admitted:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("uninstall reached process wait without writer admission")
	}
	if gate.TryLock() {
		gate.Unlock()
		t.Fatal("uninstall released writer before process stop finished")
	}
	if _, err := os.Stat(op.dir); err != nil {
		t.Fatal("uninstall removed files before process stop")
	}
	lease.Close()
	stopErr := <-done
	joined = true
	if stopErr != nil {
		t.Fatal(stopErr)
	}
	if _, err := os.Stat(op.dir); !os.IsNotExist(err) {
		t.Fatal("owned runtime directory was not removed")
	}
	if _, err := os.Stat(protected); err != nil {
		t.Fatal("uninstall touched unrelated log-directory file")
	}
	if !gate.TryLock() {
		t.Fatal("uninstall retained writer after completion")
	}
	gate.Unlock()
}
