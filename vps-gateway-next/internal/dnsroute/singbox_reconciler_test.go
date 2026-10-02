package dnsroute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
	singboxorch "github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

type fakeDNSRouteOrchestrator struct {
	effective     map[singboxorch.Slot][]byte
	calls         [][]byte
	reject        bool
	enabled       bool
	setEnabledErr error
	entered       chan struct{}
	release       chan struct{}
	holds         int
	releases      int
	savedUnheld   bool
}

func (f *fakeDNSRouteOrchestrator) HoldReloads() func() {
	f.holds++
	var once sync.Once
	return func() { once.Do(func() { f.releases++ }) }
}

func (f *fakeDNSRouteOrchestrator) LoadEffective(slot singboxorch.Slot) ([]byte, error) {
	return append([]byte(nil), f.effective[slot]...), nil
}
func (f *fakeDNSRouteOrchestrator) LoadApplied(slot singboxorch.Slot) ([]byte, error) {
	return append([]byte(nil), f.effective[slot]...), nil
}
func (f *fakeDNSRouteOrchestrator) SnapshotSlot(slot singboxorch.Slot) ([]byte, bool, error) {
	if slot != singboxorch.SlotDNSRoutes {
		panic("wrong slot")
	}
	return append([]byte(nil), f.effective[slot]...), f.enabled, nil
}
func (f *fakeDNSRouteOrchestrator) SaveAndValidate(slot singboxorch.Slot, data []byte) (singboxorch.ValidationResult, error) {
	if slot != singboxorch.SlotDNSRoutes {
		panic("wrong slot")
	}
	if f.holds <= f.releases {
		f.savedUnheld = true
	}
	f.calls = append(f.calls, append([]byte(nil), data...))
	if f.entered != nil {
		close(f.entered)
		<-f.release
	}
	if f.reject {
		return singboxorch.ValidationResult{Errors: []singboxorch.ValidationError{{Slot: slot, Kind: "test", Message: "rejected"}}}, nil
	}
	f.effective[slot] = append([]byte(nil), data...)
	return singboxorch.ValidationResult{}, nil
}

func (f *fakeDNSRouteOrchestrator) SetEnabled(slot singboxorch.Slot, enabled bool) error {
	if slot != singboxorch.SlotDNSRoutes {
		panic("wrong slot")
	}
	f.enabled = enabled
	err := f.setEnabledErr
	f.setEnabledErr = nil
	return err
}

func (f *fakeDNSRouteOrchestrator) RestoreSlot(slot singboxorch.Slot, data []byte, enabled bool) error {
	if slot != singboxorch.SlotDNSRoutes {
		panic("wrong slot")
	}
	if data == nil {
		delete(f.effective, slot)
	} else {
		f.effective[slot] = append([]byte(nil), data...)
	}
	f.enabled = enabled
	return nil
}

type fakeAWG3Records struct{ records []awg3endpoint.Record }

type transactionAwareAWG3Records struct {
	mu      sync.Mutex
	records []awg3endpoint.Record
	lock    awg3endpoint.TransactionLocker
	listed  chan struct{}
	once    sync.Once
}

func (f *transactionAwareAWG3Records) List() ([]awg3endpoint.Record, error) {
	f.once.Do(func() { close(f.listed) })
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]awg3endpoint.Record(nil), f.records...), nil
}

func (f *transactionAwareAWG3Records) TransactionLock() awg3endpoint.TransactionLocker {
	return f.lock
}

func (f *transactionAwareAWG3Records) set(records []awg3endpoint.Record) {
	f.mu.Lock()
	f.records = append([]awg3endpoint.Record(nil), records...)
	f.mu.Unlock()
}

type observedDNSRouteTransactionLock struct {
	mu            sync.RWMutex
	readAttempted chan struct{}
	once          sync.Once
}

func (l *observedDNSRouteTransactionLock) Lock()    { l.mu.Lock() }
func (l *observedDNSRouteTransactionLock) Unlock()  { l.mu.Unlock() }
func (l *observedDNSRouteTransactionLock) RUnlock() { l.mu.RUnlock() }
func (l *observedDNSRouteTransactionLock) RLock() {
	l.once.Do(func() { close(l.readAttempted) })
	l.mu.RLock()
}

