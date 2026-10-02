package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/ndms"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

type fakeHotspot struct {
	devices []ndms.Device
	err     error
}

func (f *fakeHotspot) List(ctx context.Context) ([]ndms.Device, error) {
	return f.devices, f.err
}

type fakeWGServers struct {
	servers []ndms.WireguardServer
	err     error
}

func (f *fakeWGServers) ListServers(ctx context.Context) ([]ndms.WireguardServer, error) {
	return f.servers, f.err
}

type fakeManagedServers struct {
	servers []storage.ManagedServer
}

func (f *fakeManagedServers) List() []storage.ManagedServer {
	return f.servers
}

func clientsMap(t *testing.T, h *SingboxConnectionsHandler) map[string]string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.Clients(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	var resp SingboxConnectionsClientsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return resp.Data.ClientsByIP
}

func TestSingboxConnections_Clients_HappyPath(t *testing.T) {
	hot := &fakeHotspot{devices: []ndms.Device{
		{IP: "192.168.1.5", Name: "iPhone"},
		{IP: "192.168.1.7", Name: "macbook"},
		{IP: "192.168.1.9", Hostname: "android-tablet"},
	}}
	h := NewSingboxConnectionsHandler(hot)

	req := httptest.NewRequest(http.MethodGet, "/api/singbox/connections/clients", nil)
	rec := httptest.NewRecorder()
	h.Clients(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	var resp SingboxConnectionsClientsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success=false")
	}
	want := map[string]string{
		"192.168.1.5": "iPhone",
		"192.168.1.7": "macbook",
		"192.168.1.9": "android-tablet",
	}
	if len(resp.Data.ClientsByIP) != len(want) {
		t.Fatalf("map size: got %d, want %d", len(resp.Data.ClientsByIP), len(want))
	}
	for k, v := range want {
		if got := resp.Data.ClientsByIP[k]; got != v {
			t.Errorf("key %s: got %q, want %q", k, got, v)
		}
	}
}

func TestSingboxConnections_Clients_EmptyHotspot(t *testing.T) {
	h := NewSingboxConnectionsHandler(&fakeHotspot{devices: nil})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.Clients(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	var resp SingboxConnectionsClientsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Data.ClientsByIP == nil {
		t.Fatal("clientsByIP should be non-nil empty map, not nil")
	}
	if len(resp.Data.ClientsByIP) != 0 {
		t.Fatalf("expected empty map, got %v", resp.Data.ClientsByIP)
	}
}

func TestSingboxConnections_Clients_HotspotError(t *testing.T) {
	h := NewSingboxConnectionsHandler(&fakeHotspot{err: errors.New("ndms boom")})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.Clients(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (best-effort)", rec.Code)
	}
	var resp SingboxConnectionsClientsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data.ClientsByIP) != 0 {
		t.Fatalf("expected empty map on error, got %v", resp.Data.ClientsByIP)
	}
}

func TestSingboxConnections_Clients_NilHotspot(t *testing.T) {
	h := NewSingboxConnectionsHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.Clients(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", rec.Code)
	}
}

func TestSingboxConnections_Clients_NotGet(t *testing.T) {
	h := NewSingboxConnectionsHandler(&fakeHotspot{})
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	h.Clients(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d, want 405", rec.Code)
	}
}

func TestSingboxConnections_Clients_LowercaseIPKeys(t *testing.T) {
	hot := &fakeHotspot{devices: []ndms.Device{
		{IP: "FE80::1234", Name: "ipv6-host"},
	}}
	h := NewSingboxConnectionsHandler(hot)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.Clients(rec, req)
	var resp SingboxConnectionsClientsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if got := resp.Data.ClientsByIP["fe80::1234"]; got != "ipv6-host" {
		t.Fatalf("expected lowercase key match, got map=%v", resp.Data.ClientsByIP)
	}
}

