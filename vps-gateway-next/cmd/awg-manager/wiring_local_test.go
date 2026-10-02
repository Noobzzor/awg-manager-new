package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
	singboxorch "github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

type fakeLocalProxySlotSaver struct {
	slot  singboxorch.Slot
	data  []byte
	calls int
	err   error
}

func (f *fakeLocalProxySlotSaver) SaveAndValidate(slot singboxorch.Slot, data []byte) (singboxorch.ValidationResult, error) {
	f.calls++
	f.slot = slot
	if f.err != nil {
		return singboxorch.ValidationResult{}, f.err
	}
	f.data = append([]byte(nil), data...)
	return singboxorch.ValidationResult{}, nil
}

func TestConfigureLocalProxyUsesConfiguredReachableAddress(t *testing.T) {
	saver := &fakeLocalProxySlotSaver{}
	if err := configureLocalProxy(saver, "0.0.0.0:1080"); err != nil {
		t.Fatal(err)
	}
	if saver.calls != 1 || saver.slot != singboxorch.SlotLocalProxy {
		t.Fatalf("save calls=%d slot=%q", saver.calls, saver.slot)
	}
	var slot struct {
		Inbounds []map[string]any `json:"inbounds"`
	}
	if err := json.Unmarshal(saver.data, &slot); err != nil {
		t.Fatal(err)
	}
	if len(slot.Inbounds) != 1 || slot.Inbounds[0]["type"] != "mixed" ||
		slot.Inbounds[0]["listen"] != "0.0.0.0" || slot.Inbounds[0]["listen_port"] != float64(1080) {
		t.Fatalf("unexpected local proxy slot: %s", saver.data)
	}
}

func TestConfigureLocalProxyRejectsInvalidAddressWithoutReplacingStaleSlot(t *testing.T) {
	for _, addr := range []string{"", "1080", "0.0.0.0:0", "0.0.0.0:70000"} {
		saver := &fakeLocalProxySlotSaver{data: []byte(`{"stale":true}`)}
		if err := configureLocalProxy(saver, addr); err == nil {
			t.Errorf("configureLocalProxy(%q) succeeded, want error", addr)
		}
		if saver.calls != 0 || string(saver.data) != `{"stale":true}` {
			t.Errorf("invalid address %q touched stale slot: calls=%d data=%s", addr, saver.calls, saver.data)
		}
	}
}

func TestConfigureLocalProxyPropagatesSlotSaveFailure(t *testing.T) {
	saveErr := errors.New("injected local proxy slot write failure")
	saver := &fakeLocalProxySlotSaver{err: saveErr}
	if err := configureLocalProxy(saver, "0.0.0.0:1080"); !errors.Is(err, saveErr) {
		t.Fatalf("error=%v, want %v", err, saveErr)
	}
}

func TestRunComposition_SingboxUsesOnlyPortablePhasesInOrder(t *testing.T) {
	var called []string
	record := func(name string) func() { return func() { called = append(called, name) } }
	recordErr := func(name string) func() error {
		return func() error { called = append(called, name); return nil }
	}
	forbidden := func(name string) func() {
		return func() { t.Fatalf("forbidden Keenetic phase called in singbox mode: %s", name) }
	}

	h := compositionHooks{
		core: record("core"),
		ndms: forbidden("ndms"), tunnels: forbidden("tunnels"), services: forbidden("services"),
		orchestrator: forbidden("orchestrator"), eventWiring: forbidden("event-wiring"),
		singbox: func() error { forbidden("legacy-singbox")(); return nil }, server: forbidden("legacy-server"),
		deviceProxy: forbidden("device-proxy"), router: forbidden("router"),
		listen: forbidden("keen-dns/listen"), shutdown: forbidden("legacy-shutdown"),
		proxyShutdown: forbidden("proxy-shutdown"), boot: forbidden("router-boot"),
		local: recordErr("local"), localServer: record("local-server"),
		localLifecycle: record("local-lifecycle"), serve: record("serve"),
	}

	if err := runComposition(deploymentModeSingbox, h); err != nil {
		t.Fatal(err)
	}
	want := []string{"core", "local", "local-server", "local-lifecycle", "serve"}
	if !reflect.DeepEqual(called, want) {
		t.Fatalf("local composition order = %v, want %v", called, want)
	}
}

func TestRunComposition_KeeneticPreservesLegacyPhaseOrder(t *testing.T) {
	var called []string
	record := func(name string) func() { return func() { called = append(called, name) } }
	h := compositionHooks{
		core: record("core"), ndms: record("ndms"), tunnels: record("tunnels"),
		services: record("services"), orchestrator: record("orchestrator"),
		eventWiring: record("event-wiring"), singbox: func() error { record("singbox")(); return nil },
		server: record("server"), deviceProxy: record("device-proxy"), router: record("router"),
		listen: record("listen"), shutdown: record("shutdown"),
		proxyShutdown: record("proxy-shutdown"), boot: record("boot"), serve: record("serve"),
		local:          func() error { t.Fatal("local phase called"); return nil },
		localServer:    func() { t.Fatal("local server called") },
		localLifecycle: func() { t.Fatal("local lifecycle called") },
	}

	if err := runComposition(deploymentModeKeenetic, h); err != nil {
		t.Fatal(err)
	}
	want := []string{"core", "ndms", "tunnels", "services", "orchestrator", "event-wiring", "singbox", "server", "device-proxy", "router", "listen", "shutdown", "proxy-shutdown", "boot", "serve"}
	if !reflect.DeepEqual(called, want) {
		t.Fatalf("keenetic composition order = %v, want %v", called, want)
	}
}

