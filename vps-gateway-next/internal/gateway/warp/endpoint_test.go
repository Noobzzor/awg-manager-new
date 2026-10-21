package warp

import (
	"encoding/json"
	"testing"
)

func TestRenderEndpointPreservesValidatedWARPProfile(t *testing.T) {
	p := sampleProfile()
	raw, err := RenderEndpoint(p)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Endpoints []struct {
			Type       string   `json:"type"`
			Tag        string   `json:"tag"`
			Address    []string `json:"address"`
			MTU        int      `json:"mtu"`
			PrivateKey string   `json:"private_key"`
			Peers      []struct {
				Address    string   `json:"address"`
				Port       int      `json:"port"`
				PublicKey  string   `json:"public_key"`
				AllowedIPs []string `json:"allowed_ips"`
				Reserved   []int    `json:"reserved"`
				Keepalive  int      `json:"persistent_keepalive_interval"`
			} `json:"peers"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Endpoints) != 1 {
		t.Fatal("missing endpoint")
	}
	e := cfg.Endpoints[0]
	if e.Type != "wireguard" || e.Tag != "warp" || len(e.Address) != 1 || e.Address[0] != p.IPv4+"/32" || e.PrivateKey != p.PrivateKey || e.MTU != p.MTU || len(e.Peers) != 1 {
		t.Fatal("endpoint profile mismatch")
	}
	peer := e.Peers[0]
	if peer.Address != "162.159.193.10" || peer.Port != 2408 || peer.PublicKey != p.PublicKey || len(peer.AllowedIPs) != 1 || peer.AllowedIPs[0] != "0.0.0.0/0" || len(peer.Reserved) != 3 || peer.Reserved[1] != 127 || peer.Reserved[2] != 255 || peer.Keepalive != 25 {
		t.Fatal("peer profile mismatch")
	}
}
func TestRenderEndpointRejectsInvalidProfile(t *testing.T) {
	p := sampleProfile()
	p.Reserved = nil
	if raw, err := RenderEndpoint(p); err == nil || len(raw) > 0 {
		t.Fatal("invalid endpoint rendered")
	}
}