func TestSingboxConnections_Clients_PrefersNameOverHostname(t *testing.T) {
	hot := &fakeHotspot{devices: []ndms.Device{
		{IP: "192.168.1.5", Name: "iPhone", Hostname: "anya-iphone"},
	}}
	h := NewSingboxConnectionsHandler(hot)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.Clients(rec, req)
	var resp SingboxConnectionsClientsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if got := resp.Data.ClientsByIP["192.168.1.5"]; got != "iPhone" {
		t.Fatalf("expected Name to win, got %q", got)
	}
}

func TestSingboxConnections_Clients_MergesWGServerPeers(t *testing.T) {
	h := NewSingboxConnectionsHandler(&fakeHotspot{devices: []ndms.Device{
		{IP: "192.168.1.5", Name: "iPhone"},
	}})
	h.SetWGServers(&fakeWGServers{servers: []ndms.WireguardServer{
		{ID: "Wireguard1", Peers: []ndms.WireguardServerPeer{
			{PublicKey: "a", Description: "Anya Phone", AllowedIPs: []string{"10.0.14.2/32"}},
			{PublicKey: "b", Description: "", AllowedIPs: []string{"10.0.14.3/32"}},       // empty description — skipped
			{PublicKey: "c", Description: "No IPs"},                                       // no allowedIPs — skipped
			{PublicKey: "d", Description: "Subnet", AllowedIPs: []string{"10.0.15.0/24"}}, // not a host — skipped
			{PublicKey: "e", Description: "V6 Peer", AllowedIPs: []string{"FD00::2/128"}},
		}},
	}})
	h.SetManagedServers(&fakeManagedServers{servers: []storage.ManagedServer{
		{InterfaceName: "Wireguard3", Peers: []storage.ManagedPeer{
			{PublicKey: "m1", Description: "Managed Laptop", TunnelIP: "10.20.30.2/32"},
			{PublicKey: "m2", Description: "", TunnelIP: "10.20.30.3/32"}, // empty description — skipped
			{PublicKey: "m3", Description: "Bare IP", TunnelIP: "10.20.30.4"},
		}},
	}})

	got := clientsMap(t, h)
	want := map[string]string{
		"192.168.1.5": "iPhone",
		"10.0.14.2":   "Anya Phone",     // mask stripped
		"fd00::2":     "V6 Peer",        // /128 stripped + lowercased
		"10.20.30.2":  "Managed Laptop", // managed TunnelIP mask stripped
		"10.20.30.4":  "Bare IP",        // TunnelIP without mask accepted
	}
	if len(got) != len(want) {
		t.Fatalf("map size: got %d (%v), want %d", len(got), got, len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("key %s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestSingboxConnections_Clients_HotspotWinsOverPeer(t *testing.T) {
	h := NewSingboxConnectionsHandler(&fakeHotspot{devices: []ndms.Device{
		{IP: "10.0.14.2", Name: "LAN Name"},
	}})
	h.SetWGServers(&fakeWGServers{servers: []ndms.WireguardServer{
		{ID: "Wireguard1", Peers: []ndms.WireguardServerPeer{
			{PublicKey: "a", Description: "Peer Name", AllowedIPs: []string{"10.0.14.2/32"}},
		}},
	}})
	if got := clientsMap(t, h)["10.0.14.2"]; got != "LAN Name" {
		t.Fatalf("expected hotspot name to win on collision, got %q", got)
	}
}

func TestSingboxConnections_Clients_NilPeerSourcesKeepOldBehavior(t *testing.T) {
	h := NewSingboxConnectionsHandler(&fakeHotspot{devices: []ndms.Device{
		{IP: "192.168.1.5", Name: "iPhone"},
	}})
	got := clientsMap(t, h)
	if len(got) != 1 || got["192.168.1.5"] != "iPhone" {
		t.Fatalf("expected hotspot-only map, got %v", got)
	}
}

func TestSingboxConnections_Clients_WGServerErrorStillOK(t *testing.T) {
	h := NewSingboxConnectionsHandler(&fakeHotspot{devices: []ndms.Device{
		{IP: "192.168.1.5", Name: "iPhone"},
	}})
	h.SetWGServers(&fakeWGServers{err: errors.New("ndms boom")})
	h.SetManagedServers(&fakeManagedServers{servers: []storage.ManagedServer{
		{Peers: []storage.ManagedPeer{{Description: "Managed", TunnelIP: "10.20.30.2/32"}}},
	}})
	got := clientsMap(t, h) // clientsMap asserts 200
	if got["192.168.1.5"] != "iPhone" || got["10.20.30.2"] != "Managed" {
		t.Fatalf("expected other sources to survive wg-server error, got %v", got)
	}
}

func TestSingboxConnections_Clients_PeersSurviveHotspotError(t *testing.T) {
	h := NewSingboxConnectionsHandler(&fakeHotspot{err: errors.New("ndms boom")})
	h.SetWGServers(&fakeWGServers{servers: []ndms.WireguardServer{
		{ID: "Wireguard1", Peers: []ndms.WireguardServerPeer{
			{PublicKey: "a", Description: "Peer", AllowedIPs: []string{"10.0.14.2/32"}},
		}},
	}})
	if got := clientsMap(t, h)["10.0.14.2"]; got != "Peer" {
		t.Fatalf("expected peer names despite hotspot error, got %q", got)
	}
}

func TestSingboxConnections_Clients_SkipsEmptyName(t *testing.T) {
	hot := &fakeHotspot{devices: []ndms.Device{
		{IP: "192.168.1.5"},
		{IP: "192.168.1.6", Name: "named"},
	}}
	h := NewSingboxConnectionsHandler(hot)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.Clients(rec, req)
	var resp SingboxConnectionsClientsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if _, present := resp.Data.ClientsByIP["192.168.1.5"]; present {
		t.Fatalf("expected nameless device to be skipped, got %v", resp.Data.ClientsByIP)
	}
	if resp.Data.ClientsByIP["192.168.1.6"] != "named" {
		t.Fatal("named device missing")
	}
}

func TestSingboxConnections_Clients_FirstHostEntryWins(t *testing.T) {
	h := NewSingboxConnectionsHandler(&fakeHotspot{})
	h.SetWGServers(&fakeWGServers{servers: []ndms.WireguardServer{
		{ID: "Wireguard1", Peers: []ndms.WireguardServerPeer{
			// site-to-site: маршрутизируемая подсеть впереди host-записи
			{PublicKey: "a", Description: "Office", AllowedIPs: []string{"192.168.20.0/24", "10.0.14.5/32"}},
			// full-tunnel: 0.0.0.0/0 впереди host-записи
			{PublicKey: "b", Description: "Road Warrior", AllowedIPs: []string{"0.0.0.0/0", "10.0.14.6/32"}},
			// только не-host записи — пропускается
			{PublicKey: "c", Description: "No Host", AllowedIPs: []string{"0.0.0.0/0", "10.0.16.0/24"}},
		}},
	}})

	got := clientsMap(t, h)
	want := map[string]string{
		"10.0.14.5": "Office",
		"10.0.14.6": "Road Warrior",
	}
	if len(got) != len(want) {
		t.Fatalf("map size: got %d (%v), want %d", len(got), got, len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("key %s: got %q, want %q", k, got[k], v)
		}
	}
}

func TestSingboxConnections_Clients_CanonicalIPv6Key(t *testing.T) {
	h := NewSingboxConnectionsHandler(&fakeHotspot{})
	h.SetWGServers(&fakeWGServers{servers: []ndms.WireguardServer{
		{ID: "Wireguard1", Peers: []ndms.WireguardServerPeer{
			{PublicKey: "a", Description: "V6 Long", AllowedIPs: []string{"fd00:0:0::2/128"}},
			{PublicKey: "b", Description: "V6 Zeros", AllowedIPs: []string{"fd00:0000::0003/128"}},
		}},
	}})

	got := clientsMap(t, h)
	if got["fd00::2"] != "V6 Long" {
		t.Errorf("fd00::2: got %q, want %q (map %v)", got["fd00::2"], "V6 Long", got)
	}
	if got["fd00::3"] != "V6 Zeros" {
		t.Errorf("fd00::3: got %q, want %q", got["fd00::3"], "V6 Zeros")
	}
}
