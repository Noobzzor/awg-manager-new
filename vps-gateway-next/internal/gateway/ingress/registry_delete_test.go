package ingress

import (
	"net/netip"
	"testing"
)

func TestRegistryDeleteReleasesClientAndAddress(t *testing.T) {
	a, _ := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	r := NewRegistry(a, func(string) (string, string, error) { return "private", "public", nil })
	client, err := r.Create("client-a", "Phone")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Delete(client.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Get(client.ID); ok {
		t.Fatal("deleted client still exists")
	}
	addr, err := a.Allocate("client-b")
	if err != nil {
		t.Fatal(err)
	}
	if addr != client.Address {
		t.Fatalf("released address=%s want %s", addr, client.Address)
	}
}
