package ingress

import (
	"net/netip"
	"testing"
)

func TestAllocatorReturnsStableFreeAddressesAndReusesRelease(t *testing.T) {
	a, err := NewAllocator(netip.MustParsePrefix("10.66.0.0/30"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := a.Allocate("client-a")
	if err != nil || first != netip.MustParseAddr("10.66.0.1") {
		t.Fatalf("first allocation = %v, %v", first, err)
	}
	second, err := a.Allocate("client-b")
	if err != nil || second != netip.MustParseAddr("10.66.0.2") {
		t.Fatalf("second allocation = %v, %v", second, err)
	}
	again, err := a.Allocate("client-a")
	if err != nil || again != first {
		t.Fatalf("reallocation = %v, %v", again, err)
	}
	if err := a.Release("client-a"); err != nil {
		t.Fatal(err)
	}
	reused, err := a.Allocate("client-c")
	if err != nil || reused != first {
		t.Fatalf("reused allocation = %v, %v", reused, err)
	}
}

func TestAllocatorExhaustionDoesNotReturnNetworkOrBroadcast(t *testing.T) {
	a, err := NewAllocator(netip.MustParsePrefix("10.66.0.0/30"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b"} {
		if _, err := a.Allocate(id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.Allocate("c"); err == nil {
		t.Fatal("expected exhaustion")
	}
}
