package ingress

import (
	"net/netip"
	"strings"
	"testing"
)

func TestClientPublicRedactsPrivateMaterial(t *testing.T) {
	client, err := NewClient("client-a", "Phone", netip.MustParseAddr("10.66.0.1"), "private-key", "public-key")
	if err != nil {
		t.Fatal(err)
	}
	pub := client.Public()
	if pub.ID != "client-a" || pub.Address != netip.MustParseAddr("10.66.0.1") || pub.Status != StatusActive {
		t.Fatalf("unexpected public client: %+v", pub)
	}
	if strings.Contains(pub.String(), "private-key") {
		t.Fatalf("public representation leaked private key: %s", pub.String())
	}
}

func TestNewClientRejectsInvalidIdentityAndAddress(t *testing.T) {
	cases := []struct {
		name, id, label string
		addr            netip.Addr
	}{
		{"empty id", "", "x", netip.MustParseAddr("10.66.0.1")},
		{"empty key", "x", "", netip.MustParseAddr("10.66.0.1")},
		{"invalid address", "x", "y", netip.Addr{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewClient(tc.id, tc.label, tc.addr, "private", "public"); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
