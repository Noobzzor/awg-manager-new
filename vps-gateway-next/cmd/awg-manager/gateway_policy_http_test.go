package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
	gatewaygroups "github.com/hoaxisr/awg-manager/internal/gateway/groups"
	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
	gatewaypolicy "github.com/hoaxisr/awg-manager/internal/gateway/policy"
	singboxorch "github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

type gatewayPolicyTestSaver struct {
	calls   int
	slot    singboxorch.Slot
	data    []byte
	enabled bool
}

func (s *gatewayPolicyTestSaver) SaveAndValidate(slot singboxorch.Slot, data []byte) (singboxorch.ValidationResult, error) {
	s.calls++
	s.slot = slot
	s.data = append([]byte(nil), data...)
	return singboxorch.ValidationResult{}, nil
}

func (s *gatewayPolicyTestSaver) SetEnabled(slot singboxorch.Slot, enabled bool) error {
	if slot != s.slot {
		return fmt.Errorf("enabled slot %q before saving it", slot)
	}
	s.enabled = enabled
	return nil
}

func (s *gatewayPolicyTestSaver) SnapshotSlot(_ singboxorch.Slot) ([]byte, bool, error) {
	return append([]byte(nil), s.data...), s.enabled, nil
}

func (s *gatewayPolicyTestSaver) RestoreSlot(_ singboxorch.Slot, data []byte, enabled bool) error {
	s.data = append([]byte(nil), data...)
	s.enabled = enabled
	return nil
}

type gatewayPolicyTestCatalog struct{}

func (gatewayPolicyTestCatalog) ListTags() []awg3endpoint.TagInfo {
	return []awg3endpoint.TagInfo{{Tag: "direct", Kind: "other"}}
}

func TestGatewayPolicyHTTPProfilesCRUDPersistAndApply(t *testing.T) {
	allocator, err := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	if err != nil {
		t.Fatal(err)
	}
	clients := gatewayingress.NewRegistry(allocator, func(id string) (string, string, error) {
		return "private-" + id, "public-" + id, nil
	})
	client, err := clients.Create("phone", "Phone")
	if err != nil {
		t.Fatal(err)
	}
	groupManager, err := gatewaygroups.NewManager(&httpGroupStore{}, func(id string) bool {
		_, exists := clients.Get(id)
		return exists
	})
	if err != nil {
		t.Fatal(err)
	}
	group, err := groupManager.Create("Family", []string{"phone"})
	if err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(t.TempDir(), "profiles.json")
	store, err := gatewaypolicy.NewFileStore(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	saver := &gatewayPolicyTestSaver{}
	runtime := gatewaypolicy.Runtime{Catalog: gatewayPolicyTestCatalog{}, Saver: saver}
	h := newGatewayClientHTTP(clients, filepath.Join(t.TempDir(), "clients.json"))
	h.SetGatewayGroups(groupManager)
	h.SetGatewayPolicy(store, runtime)

	create := httptest.NewRecorder()
	h.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/gateway/policies/profiles/create", strings.NewReader(`{"id":"family/policy","name":"Family policy","defaultAction":"direct","rules":[{"id":"phone-rule","action":"block","clientIds":["phone"],"groupIds":["`+group.ID+`"],"enabled":true}]}`)))
	if create.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}

	list := httptest.NewRecorder()
	h.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/gateway/policies/profiles", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "family/policy") {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	get := httptest.NewRecorder()
	h.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/gateway/policies/profiles/family%2Fpolicy", nil))
	if get.Code != http.StatusOK || !strings.Contains(get.Body.String(), "family/policy") {
		t.Fatalf("get status=%d body=%s", get.Code, get.Body.String())
	}

	apply := httptest.NewRecorder()
	h.ServeHTTP(apply, httptest.NewRequest(http.MethodPost, "/api/gateway/policies/profiles/family%2Fpolicy/apply", nil))
	if apply.Code != http.StatusOK {
		t.Fatalf("apply status=%d body=%s", apply.Code, apply.Body.String())
	}
	if saver.calls != 1 || saver.slot != singboxorch.SlotGatewayPolicy || !saver.enabled || !strings.Contains(string(saver.data), client.Address.String()+"/32") {
		t.Fatalf("applied slot calls=%d slot=%q enabled=%t data=%s", saver.calls, saver.slot, saver.enabled, saver.data)
	}

	updated := httptest.NewRecorder()
	h.ServeHTTP(updated, httptest.NewRequest(http.MethodPut, "/api/gateway/policies/profiles/family%2Fpolicy", strings.NewReader(`{"id":"family/policy","name":"Updated","defaultAction":"direct"}`)))
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), "Updated") {
		t.Fatalf("update status=%d body=%s", updated.Code, updated.Body.String())
	}
	remove := httptest.NewRecorder()
	h.ServeHTTP(remove, httptest.NewRequest(http.MethodDelete, "/api/gateway/policies/profiles/family%2Fpolicy", nil))
	if remove.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", remove.Code, remove.Body.String())
	}
	reloaded, err := gatewaypolicy.NewFileStore(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := reloaded.Load()
	if err != nil || len(profiles) != 0 {
		t.Fatalf("deleted profile persisted: profiles=%+v err=%v", profiles, err)
	}
}

