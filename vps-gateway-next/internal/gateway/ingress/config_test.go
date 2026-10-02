package ingress

import (
	"net/netip"
	"strings"
	"testing"
)

func TestRenderClientConfigContainsRequiredFieldsOnly(t *testing.T) {
	client, err := NewClient("client-a", "Phone", netip.MustParseAddr("10.66.0.1"), "client-private", "client-public")
	if err != nil {
		t.Fatal(err)
	}
	conf, err := RenderClientConfig(client, "server-public", "198.51.100.10:51820", "10.66.0.1/32", "psk")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[Interface]", "PrivateKey = client-private", "Address = 10.66.0.1/32", "[Peer]", "PublicKey = server-public", "Endpoint = 198.51.100.10:51820", "PresharedKey = psk"} {
		if !strings.Contains(conf, want) {
			t.Fatalf("config missing %q: %s", want, conf)
		}
	}
}

func TestRenderClientConfigAllowsNoPresharedKey(t *testing.T) {
	client, err := NewClient("client-a", "Phone", netip.MustParseAddr("10.66.0.2"), "client-private", "client-public")
	if err != nil {
		t.Fatal(err)
	}
	conf, err := RenderClientConfig(client, "server-public", "gateway.example:51820", "10.66.0.2/32", "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(conf, "PresharedKey") || !strings.Contains(conf, "PrivateKey = client-private") {
		t.Fatalf("unexpected client config: %s", conf)
	}
}
