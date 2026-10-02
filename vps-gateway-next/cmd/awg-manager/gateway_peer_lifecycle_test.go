package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
)

type recordingPeerApplier struct {
	applied []gatewayingress.PeerConfig
	removed []string
	err     error
}

func (a *recordingPeerApplier) ApplyPeer(_ context.Context, peer gatewayingress.PeerConfig) error {
	a.applied = append(a.applied, peer)
	return a.err
}
func (a *recordingPeerApplier) RemovePeer(_ context.Context, key string) error {
	a.removed = append(a.removed, key)
	return a.err
}

func TestGatewayClientHTTPCreateAppliesPeerBeforeSuccess(t *testing.T) {
	allocator, _ := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	_ = allocator.ReserveAddress(netip.MustParseAddr("10.66.0.1"))
	registry := gatewayingress.NewRegistry(allocator, func(id string) (string, string, error) {
		return "client-private", "client-public", nil
	})
	applier := &recordingPeerApplier{}
	h := newGatewayClientHTTP(registry, filepath.Join(t.TempDir(), "clients.json"), applier)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/gateway/clients/create", strings.NewReader(`{"id":"phone","label":"Phone"}`)))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(applier.applied) != 1 || applier.applied[0].PublicKey != "client-public" || applier.applied[0].AllowedIP != netip.MustParsePrefix("10.66.0.2/32") {
		t.Fatalf("peer apply calls=%#v", applier.applied)
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(response.Body.String(), "client-private") {
		t.Fatal("HTTP response leaked private key")
	}
}

func TestGatewayClientHTTPTransitionsApplyAndRemovePeers(t *testing.T) {
	allocator, _ := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	_ = allocator.ReserveAddress(netip.MustParseAddr("10.66.0.1"))
	registry := gatewayingress.NewRegistry(allocator, func(string) (string, string, error) { return "private", "public", nil })
	applier := &recordingPeerApplier{}
	h := newGatewayClientHTTP(registry, filepath.Join(t.TempDir(), "clients.json"), applier)
	invoke := func(path, method, body string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
		return response
	}
	for _, step := range []struct{ path, method, body string }{
		{"/api/gateway/clients/create", http.MethodPost, `{"id":"phone","label":"Phone"}`},
		{"/api/gateway/clients/phone/disable", http.MethodPost, ""},
		{"/api/gateway/clients/phone/enable", http.MethodPost, ""},
		{"/api/gateway/clients/phone/revoke", http.MethodPost, ""},
	} {
		if response := invoke(step.path, step.method, step.body); response.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", step.path, response.Code, response.Body.String())
		}
	}
	if len(applier.applied) != 2 || len(applier.removed) != 2 {
		t.Fatalf("applied=%d removed=%d", len(applier.applied), len(applier.removed))
	}
}

func TestGatewayClientHTTPCreateRollsBackWhenPeerApplyFails(t *testing.T) {
	allocator, _ := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	registry := gatewayingress.NewRegistry(allocator, func(string) (string, string, error) { return "private", "public", nil })
	applier := &recordingPeerApplier{err: errors.New("synthetic apply failure")}
	h := newGatewayClientHTTP(registry, filepath.Join(t.TempDir(), "clients.json"), applier)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/gateway/clients/create", strings.NewReader(`{"id":"phone","label":"Phone"}`)))
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if _, ok := registry.Get("phone"); ok {
		t.Fatal("client remained after peer apply failure")
	}
}
