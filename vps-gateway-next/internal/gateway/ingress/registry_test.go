package ingress

import (
	"net/netip"
	"testing"
)

func TestRegistryCreateDisableRevokeLifecycle(t *testing.T) {
	allocator, err := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(allocator, func(id string) (string, string, error) { return "private-" + id, "public-" + id, nil })
	created, err := r.Create("client-a", "Phone")
	if err != nil {
		t.Fatal(err)
	}
	if created.Address != netip.MustParseAddr("10.66.0.1") || created.Status != StatusActive {
		t.Fatalf("unexpected created client: %+v", created.Public())
	}
	if err := r.Disable("client-a"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Get("client-a"); got.Status != StatusDisabled {
		t.Fatalf("status after disable = %s", got.Status)
	}
	if err := r.Revoke("client-a"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.Get("client-a"); got.Status != StatusRevoked {
		t.Fatalf("status after revoke = %s", got.Status)
	}
	if err := r.Enable("client-a"); err == nil {
		t.Fatal("expected revoked client to remain disabled")
	}
}