func TestRunComposition_LocalStartupFailureStopsBeforeServerAndServe(t *testing.T) {
	wantErr := errors.New("required local proxy unavailable")
	var called []string
	record := func(name string) func() { return func() { called = append(called, name) } }
	err := runComposition(deploymentModeSingbox, compositionHooks{
		core: record("core"),
		local: func() error {
			called = append(called, "local")
			return wantErr
		},
		localServer: record("local-server"), localLifecycle: record("local-lifecycle"), serve: record("serve"),
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error=%v, want %v", err, wantErr)
	}
	if want := []string{"core", "local"}; !reflect.DeepEqual(called, want) {
		t.Fatalf("calls=%v, want %v", called, want)
	}
}

func TestCleanupLegacyPID_IsKeeneticOnly(t *testing.T) {
	called := 0
	remove := func(string) error { called++; return nil }
	cleanupLegacyPID(deploymentModeSingbox, remove)
	if called != 0 {
		t.Fatalf("local common core touched legacy /opt PID path %d times", called)
	}
	cleanupLegacyPID(deploymentModeKeenetic, remove)
	if called != 1 {
		t.Fatalf("keenetic cleanup calls = %d, want 1", called)
	}
}

func TestRunPlatformSetup_SingboxSkipsKeeneticAndEntwarePhases(t *testing.T) {
	called := 0
	forbidden := func() { called++ }
	runPlatformSetup(deploymentModeSingbox, forbidden, forbidden)
	if called != 0 {
		t.Fatalf("local platform setup called %d Keenetic/Entware phases", called)
	}
}

func TestRunPlatformSetup_KeeneticPreservesClockThenCAOrder(t *testing.T) {
	var called []string
	runPlatformSetup(deploymentModeKeenetic,
		func() { called = append(called, "router-clock") },
		func() { called = append(called, "ca-certs") },
	)
	want := []string{"router-clock", "ca-certs"}
	if !reflect.DeepEqual(called, want) {
		t.Fatalf("keenetic platform setup order = %v, want %v", called, want)
	}
}

func TestRunWithCleanup_BindFailureCleansBeforeReturningError(t *testing.T) {
	bindErr := errors.New("bind failed")
	var order []string
	err := runWithCleanup(func() error {
		order = append(order, "bind")
		return bindErr
	}, func() {
		order = append(order, "cleanup")
	})
	if !errors.Is(err, bindErr) {
		t.Fatalf("error = %v, want bind failure", err)
	}
	if want := []string{"bind", "cleanup"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
}

func TestJoinedWorker_StopsAndCompletesBeforeNextCleanup(t *testing.T) {
	for i := 0; i < 64; i++ {
		var started, completed atomic.Bool
		stop := startJoinedWorker(func(ctx context.Context) {
			started.Store(true)
			<-ctx.Done()
			completed.Store(true)
		})
		deadline := time.Now().Add(time.Second)
		for !started.Load() && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		if !started.Load() {
			t.Fatal("worker did not start")
		}

		a := &app{}
		a.deferOnExit(func() {
			if !completed.Load() {
				t.Fatal("orchestrator cleanup ran before watchdog completion")
			}
		})
		a.deferOnExit(stop)
		a.runOnExit()
	}
}

type observedLocalTunnelTransactionLock struct {
	mu            sync.RWMutex
	readAttempted chan struct{}
	once          sync.Once
}

func (l *observedLocalTunnelTransactionLock) Lock()    { l.mu.Lock() }
func (l *observedLocalTunnelTransactionLock) Unlock()  { l.mu.Unlock() }
func (l *observedLocalTunnelTransactionLock) RUnlock() { l.mu.RUnlock() }
func (l *observedLocalTunnelTransactionLock) RLock() {
	l.once.Do(func() { close(l.readAttempted) })
	l.mu.RLock()
}

func TestLocalTunnelsReaderWaitsForAwg3MutationTransaction(t *testing.T) {
	store := awg3endpoint.NewStore(filepath.Join(t.TempDir(), "awg3.json"))
	lock := &observedLocalTunnelTransactionLock{readAttempted: make(chan struct{})}
	handler := newLocalTunnelsHandler(store, lock)

	// Model the real handler transaction at its dangerous point: Store.Add has
	// published the candidate, while AWG slot/DNS reconciliation can still fail.
	lock.Lock()
	if err := store.Add(awg3endpoint.Record{ID: "candidate", Tag: "candidate", Endpoint: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	readDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		rec := httptest.NewRecorder()
		handler(rec, httptest.NewRequest(http.MethodGet, "/api/tunnels", nil))
		readDone <- rec
	}()
	<-lock.readAttempted
	select {
	case rec := <-readDone:
		t.Fatalf("local tunnel list completed inside AWG3 transaction: code=%d body=%s", rec.Code, rec.Body.String())
	default:
	}
	if err := store.Delete("candidate"); err != nil {
		t.Fatal(err)
	}
	lock.Unlock()

	select {
	case rec := <-readDone:
		if rec.Code != http.StatusOK {
			t.Fatalf("local tunnel list code=%d body=%s", rec.Code, rec.Body.String())
		}
		var env struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatal(err)
		}
		if len(env.Data) != 0 {
			t.Fatalf("local tunnel list exposed rolled-back candidate: %+v", env.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("local tunnel list did not resume after transaction rollback")
	}
}