func (f fakeAWG3Records) List() ([]awg3endpoint.Record, error) {
	return append([]awg3endpoint.Record(nil), f.records...), nil
}

type downloaderFunc func(context.Context, SubscriptionDownloadRequest) ([]byte, SubscriptionDownloadMeta, error)

func (f downloaderFunc) ReadAll(ctx context.Context, req SubscriptionDownloadRequest) ([]byte, SubscriptionDownloadMeta, error) {
	return f(ctx, req)
}

func newLocalDNSRouteService(t *testing.T, orch *fakeDNSRouteOrchestrator) (*SingboxService, *Store) {
	t.Helper()
	store := NewStore(t.TempDir())
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	reconciler := NewSingboxReconciler(orch, fakeAWG3Records{records: []awg3endpoint.Record{{ID: "ep-1", Tag: "awg-primary"}}}, "1.1.1.1")
	return NewSingboxService(store, reconciler, nil), store
}

func routeInput(name, domain string) DomainList {
	return DomainList{Name: name, Backend: BackendSingbox, ManualDomains: []string{domain}, Routes: []RouteTarget{{TunnelID: "ep-1", Fallback: "reject"}}}
}

func TestSingboxServiceMutationHoldsReloadThroughCommit(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
	svc, _ := newLocalDNSRouteService(t, orch)
	if _, err := svc.Create(context.Background(), routeInput("held", "held.example")); err != nil {
		t.Fatal(err)
	}
	if orch.savedUnheld || orch.holds != 1 || orch.releases != 1 {
		t.Fatalf("reload hold lifecycle: unheld=%t holds=%d releases=%d", orch.savedUnheld, orch.holds, orch.releases)
	}
}

func TestSingboxReconcilerResolvesAWG3AndReservesEffectiveTags(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{
		singboxorch.SlotAwg3:   []byte(`{"endpoints":[{"type":"wireguard","tag":"awg-primary"}]}`),
		singboxorch.SlotRouter: []byte(`{"outbounds":[{"type":"direct","tag":"foreign-out"}],"dns":{"servers":[{"type":"udp","tag":"foreign-dns","server":"1.1.1.1"}]}}`),
	}}
	r := NewSingboxReconciler(orch, fakeAWG3Records{records: []awg3endpoint.Record{{ID: "ep-1", Tag: "awg-primary"}}}, "1.1.1.1")
	if err := r.Reconcile([]DomainList{{ID: "x", Backend: BackendSingbox, Enabled: true, Domains: []string{"example.com"}, Routes: []RouteTarget{{TunnelID: "ep-1"}}}}); err != nil {
		t.Fatal(err)
	}
	if len(orch.calls) != 1 || !strings.Contains(string(orch.calls[0]), `"outbound": "awg-primary"`) {
		t.Fatalf("calls=%d fragment=%s", len(orch.calls), orch.calls[0])
	}

	// A generated tag colliding with the current effective config is rejected
	// before SaveAndValidate can replace the active slot.
	orch.effective[singboxorch.SlotRouter] = []byte(`{"outbounds":[{"type":"direct","tag":"dnsroute-x-outbound"}]}`)
	if err := r.Reconcile([]DomainList{{ID: "x", Backend: BackendSingbox, Enabled: true, Domains: []string{"example.com"}, Routes: []RouteTarget{{TunnelID: "ep-1"}, {TunnelID: "direct"}}}}); err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("collision error=%v", err)
	}
	if len(orch.calls) != 1 {
		t.Fatalf("validation called after compile collision: %d", len(orch.calls))
	}
}

func TestSingboxReconcilerDisablesEmptyDNSRoutesSlotAfterValidation(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
	r := NewSingboxReconciler(orch, fakeAWG3Records{}, "1.1.1.1")
	if err := r.Reconcile(nil); err != nil {
		t.Fatal(err)
	}
	if orch.enabled {
		t.Fatal("empty local DNS routes slot must stay disabled")
	}
	if len(orch.calls) != 1 || string(orch.calls[0]) != "{}\n" {
		t.Fatalf("empty candidate was not saved and validated: %q", orch.calls)
	}

	orch.enabled = true
	orch.reject = true
	if err := r.Reconcile(nil); err == nil {
		t.Fatal("expected rejected candidate")
	}
	if !orch.enabled {
		t.Fatal("rejected candidate must not change the DNS routes slot state")
	}
}

