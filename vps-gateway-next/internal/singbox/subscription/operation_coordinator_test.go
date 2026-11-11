package subscription

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
)

const coordinatorInline = "vless://3a3b1c2e-9999-4321-aaaa-1234567890a1@a.example:443?security=tls&sni=a#Member"

func TestSubscriptionOperationCoordinatorCancelledQueuePreservesState(t *testing.T) {
	names := []string{"create", "refresh", "delete", "update", "reconcile", "sync-proxies", "active", "add-member", "remove-member", "exclude", "restore", "move-info", "remove-info", "orphans", "create-group", "update-group", "delete-group"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			svc, mut, _ := newTestServiceWithGroups(t)
			sub, err := svc.Create(context.Background(), CreateInput{Label: "seed", Inline: coordinatorInline, Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			group, err := svc.CreateGroup(context.Background(), GroupCreateInput{Label: "seed-group", UseSubscriptionIDs: []string{sub.ID}, Enabled: true})
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(svc.List())
			beforeGroups, _ := json.Marshal(svc.ListGroups())
			reloads, rollbacks, adds, proxies := mut.reloads, mut.rollbacks, len(mut.addedOutbounds), len(mut.ensuredProxies)
			gate := &heavyop.Gate{}
			svc.SetOperationCoordinator(gate)
			gate.Lock()
			defer gate.Unlock()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			member := sub.MemberTags[0]
			label := "updated"
			switch name {
			case "create":
				_, err = svc.Create(ctx, CreateInput{Label: "another", Inline: coordinatorInline, Enabled: true})
			case "refresh":
				_, err = svc.Refresh(ctx, sub.ID)
			case "delete":
				err = svc.Delete(ctx, sub.ID)
			case "update":
				_, err = svc.UpdateContext(ctx, sub.ID, UpdatePatch{Label: &label})
			case "reconcile":
				err = svc.Reconcile(ctx)
			case "sync-proxies":
				err = svc.SyncProxies(ctx)
			case "active":
				err = svc.SetActiveMember(ctx, sub.ID, member)
			case "add-member":
				_, err = svc.AddManualMember(ctx, sub.ID, coordinatorInline)
			case "remove-member":
				_, err = svc.RemoveMember(ctx, sub.ID, member)
			case "exclude":
				_, err = svc.ExcludeMembers(ctx, sub.ID, []string{member})
			case "restore":
				_, err = svc.RestoreMembers(ctx, sub.ID, []string{member})
			case "move-info":
				_, err = svc.MoveRejectedToInfo(ctx, sub.ID, member)
			case "remove-info":
				_, err = svc.RemoveInfoItem(ctx, sub.ID, "missing")
			case "orphans":
				err = svc.DeleteOrphans(ctx, sub.ID)
			case "create-group":
				_, err = svc.CreateGroup(ctx, GroupCreateInput{Label: "another-group", UseSubscriptionIDs: []string{sub.ID}})
			case "update-group":
				_, err = svc.UpdateGroup(ctx, group.ID, GroupUpdatePatch{})
			case "delete-group":
				err = svc.DeleteGroup(ctx, group.ID)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("operation escaped shared writer queue: %v", err)
			}
			after, _ := json.Marshal(svc.List())
			afterGroups, _ := json.Marshal(svc.ListGroups())
			if string(before) != string(after) || string(beforeGroups) != string(afterGroups) {
				t.Fatal("cancelled queue changed subscription/group state")
			}
			if mut.reloads != reloads || mut.rollbacks != rollbacks || len(mut.addedOutbounds) != adds || len(mut.ensuredProxies) != proxies {
				t.Fatal("cancelled queue touched adapter batch or proxy")
			}
		})
	}
}

type coordinatedSubscriptionMutator struct {
	*fakeMutator
	gate         *heavyop.Gate
	reloadHeld   bool
	rollbackHeld bool
}

func (m *coordinatedSubscriptionMutator) Reload(ctx context.Context) error {
	if m.gate.TryLock() {
		m.gate.Unlock()
	} else {
		m.reloadHeld = true
	}
	return m.fakeMutator.Reload(ctx)
}
func (m *coordinatedSubscriptionMutator) Rollback() {
	if m.gate.TryLock() {
		m.gate.Unlock()
	} else {
		m.rollbackHeld = true
	}
	m.fakeMutator.Rollback()
}

func TestSubscriptionOperationCoordinatorSpansFetchCommitAndRollback(t *testing.T) {
	for _, reject := range []bool{false, true} {
		name := "commit"
		if reject {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			svc, base := newTestService(t)
			gate := &heavyop.Gate{}
			svc.SetOperationCoordinator(gate)
			mut := &coordinatedSubscriptionMutator{fakeMutator: base, gate: gate}
			svc.mutator = mut
			if reject {
				base.reloadErr = errors.New("injected reload rejection")
			}
			var fetchHeld atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if gate.TryLock() {
					gate.Unlock()
				} else {
					fetchHeld.Store(true)
				}
				_, _ = w.Write([]byte(coordinatorInline))
			}))
			defer server.Close()
			_, err := svc.Create(context.Background(), CreateInput{Label: "remote", URL: server.URL, Enabled: true})
			if (err != nil) != reject {
				t.Fatalf("unexpected create result: %v", err)
			}
			if !fetchHeld.Load() || !mut.reloadHeld || reject && !mut.rollbackHeld {
				t.Fatalf("operation escaped gate: fetch=%t reload=%t rollback=%t", fetchHeld.Load(), mut.reloadHeld, mut.rollbackHeld)
			}
			if !gate.TryLock() {
				t.Fatal("finished operation retained coordinator")
			}
			gate.Unlock()
		})
	}
}

func TestSubscriptionOperationCoordinatorScheduledRefreshRespectsCancellation(t *testing.T) {
	svc, mut := newTestService(t)
	sub := createURLSubWithMembers(t, svc, 1)
	hours := 1
	if _, err := svc.Update(sub.ID, UpdatePatch{RefreshHours: &hours}); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(svc.List())
	reloads := mut.reloads
	gate := &heavyop.Gate{}
	svc.SetOperationCoordinator(gate)
	gate.Lock()
	defer gate.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	scheduler := NewScheduler(svc.store, func(ctx context.Context, id string) error { _, err := svc.Refresh(ctx, id); result <- err; return err })
	scheduler.tick(ctx, sub.LastFetched.Add(48*time.Hour))
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("scheduled refresh escaped queue: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("scheduled refresh ignored cancellation")
	}
	scheduler.workerWG.Wait()
	after, _ := json.Marshal(svc.List())
	if string(before) != string(after) || reloads != mut.reloads {
		t.Fatal("cancelled scheduler mutated subscription")
	}
}

func TestSubscriptionOperationCoordinatorPrivateRebuildsDoNotRelock(t *testing.T) {
	svc, _, _ := newTestServiceWithGroups(t)
	sub, err := svc.Create(context.Background(), CreateInput{Label: "seed", Inline: coordinatorInline, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	gate := &heavyop.Gate{}
	svc.SetOperationCoordinator(gate)
	mode := ModeURLTest
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := svc.UpdateContext(ctx, sub.ID, UpdatePatch{Mode: &mode}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	label := "legacy-compatible"
	if _, err := svc.Update(sub.ID, UpdatePatch{Label: &label}); err != nil {
		t.Fatal(err)
	}
	if !gate.TryLock() {
		t.Fatal("nested rebuild retained coordinator")
	}
	gate.Unlock()
}
