package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
)

func TestGatewayClientHTTPCreateListAndDisable(t *testing.T) {
	allocator, err := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	if err != nil {
		t.Fatal(err)
	}
	registry := gatewayingress.NewRegistry(allocator, func(id string) (string, string, error) {
		return "private-" + id, "public-" + id, nil
	})
	path := filepath.Join(t.TempDir(), "clients.json")
	h := newGatewayClientHTTP(registry, path)

	create := httptest.NewRecorder()
	createReq := httptest.NewRequest(http.MethodPost, "/api/gateway/clients/create", strings.NewReader(`{"id":"phone","label":"Phone"}`))
	h.ServeHTTP(create, createReq)
	if create.Code != http.StatusOK || strings.Contains(create.Body.String(), "private-phone") {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}

	list := httptest.NewRecorder()
	h.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/gateway/clients", nil))
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "phone") {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}

	disable := httptest.NewRecorder()
	h.ServeHTTP(disable, httptest.NewRequest(http.MethodPost, "/api/gateway/clients/phone/disable", nil))
	if disable.Code != http.StatusOK || !strings.Contains(disable.Body.String(), `"disabled"`) {
		t.Fatalf("disable status=%d body=%s", disable.Code, disable.Body.String())
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(list.Body.String()), &envelope); err != nil {
		t.Fatal(err)
	}
}
