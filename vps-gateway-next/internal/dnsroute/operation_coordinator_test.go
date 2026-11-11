package dnsroute

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
	singboxorch "github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

func TestDNSOperationCoordinatorCancelledQueueHasNoSideEffects(t *testing.T) {
	for _, reconcile := range []bool{false, true} {
		name := "create"
		if reconcile {
			name = "reconcile"
		}
		t.Run(name, func(t *testing.T) {
			orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
			svc, store := newLocalDNSRouteService(t, orch)
			gate := &heavyop.Gate{}
			svc.SetOperationCoordinator(gate)
			gate.Lock()
			defer gate.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			var err error
			if reconcile {
				err = svc.Reconcile(ctx)
			} else {
				_, err = svc.Create(ctx, routeInput("queued", "queued.example"))
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("queued operation escaped coordinator: %v", err)
			}
			if len(orch.calls) != 0 || len(store.GetCached().Lists) != 0 {
				t.Fatal("cancelled queue changed runtime or persisted DNS lists")
			}
		})
	}
}

type coordinatedDNSRestore struct {
	*fakeDNSRouteOrchestrator
	gate *heavyop.Gate
	held bool
}

func (o *coordinatedDNSRestore) RestoreSlot(slot singboxorch.Slot, data []byte, enabled bool) error {
	if o.gate.TryLock() {
		o.gate.Unlock()
	} else {
		o.held = true
	}
	return o.fakeDNSRouteOrchestrator.RestoreSlot(slot, data, enabled)
}

func TestDNSOperationCoordinatorRetainedThroughRollback(t *testing.T) {
	for _, failure := range []string{"durable-store", "slot-enable"} {
		t.Run(failure, func(t *testing.T) {
			original := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
			svc, store := newLocalDNSRouteService(t, original)
			gate := &heavyop.Gate{}
			svc.SetOperationCoordinator(gate)
			created, err := svc.Create(context.Background(), routeInput("before", "before.example"))
			if err != nil {
				t.Fatal(err)
			}
			prior := string(original.effective[singboxorch.SlotDNSRoutes])
			observed := &coordinatedDNSRestore{fakeDNSRouteOrchestrator: original, gate: gate}
			svc.reconciler.orch = observed
			if failure == "durable-store" {
				store.path = filepath.Join(t.TempDir(), "directory")
				if err := os.Mkdir(store.path, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				original.setEnabledErr = errors.New("injected enable failure")
			}
			created.Name = "candidate"
			created.ManualDomains = []string{"candidate.example"}
			if _, err := svc.Update(context.Background(), *created); err == nil {
				t.Fatal("injected commit failure accepted")
			}
			if !observed.held {
				t.Fatal("rollback ran after releasing writer coordinator")
			}
			if string(original.effective[singboxorch.SlotDNSRoutes]) != prior {
				t.Fatal("rollback did not restore prior DNS slot")
			}
			if !gate.TryLock() {
				t.Fatal("failed operation retained writer coordinator")
			}
			gate.Unlock()
		})
	}
}

func TestDNSOperationCoordinatorHeldThroughSubscriptionDownload(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
	svc, store := newLocalDNSRouteService(t, orch)
	if err := store.Save(&StoreData{Lists: []DomainList{{ID: "list_1", Name: "subscription", Backend: BackendSingbox, Enabled: true, Routes: []RouteTarget{{TunnelID: "ep-1", Fallback: "reject"}}, Subscriptions: []Subscription{{URL: "source"}}}}, HRRuleIcons: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	gate := &heavyop.Gate{}
	svc.SetOperationCoordinator(gate)
	held := false
	svc.SetDownloader(downloaderFunc(func(context.Context, SubscriptionDownloadRequest) ([]byte, SubscriptionDownloadMeta, error) {
		if gate.TryLock() {
			gate.Unlock()
		} else {
			held = true
		}
		return []byte("downloaded.example\n"), SubscriptionDownloadMeta{ContentType: "text/plain"}, nil
	}))
	if err := svc.RefreshAllSubscriptions(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Fatal("shared writer entered while subscription candidate was being prepared")
	}
	if !gate.TryLock() {
		t.Fatal("completed refresh retained coordinator")
	}
	gate.Unlock()
	if len(orch.calls) != 1 {
		t.Fatalf("refresh did not reconcile once: %d", len(orch.calls))
	}
}
