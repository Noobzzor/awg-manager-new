package ingress

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"testing"
)

func TestPublicListAndOperationalErrorsDoNotLeakSecrets(t *testing.T) {
	allocator, _ := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	r := NewRegistry(allocator, func(id string) (string, string, error) { return "private-secret-" + id, "public-" + id, nil })
	if _, err := r.Create("client-a", "Phone"); err != nil {
		t.Fatal(err)
	}
	public := fmt.Sprintf("%v", r.List())
	if strings.Contains(public, "private-secret") || strings.Contains(public, "psk-secret") {
		t.Fatalf("public list leaked secret: %s", public)
	}
	applier := NewCommandApplier("awg", "awg0", func(_ context.Context, _ string, _ ...string) (string, error) {
		return "", fmt.Errorf("command failed")
	})
	peer := PeerConfig{PublicKey: "public", AllowedIP: netip.MustParsePrefix("10.66.0.1/32"), presharedKey: "psk-secret"}
	if err := applier.ApplyPeer(context.Background(), peer); err == nil || strings.Contains(err.Error(), "psk-secret") {
		t.Fatalf("runtime error leaked secret: %v", err)
	}
}
