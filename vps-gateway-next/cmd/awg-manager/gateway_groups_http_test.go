package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	gatewaygroups "github.com/hoaxisr/awg-manager/internal/gateway/groups"
	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
)

type httpGroupStore struct {
	groups []gatewaygroups.Group
}

func (s *httpGroupStore) Load() ([]gatewaygroups.Group, error) {
	return append([]gatewaygroups.Group(nil), s.groups...), nil
}

func (s *httpGroupStore) Save(groups []gatewaygroups.Group) error {
	s.groups = append([]gatewaygroups.Group(nil), groups...)
	return nil
}

func TestGatewayGroupHTTPCreateUpdateListAndDelete(t *testing.T) {
	allocator, err := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	if err != nil {
		t.Fatal(err)
	}
	registry := gatewayingress.NewRegistry(allocator, func(id string) (string, string, error) {
		return "private-" + id, "public-" + id, nil
	})
	for _, id := range []string{"phone", "tablet"} {
		if _, err := registry.Create(id, id); err != nil {
			t.Fatal(err)
		}
	}
	manager, err := gatewaygroups.NewManager(&httpGroupStore{}, func(id string) bool {
		_, exists := registry.Get(id)
		return exists
	})
	if err != nil {
		t.Fatal(err)
	}
	h := newGatewayClientHTTP(registry, filepath.Join(t.TempDir(), "clients.json"))
	h.SetGatewayGroups(manager)

	create := httptest.NewRecorder()
	h.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/gateway/groups/create", strings.NewReader(`{"name":"Family","clientIds":["phone"]}`)))
	if create.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	var createdEnvelope struct {
		Data gatewaygroups.Group `json:"data"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &createdEnvelope); err != nil {
		t.Fatal(err)
	}
	if createdEnvelope.Data.ID == "" || len(createdEnvelope.Data.ClientIDs) != 1 || createdEnvelope.Data.ClientIDs[0] != "phone" {
		t.Fatalf("unexpected create response: %+v", createdEnvelope.Data)
	}

	list := httptest.NewRecorder()
	h.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/gateway/groups", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), createdEnvelope.Data.ID) {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	updatePath := "/api/gateway/groups/" + createdEnvelope.Data.ID
	update := httptest.NewRecorder()
	h.ServeHTTP(update, httptest.NewRequest(http.MethodPut, updatePath, strings.NewReader(`{"name":"Household","clientIds":["tablet"]}`)))
	if update.Code != http.StatusOK || !strings.Contains(update.Body.String(), "tablet") || !strings.Contains(update.Body.String(), "Household") {
		t.Fatalf("update status=%d body=%s", update.Code, update.Body.String())
	}

	remove := httptest.NewRecorder()
	h.ServeHTTP(remove, httptest.NewRequest(http.MethodDelete, updatePath, nil))
	if remove.Code != http.StatusOK {
		t.Fatalf("delete status=%d body=%s", remove.Code, remove.Body.String())
	}
	list = httptest.NewRecorder()
	h.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/gateway/groups", nil))
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), createdEnvelope.Data.ID) {
		t.Fatalf("deleted group remains in list: status=%d body=%s", list.Code, list.Body.String())
	}
}
