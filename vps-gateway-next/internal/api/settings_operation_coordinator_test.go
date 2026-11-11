package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

// The optional startup contract lets the RED tests exercise existing behavior
// before the new setter exists, rather than failing to compile.
func configureSettingsWriter(h *SettingsHandler, gate *heavyop.Gate) {
	if configurable, ok := any(h).(interface{ SetOperationCoordinator(*heavyop.Gate) }); ok {
		configurable.SetOperationCoordinator(gate)
	}
}

func TestSettingsOperationCoordinatorCancelledQueuePreservesStore(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewSettingsStore(dir)
	if _, err := store.Load(); err != nil {
		t.Fatal(err)
	}
	h := NewSettingsHandler(store, nil)
	path := filepath.Join(dir, "settings.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	h.SetApplyBootstrapDNS(func(string) error { called = true; return nil })
	gate := &heavyop.Gate{}
	configureSettingsWriter(h, gate)
	gate.Lock()
	defer gate.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	r := httptest.NewRequest(http.MethodPost, "/api/settings/update", strings.NewReader(`{"singboxBootstrapDNS":"8.8.8.8"}`)).WithContext(ctx)
	w := httptest.NewRecorder()
	h.Update(w, r)
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusRequestTimeout || !strings.Contains(w.Body.String(), "REQUEST_CANCELLED") || called || !bytes.Equal(before, after) {
		t.Fatalf("queued settings update escaped admission: status=%d callback=%t storeChanged=%t", w.Code, called, !bytes.Equal(before, after))
	}
}

func TestSettingsOperationCoordinatorSpansApplyAndPreservesErrorSemantics(t *testing.T) {
	for _, reject := range []bool{false, true} {
		name := "success"
		if reject {
			name = "apply-error"
		}
		t.Run(name, func(t *testing.T) {
			h, store := newSettingsHandlerForTest(t)
			gate := &heavyop.Gate{}
			configureSettingsWriter(h, gate)
			held, storedBeforeApply := false, false
			h.SetApplyBootstrapDNS(func(value string) error {
				if gate.TryLock() {
					gate.Unlock()
				} else {
					held = true
				}
				s, err := store.Get()
				storedBeforeApply = err == nil && s.SingboxBootstrapDNS == value
				if reject {
					return errors.New("injected apply failure")
				}
				return nil
			})
			w := postSettingsUpdate(t, h, `{"singboxBootstrapDNS":"8.8.8.8"}`)
			want := http.StatusOK
			if reject {
				want = http.StatusBadRequest
			}
			s, err := store.Get()
			if w.Code != want || !held || !storedBeforeApply || err != nil || s.SingboxBootstrapDNS != "8.8.8.8" {
				t.Fatalf("settings apply escaped writer or changed persistence semantics: status=%d held=%t storedBeforeApply=%t", w.Code, held, storedBeforeApply)
			}
			if !gate.TryLock() {
				t.Fatal("completed settings request retained writer gate")
			}
			gate.Unlock()
		})
	}
}

type settingsBodyProbe struct {
	*strings.Reader
	gate              *heavyop.Gate
	observed, blocked bool
}

func (b *settingsBodyProbe) Read(p []byte) (int, error) {
	b.observed = true
	if b.gate.TryLock() {
		b.gate.Unlock()
	} else {
		b.blocked = true
	}
	return b.Reader.Read(p)
}
func (*settingsBodyProbe) Close() error { return nil }

func TestSettingsOperationCoordinatorReadsBodyBeforeAdmission(t *testing.T) {
	h, _ := newSettingsHandlerForTest(t)
	gate := &heavyop.Gate{}
	configureSettingsWriter(h, gate)
	body := &settingsBodyProbe{Reader: strings.NewReader(`{"singboxBootstrapDNS":"8.8.8.8"}`), gate: gate}
	r := httptest.NewRequest(http.MethodPost, "/api/settings/update", nil)
	r.Body = body
	w := httptest.NewRecorder()
	h.Update(w, r)
	if w.Code != http.StatusOK || !body.observed || body.blocked {
		t.Fatal("request body was not consumed before writer admission")
	}
}
