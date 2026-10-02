package ndmsinfo

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/ndms"
	"github.com/hoaxisr/awg-manager/internal/ndms/query"
)

// fakeGetter implements query.Getter. Get returns either a configured slice
// of routes or a configured error, switchable under a mutex so a goroutine
// can flip the mode mid-test.
type fakeGetter struct {
	mu     sync.Mutex
	routes []ndms.Route
	err    error
}

func (f *fakeGetter) setRoutes(routes []ndms.Route) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes = routes
	f.err = nil
}

func (f *fakeGetter) setError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes = nil
	f.err = err
}

func (f *fakeGetter) Get(_ context.Context, _ string, dst any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	out, ok := dst.(*[]ndms.Route)
	if !ok {
		return errors.New("fakeGetter: unexpected dst type")
	}
	*out = f.routes
	return nil
}

func (f *fakeGetter) GetRaw(_ context.Context, _ string) ([]byte, error) { return nil, nil }

func (f *fakeGetter) Post(_ context.Context, _ any) (json.RawMessage, error) { return nil, nil }

func newRouteStore(g query.Getter) *query.RouteStore {
	return query.NewRouteStore(g, query.NopLogger())
}

func TestWaitForNDMS_ImmediateReady(t *testing.T) {
	fg := &fakeGetter{}
	fg.setRoutes([]ndms.Route{{Destination: "0.0.0.0/0", Gateway: "1.2.3.4", Interface: "PPPoE0"}})
	routes := newRouteStore(fg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	if err := WaitForNDMS(ctx, routes, nil); err != nil {
		t.Fatalf("WaitForNDMS: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Errorf("expected pre-loop success, took %s", elapsed)
	}
}

func TestWaitForNDMS_ErrNoDefaultRouteCountsAsReady(t *testing.T) {
	fg := &fakeGetter{}
	// NDMS answered, but no default route exists yet — must count as alive.
	fg.setRoutes([]ndms.Route{{Destination: "10.0.0.0/24", Gateway: "", Interface: "Wireguard0"}})
	routes := newRouteStore(fg)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	if err := WaitForNDMS(ctx, routes, nil); err != nil {
		t.Fatalf("WaitForNDMS: %v", err)
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Errorf("expected pre-loop success, took %s", elapsed)
	}
}

func TestWaitForNDMS_ContextCancelled(t *testing.T) {
	fg := &fakeGetter{}
	// Never ready: transport error on every probe.
	fg.setError(errors.New("rci: connection refused"))
	routes := newRouteStore(fg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the call

	start := time.Now()
	err := WaitForNDMS(ctx, routes, nil)
	if err == nil {
		t.Fatal("WaitForNDMS: want non-nil error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("want context.Canceled, got %v", err)
	}
	if elapsed := time.Since(start); elapsed >= 2*time.Second {
		t.Errorf("expected prompt return, took %s", elapsed)
	}
}

func TestWaitForNDMS_BecomesReadyDuringLoop(t *testing.T) {
	fg := &fakeGetter{}
	fg.setError(errors.New("rci: connection refused"))
	routes := newRouteStore(fg)

	go func() {
		time.Sleep(1200 * time.Millisecond)
		fg.setRoutes([]ndms.Route{{Destination: "0.0.0.0/0", Gateway: "1.2.3.4", Interface: "PPPoE0"}})
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := WaitForNDMS(ctx, routes, nil); err != nil {
		t.Fatalf("WaitForNDMS: %v", err)
	}
}
