package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"

	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
)

type fixedGatewayPeerApplier struct {
	recordingPeerApplier
	status  gatewayingress.PeerStatus
	readErr error
}

func (a fixedGatewayPeerApplier) ReadPeerStatus(_ context.Context, _ string) (gatewayingress.PeerStatus, error) {
	return a.status, a.readErr
}

func TestGatewayClientConfigDownloadIsPrivateAndActiveOnly(t *testing.T) {
	allocator, _ := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	_ = allocator.ReserveAddress(netip.MustParseAddr("10.66.0.1"))
	registry := gatewayingress.NewRegistry(allocator, func(string) (string, string, error) { return "client-private", "client-public", nil })
	client, err := registry.Create("phone", "Phone")
	if err != nil {
		t.Fatal(err)
	}
	h := newGatewayClientHTTP(registry, filepath.Join(t.TempDir(), "clients.json"), &fixedGatewayPeerApplier{
		status: gatewayingress.PeerStatus{PublicKey: "client-public"},
	})
	h.SetClientConfig("server-public", "gateway.example:51820")

	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/gateway/clients/phone/config", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	for name, want := range map[string]string{
		"Cache-Control":          "no-store, private, max-age=0",
		"Content-Disposition":    `attachment; filename="awg-client.conf"`,
		"X-Content-Type-Options": "nosniff",
	} {
		if got := response.Header().Get(name); got != want {
			t.Errorf("%s=%q want %q", name, got, want)
		}
	}
	if !strings.Contains(response.Body.String(), "PrivateKey = client-private") || !strings.Contains(response.Body.String(), "Address = 10.66.0.2/32") || strings.Contains(response.Body.String(), "PresharedKey") {
		t.Fatalf("unexpected client profile: %s", response.Body.String())
	}

	if err := registry.Disable(client.ID); err != nil {
		t.Fatal(err)
	}
	blocked := httptest.NewRecorder()
	h.ServeHTTP(blocked, httptest.NewRequest(http.MethodGet, "/api/gateway/clients/phone/config", nil))
	if blocked.Code != http.StatusConflict {
		t.Fatalf("disabled config status=%d body=%s", blocked.Code, blocked.Body.String())
	}
}

func TestGatewayClientConfigDownloadRequiresPublicEndpoint(t *testing.T) {
	allocator, _ := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	registry := gatewayingress.NewRegistry(allocator, func(string) (string, string, error) { return "private", "public", nil })
	_, _ = registry.Create("phone", "Phone")
	h := newGatewayClientHTTP(registry, filepath.Join(t.TempDir(), "clients.json"))
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/gateway/clients/phone/config", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestGatewayClientConfigFailsClosedWhenRuntimePeerIsMissing(t *testing.T) {
	allocator, _ := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	_ = allocator.ReserveAddress(netip.MustParseAddr("10.66.0.1"))
	registry := gatewayingress.NewRegistry(allocator, func(string) (string, string, error) { return "client-private", "client-public", nil })
	_, err := registry.Create("phone", "Phone")
	if err != nil {
		t.Fatal(err)
	}
	h := newGatewayClientHTTP(registry, filepath.Join(t.TempDir(), "clients.json"), &fixedGatewayPeerApplier{readErr: gatewayingress.ErrPeerNotFound})
	h.SetClientConfig("server-public", "gateway.example:51820")

	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/gateway/clients/phone/config", nil))
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "client-private") {
		t.Fatal("runtime-missing response leaked the client private key")
	}
}
