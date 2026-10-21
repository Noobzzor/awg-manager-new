package warp

import (
	"encoding/json"
	"net/netip"
)

func RenderEndpoint(p Profile) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	ap, _ := netip.ParseAddrPort(p.Endpoint)
	return json.MarshalIndent(map[string]any{"endpoints": []any{map[string]any{"type": "wireguard", "tag": Tag, "address": []string{p.IPv4 + "/32"}, "mtu": p.MTU, "private_key": p.PrivateKey, "peers": []any{map[string]any{"address": ap.Addr().String(), "port": ap.Port(), "public_key": p.PublicKey, "allowed_ips": []string{"0.0.0.0/0"}, "reserved": p.Reserved, "persistent_keepalive_interval": 25}}}}}, "", "  ")
}
