package ingress

import (
	"net/netip"
	"testing"
)

func TestAllocatorSkipsReservedServerAddress(t *testing.T) {
	a, err := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.ReserveAddress(netip.MustParseAddr("10.66.0.1")); err != nil {
		t.Fatal(err)
	}
	addr, err := a.Allocate("client-a")
	if err != nil {
		t.Fatal(err)
	}
	if addr != netip.MustParseAddr("10.66.0.2") {
		t.Fatalf("allocated %s, want 10.66.0.2", addr)
	}
}
