package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
	"github.com/hoaxisr/awg-manager/internal/dnsroute"
	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

func TestAWG3OperationCoordinatorCancelledQueueHasNoSideEffects(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			store := awg3endpoint.NewStore(filepath.Join(t.TempDir(), "awg3.json"))
			svc := &fakeAwg3Service{}
			h := NewAwg3Handler(store, svc, nil, nil)
			gate := &heavyop.Gate{}
			h.SetOperationCoordinator(gate)
			gate.Lock()
			defer gate.Unlock()
			path := awg3BasePath
			if method != http.MethodPost {
				path += "/missing"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			req := httptest.NewRequest(method, path, strings.NewReader(`{"tag":"queued","config":`+validEndpoint+`}`)).WithContext(ctx)
			w := httptest.NewRecorder()
			h.Handle(w, req)
			if w.Code != http.StatusRequestTimeout {
				t.Fatalf("queued mutation escaped coordinator: status=%d body=%s", w.Code, w.Body.String())
			}
			records, err := store.List()
			if err != nil {
				t.Fatal(err)
			}
			if len(records) != 0 || svc.syncCnt != 0 {
				t.Fatal("cancelled mutation changed AWG3 store or slot")
			}
		})
	}
}

type coordinatedAWG3Restore struct {
	*fakeAwg3Service
	gate *heavyop.Gate
	held bool
}

func (s *coordinatedAWG3Restore) RestoreSlot(data []byte, enabled bool) error {
	if s.gate.TryLock() {
		s.gate.Unlock()
	} else {
		s.held = true
	}
	return s.fakeAwg3Service.RestoreSlot(data, enabled)
}

func TestAWG3OperationCoordinatorRetainedThroughDependentRollback(t *testing.T) {
	store := awg3endpoint.NewStore(filepath.Join(t.TempDir(), "awg3.json"))
	gate := &heavyop.Gate{}
	svc := &coordinatedAWG3Restore{fakeAwg3Service: &fakeAwg3Service{}, gate: gate}
	h := NewAwg3Handler(store, svc, nil, nil)
	h.SetOperationCoordinator(gate)
	dnsHeld := false
	h.SetAfterMutation(func([]awg3endpoint.Record) (func() error, error) {
		return func() error {
			if gate.TryLock() {
				gate.Unlock()
			} else {
				dnsHeld = true
			}
			return nil
		}, nil
	})
	h.commitCandidate = func([]awg3endpoint.Record) error { return errors.New("injected commit failure") }
	w := httptest.NewRecorder()
	h.Handle(w, httptest.NewRequest(http.MethodPost, awg3BasePath, strings.NewReader(`{"tag":"new-exit","config":`+validEndpoint+`}`)))
	if w.Code == http.StatusOK {
		t.Fatal("failed endpoint commit reported success")
	}
	if !svc.held || !dnsHeld {
		t.Fatalf("rollback escaped writer coordinator: AWG=%t DNS=%t", svc.held, dnsHeld)
	}
	if !gate.TryLock() {
		t.Fatal("failed import retained writer coordinator")
	}
	gate.Unlock()
	records, err := store.List()
	if err != nil || len(records) != 0 {
		t.Fatalf("failed import persisted candidate: %v %v", records, err)
	}
}

func TestAWG3OperationCoordinatorRealDNSCallbackDoesNotRelock(t *testing.T) {
	dir := t.TempDir()
	o := orchestrator.NewWithAppliedPath(dir, nil, filepath.Join(t.TempDir(), "applied.json"))
	defer o.Close()
	for _, meta := range orchestrator.KnownSlots() {
		if err := o.Register(meta); err != nil {
			t.Fatal(err)
		}
	}
	if err := o.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	store := awg3endpoint.NewStore(filepath.Join(t.TempDir(), "awg3.json"))
	dnsStore := dnsroute.NewStore(t.TempDir())
	if _, err := dnsStore.Load(); err != nil {
		t.Fatal(err)
	}
	dnsSvc := dnsroute.NewSingboxService(dnsStore, dnsroute.NewSingboxReconciler(o, store, "1.1.1.1"), nil)
	gate := &heavyop.Gate{}
	dnsSvc.SetOperationCoordinator(gate)
	h := NewAwg3Handler(store, awg3endpoint.NewService(store, o, nil), nil, nil)
	h.SetOperationCoordinator(gate)
	callbackHeld := false
	h.SetAfterMutation(func(candidate []awg3endpoint.Record) (func() error, error) {
		if gate.TryLock() {
			gate.Unlock()
		} else {
			callbackHeld = true
		}
		return dnsSvc.ReconcileAWG3Candidate(context.Background(), candidate)
	})
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Handle(w, httptest.NewRequest(http.MethodPost, awg3BasePath, strings.NewReader(`{"tag":"new-exit","config":`+validEndpoint+`}`)))
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dependent DNS callback re-entered writer/AWG3 transaction gate")
	}
	if w.Code != http.StatusOK {
		t.Fatalf("real dependent apply failed: %d %s", w.Code, w.Body.String())
	}
	if !callbackHeld {
		t.Fatal("DNS callback ran outside outer transaction")
	}
	if !gate.TryLock() {
		t.Fatal("dependent apply retained writer coordinator")
	}
	gate.Unlock()
}

func TestAWG3OperationCoordinatorHeldThroughDurableCommit(t *testing.T) {
	store := awg3endpoint.NewStore(filepath.Join(t.TempDir(), "awg3.json"))
	svc := &fakeAwg3Service{}
	h := NewAwg3Handler(store, svc, nil, nil)
	gate := &heavyop.Gate{}
	h.SetOperationCoordinator(gate)
	held := false
	h.commitCandidate = func(candidate []awg3endpoint.Record) error {
		if gate.TryLock() {
			gate.Unlock()
		} else {
			held = true
		}
		return store.Replace(candidate)
	}
	w := httptest.NewRecorder()
	h.Handle(w, httptest.NewRequest(http.MethodPost, awg3BasePath, strings.NewReader(`{"tag":"new-exit","config":`+validEndpoint+`}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("import failed: %d %s", w.Code, w.Body.String())
	}
	if !held {
		t.Fatal("writer coordinator released before durable endpoint commit")
	}
	if !gate.TryLock() {
		t.Fatal("completed import retained coordinator")
	}
	gate.Unlock()
}

type coordinatorProbeReader struct {
	*strings.Reader
	gate    *heavyop.Gate
	read    bool
	blocked bool
}

func (r *coordinatorProbeReader) Read(p []byte) (int, error) {
	r.read = true
	if r.gate.TryLock() {
		r.gate.Unlock()
	} else {
		r.blocked = true
	}
	return r.Reader.Read(p)
}

func TestAWG3OperationCoordinatorDoesNotHoldWriterWhileReadingRequest(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPatch} {
		t.Run(method, func(t *testing.T) {
			store := awg3endpoint.NewStore(filepath.Join(t.TempDir(), "awg3.json"))
			h := NewAwg3Handler(store, &fakeAwg3Service{}, nil, nil)
			gate := &heavyop.Gate{}
			h.SetOperationCoordinator(gate)
			body := &coordinatorProbeReader{Reader: strings.NewReader(`{"tag":"new-exit","config":` + validEndpoint + `}`), gate: gate}
			path := awg3BasePath
			if method == http.MethodPatch {
				path += "/missing"
			}
			h.Handle(httptest.NewRecorder(), httptest.NewRequest(method, path, body))
			if !body.read || body.blocked {
				t.Fatal("reading request body holds shared writer coordinator")
			}
		})
	}
}