func TestGatewayPolicyHTTPInvalidApplyDoesNotReplaceLastKnownGood(t *testing.T) {
	store, err := gatewaypolicy.NewFileStore(filepath.Join(t.TempDir(), "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	good := gatewaypolicy.Profile{ID: "good", Name: "Good", DefaultAction: gatewaypolicy.ActionDirect}
	bad := gatewaypolicy.Profile{ID: "bad", Name: "Bad", DefaultAction: gatewaypolicy.ActionVPN}
	if err := store.Save([]gatewaypolicy.Profile{good, bad}); err != nil {
		t.Fatal(err)
	}
	allocator, _ := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	clients := gatewayingress.NewRegistry(allocator, func(id string) (string, string, error) { return "priv", "pub", nil })
	saver := &gatewayPolicyTestSaver{}
	h := newGatewayClientHTTP(clients, filepath.Join(t.TempDir(), "clients.json"))
	h.SetGatewayPolicy(store, gatewaypolicy.Runtime{Catalog: gatewayPolicyTestCatalog{}, Saver: saver})
	goodApply := httptest.NewRecorder()
	h.ServeHTTP(goodApply, httptest.NewRequest(http.MethodPost, "/api/gateway/policies/profiles/good/apply", nil))
	if goodApply.Code != http.StatusOK || saver.calls != 1 {
		t.Fatalf("known-good apply status=%d calls=%d body=%s", goodApply.Code, saver.calls, goodApply.Body.String())
	}
	lastKnownGood := append([]byte(nil), saver.data...)
	apply := httptest.NewRecorder()
	h.ServeHTTP(apply, httptest.NewRequest(http.MethodPost, "/api/gateway/policies/profiles/bad/apply", nil))
	if apply.Code != http.StatusConflict || saver.calls != 1 || string(saver.data) != string(lastKnownGood) {
		t.Fatalf("invalid apply status=%d calls=%d data=%s body=%s", apply.Code, saver.calls, saver.data, apply.Body.String())
	}
	persisted, err := store.Load()
	if err != nil || len(persisted) != 2 || persisted[0].ID != "good" || persisted[1].ID != "bad" {
		t.Fatalf("profiles changed after invalid apply: profiles=%+v err=%v", persisted, err)
	}
}

func TestGatewayPolicyHTTPRejectsUnresolvedSourceAddress(t *testing.T) {
	store, err := gatewaypolicy.NewFileStore(filepath.Join(t.TempDir(), "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save([]gatewaypolicy.Profile{{ID: "missing-client", Name: "Missing", DefaultAction: gatewaypolicy.ActionDirect, Rules: []gatewaypolicy.Rule{{ID: "rule", Action: gatewaypolicy.ActionBlock, ClientIDs: []string{"absent"}, Enabled: true}}}}); err != nil {
		t.Fatal(err)
	}
	allocator, _ := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	clients := gatewayingress.NewRegistry(allocator, func(id string) (string, string, error) { return "priv", "pub", nil })
	saver := &gatewayPolicyTestSaver{}
	h := newGatewayClientHTTP(clients, filepath.Join(t.TempDir(), "clients.json"))
	h.SetGatewayPolicy(store, gatewaypolicy.Runtime{Catalog: gatewayPolicyTestCatalog{}, Saver: saver})
	apply := httptest.NewRecorder()
	h.ServeHTTP(apply, httptest.NewRequest(http.MethodPost, "/api/gateway/policies/profiles/missing-client/apply", nil))
	if apply.Code != http.StatusConflict || saver.calls != 0 || !strings.Contains(apply.Body.String(), "unknown client") {
		t.Fatalf("unresolved source apply status=%d calls=%d body=%s", apply.Code, saver.calls, apply.Body.String())
	}
}

func TestGatewayPolicyHTTPPreviewCompilesWithoutApplying(t *testing.T) {
	allocator, err := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	if err != nil {
		t.Fatal(err)
	}
	clients := gatewayingress.NewRegistry(allocator, func(id string) (string, string, error) {
		return "private-" + id, "public-" + id, nil
	})
	client, err := clients.Create("phone", "Phone")
	if err != nil {
		t.Fatal(err)
	}
	store, err := gatewaypolicy.NewFileStore(filepath.Join(t.TempDir(), "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	saver := &gatewayPolicyTestSaver{}
	h := newGatewayClientHTTP(clients, filepath.Join(t.TempDir(), "clients.json"))
	h.SetGatewayPolicy(store, gatewaypolicy.Runtime{Catalog: gatewayPolicyTestCatalog{}, Saver: saver})
	request := `{"id":"home","name":"Home","defaultAction":"direct","rules":[{"id":"video","action":"direct","outbound":"direct","priority":10,"clientIds":["phone"],"domainSuffixes":["example.com"],"enabled":true}]}`
	preview := httptest.NewRecorder()
	h.ServeHTTP(preview, httptest.NewRequest(http.MethodPost, "/api/gateway/policies/profiles/preview", strings.NewReader(request)))
	if preview.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", preview.Code, preview.Body.String())
	}
	for _, want := range []string{`"final":"direct"`, `"source_ip_cidr":["` + client.Address.String() + `/32"]`, `"domain_suffix":["example.com"]`} {
		if !strings.Contains(preview.Body.String(), want) {
			t.Errorf("preview body missing %s: %s", want, preview.Body.String())
		}
	}
	if saver.calls != 0 {
		t.Fatalf("preview applied policy %d times", saver.calls)
	}
}

func TestGatewayPolicyHTTPPreviewFailsClosedForUnavailableSourceAndOutbound(t *testing.T) {
	allocator, err := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	if err != nil {
		t.Fatal(err)
	}
	clients := gatewayingress.NewRegistry(allocator, func(id string) (string, string, error) { return "private", "public", nil })
	store, err := gatewaypolicy.NewFileStore(filepath.Join(t.TempDir(), "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	saver := &gatewayPolicyTestSaver{}
	h := newGatewayClientHTTP(clients, filepath.Join(t.TempDir(), "clients.json"))
	h.SetGatewayPolicy(store, gatewaypolicy.Runtime{Catalog: gatewayPolicyTestCatalog{}, Saver: saver})

	for _, test := range []struct {
		name   string
		body   string
		status int
		want   string
	}{
		{name: "unresolved client", body: `{"id":"p","name":"P","defaultAction":"direct","rules":[{"id":"r","action":"block","clientIds":["missing"],"enabled":true}]}`, status: http.StatusConflict, want: "unknown client"},
		{name: "unconfigured WARP", body: `{"id":"p","name":"P","defaultAction":"warp"}`, status: http.StatusUnprocessableEntity, want: "no default outbound"},
	} {
		t.Run(test.name, func(t *testing.T) {
			preview := httptest.NewRecorder()
			h.ServeHTTP(preview, httptest.NewRequest(http.MethodPost, "/api/gateway/policies/profiles/preview", strings.NewReader(test.body)))
			if preview.Code != test.status || !strings.Contains(preview.Body.String(), test.want) || saver.calls != 0 {
				t.Fatalf("preview status=%d calls=%d body=%s", preview.Code, saver.calls, preview.Body.String())
			}
		})
	}
}