func TestSingboxServiceRestoresExactDisabledSlotWhenDurableCommitFails(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
	svc, store := newLocalDNSRouteService(t, orch)
	ctx := context.Background()
	created, err := svc.Create(ctx, routeInput("stable", "stable.example"))
	if err != nil {
		t.Fatal(err)
	}
	orch.enabled = false
	priorSlot := append([]byte(nil), orch.effective[singboxorch.SlotDNSRoutes]...)
	priorCache, _ := json.Marshal(store.GetCached())
	store.path = filepath.Join(t.TempDir(), "directory")
	if err := os.Mkdir(store.path, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err = svc.Update(ctx, DomainList{ID: created.ID, Name: "candidate", ManualDomains: []string{"candidate.example"}, Routes: created.Routes})
	if err == nil || !strings.Contains(err.Error(), "commit DNS routes") {
		t.Fatalf("update error=%v", err)
	}
	if got := orch.effective[singboxorch.SlotDNSRoutes]; string(got) != string(priorSlot) {
		t.Fatalf("slot bytes not restored\nwant=%s\ngot=%s", priorSlot, got)
	}
	if orch.enabled {
		t.Fatal("previously disabled slot was enabled after durable commit failure")
	}
	gotCache, _ := json.Marshal(store.GetCached())
	if string(gotCache) != string(priorCache) {
		t.Fatalf("cache changed after durable commit failure\nwant=%s\ngot=%s", priorCache, gotCache)
	}
}

func TestSingboxServiceRestoresExactSlotWhenSetEnabledFails(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
	svc, store := newLocalDNSRouteService(t, orch)
	ctx := context.Background()
	created, err := svc.Create(ctx, routeInput("stable", "stable.example"))
	if err != nil {
		t.Fatal(err)
	}
	orch.enabled = false
	priorSlot := append([]byte(nil), orch.effective[singboxorch.SlotDNSRoutes]...)
	priorCache, _ := json.Marshal(store.GetCached())
	orch.setEnabledErr = errors.New("injected toggle failure")

	_, err = svc.Update(ctx, DomainList{ID: created.ID, Name: "candidate", ManualDomains: []string{"candidate.example"}, Routes: created.Routes})
	if err == nil || !strings.Contains(err.Error(), "injected toggle failure") {
		t.Fatalf("update error=%v", err)
	}
	if got := orch.effective[singboxorch.SlotDNSRoutes]; string(got) != string(priorSlot) {
		t.Fatalf("slot bytes not restored\nwant=%s\ngot=%s", priorSlot, got)
	}
	if orch.enabled {
		t.Fatal("previously disabled slot was enabled after SetEnabled failure")
	}
	gotCache, _ := json.Marshal(store.GetCached())
	if string(gotCache) != string(priorCache) {
		t.Fatalf("cache changed after SetEnabled failure\nwant=%s\ngot=%s", priorCache, gotCache)
	}
}

func TestSingboxServiceMutationsRebuildFullCandidateAndRollbackRejectedState(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
	svc, store := newLocalDNSRouteService(t, orch)
	ctx := context.Background()

	created, err := svc.Create(ctx, routeInput("one", "one.example"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.Update(ctx, DomainList{ID: created.ID, Name: "one changed", Backend: BackendSingbox, ManualDomains: []string{"one.example", "two.example"}, Routes: created.Routes}); err != nil {
		t.Fatal(err)
	}
	if err = svc.SetEnabled(ctx, created.ID, false); err != nil {
		t.Fatal(err)
	}
	if err = svc.SetEnabled(ctx, created.ID, true); err != nil {
		t.Fatal(err)
	}
	if err = svc.RefreshSubscriptions(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if err = svc.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if len(orch.calls) != 6 {
		t.Fatalf("SaveAndValidate calls=%d, want 6", len(orch.calls))
	}
	if string(orch.calls[len(orch.calls)-1]) != "{}\n" {
		t.Fatalf("delete candidate=%s", orch.calls[len(orch.calls)-1])
	}

	created, err = svc.Create(ctx, routeInput("stable", "stable.example"))
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(store.GetCached())
	activeBefore := append([]byte(nil), orch.effective[singboxorch.SlotDNSRoutes]...)
	orch.reject = true
	_, err = svc.Update(ctx, DomainList{ID: created.ID, Name: "must rollback", Backend: BackendSingbox, ManualDomains: []string{"invalid.example"}, Routes: created.Routes})
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("update error=%v", err)
	}
	after, _ := json.Marshal(store.GetCached())
	if string(after) != string(before) {
		t.Fatalf("persisted state changed after rejection\nbefore=%s\nafter=%s", before, after)
	}
	if string(orch.effective[singboxorch.SlotDNSRoutes]) != string(activeBefore) {
		t.Fatal("active fragment changed after rejection")
	}
}

func TestSingboxServiceDoesNotPersistOrExposeCandidateBeforeValidation(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
	svc, store := newLocalDNSRouteService(t, orch)
	lock := &observedDNSRouteTransactionLock{readAttempted: make(chan struct{})}
	svc.mu = lock
	ctx := context.Background()
	created, err := svc.Create(ctx, routeInput("stable", "stable.example"))
	if err != nil {
		t.Fatal(err)
	}

	orch.entered = make(chan struct{})
	orch.release = make(chan struct{})
	updateDone := make(chan error, 1)
	go func() {
		_, err := svc.Update(ctx, DomainList{ID: created.ID, Name: "candidate", Backend: BackendSingbox, ManualDomains: []string{"candidate.example"}, Routes: created.Routes})
		updateDone <- err
	}()
	<-orch.entered

	raw, err := os.ReadFile(store.path)
	if err != nil {
		close(orch.release)
		t.Fatal(err)
	}
	diskStayedStable := strings.Contains(string(raw), "stable.example") && !strings.Contains(string(raw), "candidate.example")

	readDone := make(chan []DomainList, 1)
	go func() {
		lists, _ := svc.List(ctx)
		readDone <- lists
	}()
	<-lock.readAttempted
	select {
	case <-readDone:
		t.Fatal("List completed while the validation transaction lock was held")
	default:
	}

	close(orch.release)
	if err := <-updateDone; err != nil {
		t.Fatal(err)
	}
	if !diskStayedStable {
		t.Fatalf("candidate reached durable storage before validation: %s", raw)
	}

	select {
	case lists := <-readDone:
		if len(lists) != 1 || lists[0].Name != "candidate" {
			t.Fatalf("post-commit lists=%+v", lists)
		}
	case <-time.After(time.Second):
		t.Fatal("List did not resume after commit")
	}
}

func TestSingboxServiceRefreshAllIsAllOrNothing(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
	svc, store := newLocalDNSRouteService(t, orch)
	data := &StoreData{Lists: []DomainList{
		{ID: "list_1", Name: "first", Backend: BackendSingbox, Enabled: true, ManualDomains: []string{"manual-one.example"}, Domains: []string{"manual-one.example"}, Subscriptions: []Subscription{{URL: "first"}}},
		{ID: "list_2", Name: "second", Backend: BackendSingbox, Enabled: true, ManualDomains: []string{"manual-two.example"}, Domains: []string{"manual-two.example"}, Subscriptions: []Subscription{{URL: "second"}}},
	}, HRRuleIcons: map[string]string{}}
	if err := store.Save(data); err != nil {
		t.Fatal(err)
	}
	svc.SetDownloader(downloaderFunc(func(_ context.Context, req SubscriptionDownloadRequest) ([]byte, SubscriptionDownloadMeta, error) {
		if req.URL == "first" {
			return []byte("fetched.example\n"), SubscriptionDownloadMeta{ContentType: "text/plain"}, nil
		}
		lines := make([]string, 0, MaxSubnetsPerList+1)
		for i := 0; i <= MaxSubnetsPerList; i++ {
			lines = append(lines, fmt.Sprintf("10.%d.%d.0/24", i/256, i%256))
		}
		return []byte(strings.Join(lines, "\n")), SubscriptionDownloadMeta{ContentType: "text/plain"}, nil
	}))

	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RefreshAllSubscriptions(context.Background()); err == nil {
		t.Fatal("expected later refresh to fail subnet validation")
	}
	after, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("refresh-all partially persisted an earlier list\nbefore=%s\nafter=%s", before, after)
	}
	if len(orch.calls) != 0 {
		t.Fatalf("invalid refresh-all candidate reached sing-box: calls=%d", len(orch.calls))
	}
	cached, _ := json.Marshal(store.GetCached())
	var persisted StoreData
	if err := json.Unmarshal(before, &persisted); err != nil {
		t.Fatal(err)
	}
	wantCache, _ := json.Marshal(&persisted)
	if string(cached) != string(wantCache) {
		t.Fatalf("refresh-all changed cached state\nwant=%s\ngot=%s", wantCache, cached)
	}
}

func TestSingboxServiceUpdateForcesSingboxBackend(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
	svc, _ := newLocalDNSRouteService(t, orch)
	created, err := svc.Create(context.Background(), routeInput("stable", "stable.example"))
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.Update(context.Background(), DomainList{
		ID: created.ID, Name: "still local", Backend: "ndms",
		ManualDomains: []string{"local.example"}, Routes: created.Routes,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Backend != BackendSingbox {
		t.Fatalf("backend=%q, want %q", updated.Backend, BackendSingbox)
	}
	lists, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(lists) != 1 || lists[0].Backend != BackendSingbox {
		t.Fatalf("persisted lists=%+v", lists)
	}
}

func TestSingboxServiceBatchMutationsReconcileOnceAndBootReconcile(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
	svc, _ := newLocalDNSRouteService(t, orch)
	ctx := context.Background()
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	created, err := svc.CreateBatch(ctx, []DomainList{routeInput("one", "one.example"), routeInput("two", "two.example")})
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 {
		t.Fatalf("created=%d", len(created))
	}
	if len(orch.calls) != 2 {
		t.Fatalf("calls after boot+batch=%d", len(orch.calls))
	}
	if _, err := svc.DeleteBatch(ctx, []string{created[0].ID, created[1].ID}); err != nil {
		t.Fatal(err)
	}
	if len(orch.calls) != 3 {
		t.Fatalf("calls after delete batch=%d", len(orch.calls))
	}
}

func TestSingboxServiceExternalReconcileWaitsForCommittedAWG3Transaction(t *testing.T) {
	lock := &observedDNSRouteTransactionLock{readAttempted: make(chan struct{})}
	endpoints := &transactionAwareAWG3Records{
		records: []awg3endpoint.Record{{ID: "ep-1", Tag: "old-tag"}},
		lock:    lock, listed: make(chan struct{}),
	}
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{
		singboxorch.SlotAwg3: []byte(`{"endpoints":[{"type":"wireguard","tag":"old-tag"}]}`),
	}}
	store := NewStore(t.TempDir())
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&StoreData{Lists: []DomainList{{
		ID: "list-1", Name: "route", Backend: BackendSingbox, Enabled: true,
		Domains: []string{"example.com"}, Routes: []RouteTarget{{TunnelID: "ep-1"}},
	}}, HRRuleIcons: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	svc := NewSingboxService(store, NewSingboxReconciler(orch, endpoints, "1.1.1.1"), nil)

	lock.Lock()
	done := make(chan error, 1)
	go func() { done <- svc.Reconcile(context.Background()) }()
	select {
	case <-lock.readAttempted:
	case err := <-done:
		lock.Unlock()
		t.Fatalf("external DNS reconcile did not wait for AWG3 transaction: %v", err)
	case <-time.After(time.Second):
		lock.Unlock()
		t.Fatal("external DNS reconcile never attempted the AWG3 transaction read lock")
	}
	select {
	case <-endpoints.listed:
		lock.Unlock()
		t.Fatal("AWG3 records were listed before the transaction committed")
	default:
	}

	endpoints.set([]awg3endpoint.Record{{ID: "ep-1", Tag: "committed-tag"}})
	orch.effective[singboxorch.SlotAwg3] = []byte(`{"endpoints":[{"type":"wireguard","tag":"committed-tag"}]}`)
	lock.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("external DNS reconcile did not resume after AWG3 commit")
	}
	if len(orch.calls) != 1 || !strings.Contains(string(orch.calls[0]), `"outbound": "committed-tag"`) {
		t.Fatalf("DNS reconcile did not use committed AWG3 snapshot: %s", orch.calls)
	}
}

func TestSingboxServiceInAWG3TransactionReconcileDoesNotRelock(t *testing.T) {
	lock := &observedDNSRouteTransactionLock{readAttempted: make(chan struct{})}
	endpoints := &transactionAwareAWG3Records{
		records: []awg3endpoint.Record{{ID: "ep-1", Tag: "candidate-tag"}},
		lock:    lock, listed: make(chan struct{}),
	}
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{
		singboxorch.SlotAwg3: []byte(`{"endpoints":[{"type":"wireguard","tag":"candidate-tag"}]}`),
	}}
	store := NewStore(t.TempDir())
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&StoreData{Lists: []DomainList{{
		ID: "list-1", Name: "route", Backend: BackendSingbox, Enabled: true,
		Domains: []string{"example.com"}, Routes: []RouteTarget{{TunnelID: "ep-1"}},
	}}, HRRuleIcons: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	svc := NewSingboxService(store, NewSingboxReconciler(orch, endpoints, "1.1.1.1"), nil)

	lock.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := svc.ReconcileAWG3Candidate(context.Background(), []awg3endpoint.Record{{ID: "ep-1", Tag: "candidate-tag"}})
		done <- err
	}()
	select {
	case err := <-done:
		lock.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	case <-lock.readAttempted:
		lock.Unlock()
		t.Fatal("in-transaction DNS reconcile tried to reacquire AWG3 read lock")
	case <-time.After(time.Second):
		lock.Unlock()
		t.Fatal("in-transaction DNS reconcile deadlocked")
	}
	if len(orch.calls) != 1 || !strings.Contains(string(orch.calls[0]), `"outbound": "candidate-tag"`) {
		t.Fatalf("in-transaction DNS reconcile missed candidate snapshot: %s", orch.calls)
	}
}

func TestSingboxServiceLockOrderAWG3BeforeDNSPreventsDeadlock(t *testing.T) {
	lock := &observedDNSRouteTransactionLock{readAttempted: make(chan struct{})}
	endpoints := &transactionAwareAWG3Records{
		records: []awg3endpoint.Record{{ID: "ep-1", Tag: "old-tag"}},
		lock:    lock, listed: make(chan struct{}),
	}
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{
		singboxorch.SlotAwg3: []byte(`{"endpoints":[{"type":"awg","tag":"candidate-tag"}]}`),
	}}
	store := NewStore(t.TempDir())
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&StoreData{Lists: nil, HRRuleIcons: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	svc := NewSingboxService(store, NewSingboxReconciler(orch, endpoints, "1.1.1.1"), nil)

	lock.Lock()
	externalDone := make(chan error, 1)
	go func() { externalDone <- svc.Reconcile(context.Background()) }()
	select {
	case <-lock.readAttempted:
	case <-time.After(time.Second):
		lock.Unlock()
		t.Fatal("ordinary DNS reconcile did not attempt AWG3 read lock")
	}

	candidateDone := make(chan error, 1)
	go func() {
		_, err := svc.ReconcileAWG3Candidate(context.Background(), []awg3endpoint.Record{{ID: "ep-1", Tag: "candidate-tag"}})
		candidateDone <- err
	}()
	select {
	case err := <-candidateDone:
		if err != nil {
			lock.Unlock()
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		lock.Unlock()
		t.Fatal("AWG3 candidate reconcile deadlocked behind ordinary DNS mutation")
	}
	lock.Unlock()
	select {
	case err := <-externalDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("ordinary DNS reconcile did not resume")
	}
}

func TestSingboxReconcilerPropagatesInfrastructureErrors(t *testing.T) {
	orch := &fakeDNSRouteOrchestrator{effective: map[singboxorch.Slot][]byte{}}
	r := NewSingboxReconciler(orch, fakeAWG3Records{}, "1.1.1.1")
	// Missing endpoint is a target-resolution error and never reaches validation.
	err := r.Reconcile([]DomainList{{ID: "x", Backend: BackendSingbox, Enabled: true, Domains: []string{"x.example"}, Routes: []RouteTarget{{TunnelID: "missing"}}}})
	if err == nil {
		t.Fatal("expected missing endpoint error")
	}
	if len(orch.calls) != 0 {
		t.Fatal("invalid candidate reached orchestrator")
	}
	_ = errors.New // keep errors import available for future fake infra branches
}
