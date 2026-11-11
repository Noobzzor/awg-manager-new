package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	groups "github.com/hoaxisr/awg-manager/internal/gateway/groups"
	policy "github.com/hoaxisr/awg-manager/internal/gateway/policy"
	"github.com/hoaxisr/awg-manager/internal/gateway/routerpolicy"
	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
)

func gatewayOwnerFixture(t *testing.T) (*gatewayClientHTTP, *warpHTTPBackend) {
	t.Helper()
	h, _, _ := newGatewaySubscriptionTestHandler(t)
	gm, err := groups.NewManager(&httpGroupStore{}, func(id string) bool { _, ok := h.registry.Get(id); return ok })
	if err != nil {
		t.Fatal(err)
	}
	h.SetGatewayGroups(gm)
	store, err := policy.NewFileStore(filepath.Join(filepath.Dir(h.path), "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	h.SetGatewayPolicy(store, policy.Runtime{Catalog: gatewayPolicyTestCatalog{}, Saver: &gatewayPolicyTestSaver{}})
	warp := &warpHTTPBackend{}
	h.SetGatewayWARP(warp)
	return h, warp
}

func gatewayOwnerSnapshot(t *testing.T, h *gatewayClientHTTP, w *warpHTTPBackend) []byte {
	t.Helper()
	profiles, err := h.policies.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal([]any{h.registry.List(), h.groups.manager.List(), profiles, h.subscriptions.manager.List(), w.calls})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestGatewayOwnerHTTPRejectsBeforeStateChanges(t *testing.T) {
	profile, _ := json.Marshal(warpHTTPProfile())
	for _, tc := range []struct{ name, method, path, body string }{
		{"create", http.MethodPost, "/api/gateway/clients/create", `{"id":"tablet","label":"Tablet"}`},
		{"transition", http.MethodPost, "/api/gateway/clients/phone/disable", ""},
		{"profile", http.MethodPost, "/api/gateway/policies/profiles/create", `{"id":"blocked","name":"Blocked","defaultAction":"direct","rules":[]}`},
		{"group", http.MethodPost, "/api/gateway/groups/create", `{"name":"Blocked","clientIds":["phone"]}`},
		{"subscription", http.MethodPost, "/api/gateway/subscriptions/create", `{"clientId":"phone","expiresInDays":3}`},
		{"warp", http.MethodPost, "/api/gateway/warp/import", string(profile)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, warp := gatewayOwnerFixture(t)
			root := t.TempDir()
			dir := routerpolicy.OriginalStateDir(root)
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "owner.json"), []byte(`{"version":1,"mode":"original"}`), 0600); err != nil {
				t.Fatal(err)
			}
			gate := &heavyop.Gate{}
			checked, held := false, false
			h.SetMutationAdmission(gate, func(ctx context.Context) error {
				checked = true
				if gate.TryLock() {
					gate.Unlock()
				} else {
					held = true
				}
				return routerpolicy.RequireGatewayOwner(ctx, root)
			})
			before := gatewayOwnerSnapshot(t, h, warp)
			out := httptest.NewRecorder()
			h.ServeHTTP(out, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if out.Code != http.StatusConflict || !checked || !held {
				t.Fatal("legacy mutation bypassed coordinated ownership admission")
			}
			if !bytes.Equal(before, gatewayOwnerSnapshot(t, h, warp)) {
				t.Fatal("rejected legacy request changed persistent state")
			}
			if _, err := os.Stat(h.path); !os.IsNotExist(err) {
				t.Fatal("rejected legacy request wrote clients file")
			}
		})
	}
}

func TestGatewayOwnerHTTPPreservesReads(t *testing.T) {
	h, _ := gatewayOwnerFixture(t)
	called := false
	h.SetMutationAdmission(&heavyop.Gate{}, func(context.Context) error { called = true; return routerpolicy.ErrGatewayPolicyOwnership })
	out := httptest.NewRecorder()
	h.ServeHTTP(out, httptest.NewRequest(http.MethodGet, "/api/gateway/clients", nil))
	if out.Code != 200 || called {
		t.Fatal("owner admission blocked read-only gateway listing")
	}
}

func TestGatewayOwnerHTTPAdmitsLegacyUnderGate(t *testing.T) {
	h, _ := gatewayOwnerFixture(t)
	root := t.TempDir()
	gate := &heavyop.Gate{}
	checked, held := false, false
	h.SetMutationAdmission(gate, func(ctx context.Context) error {
		checked = true
		if gate.TryLock() {
			gate.Unlock()
		} else {
			held = true
		}
		return routerpolicy.RequireGatewayOwner(ctx, root)
	})
	out := httptest.NewRecorder()
	h.ServeHTTP(out, httptest.NewRequest(http.MethodPost, "/api/gateway/clients/phone/disable", nil))
	if out.Code != 200 || !checked || !held {
		t.Fatal("legacy write did not retain its normal operation under shared admission")
	}
	if !gate.TryLock() {
		t.Fatal("gateway mutation leaked the writer gate")
	}
	gate.Unlock()
}

func TestGatewayOwnerHTTPQueuedCancellationDoesNotWrite(t *testing.T) {
	h, w := gatewayOwnerFixture(t)
	gate := &heavyop.Gate{}
	gate.Lock()
	defer gate.Unlock()
	checked := false
	h.SetMutationAdmission(gate, func(context.Context) error { checked = true; return nil })
	before := gatewayOwnerSnapshot(t, h, w)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/gateway/clients/phone/disable", nil).WithContext(ctx)
	out := httptest.NewRecorder()
	h.ServeHTTP(out, req)
	if out.Code != http.StatusConflict || checked || !bytes.Equal(before, gatewayOwnerSnapshot(t, h, w)) {
		t.Fatal("cancelled gateway writer changed state or bypassed queue")
	}
}
