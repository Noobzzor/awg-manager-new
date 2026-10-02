package ingress

import (
	"context"
	"net/netip"
	"testing"
)

type fakePeerApplier struct {
	apply  int
	remove int
	last   PeerConfig
}

func (f *fakePeerApplier) ApplyPeer(_ context.Context, peer PeerConfig) error {
	f.apply++
	f.last = peer
	return nil
}
func (f *fakePeerApplier) RemovePeer(_ context.Context, _ string) error { f.remove++; return nil }

func TestBuildPeerConfigRejectsNonActiveClientsAndRedactsPSK(t *testing.T) {
	client, err := NewClient("client-a", "Phone", netip.MustParseAddr("10.66.0.1"), "private", "public")
	if err != nil {
		t.Fatal(err)
	}
	peer, err := BuildPeerConfig(client, "psk-secret")
	if err != nil || peer.PublicKey != "public" || peer.AllowedIP.String() != "10.66.0.1/32" {
		t.Fatalf("peer config = %+v, err=%v", peer, err)
	}
	if contains(peer.String(), "psk-secret") {
		t.Fatalf("peer string leaked preshared key: %s", peer.String())
	}
	client.Disable()
	if _, err := BuildPeerConfig(client, "psk-secret"); err == nil {
		t.Fatal("expected disabled client to be rejected")
	}
	client.Revoke()
	if _, err := BuildPeerConfig(client, "psk-secret"); err == nil {
		t.Fatal("expected revoked client to be rejected")
	}
}

func TestRegistrySyncRemovesDisabledPeer(t *testing.T) {
	a, _ := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	r := NewRegistry(a, func(id string) (string, string, error) { return "private", "public-" + id, nil })
	if _, err := r.Create("client-a", "Phone"); err != nil {
		t.Fatal(err)
	}
	fake := &fakePeerApplier{}
	if err := r.SyncClient(context.Background(), "client-a", "psk", fake); err != nil {
		t.Fatal(err)
	}
	if fake.apply != 1 || fake.last.AllowedIP.String() != "10.66.0.1/32" {
		t.Fatalf("apply=%d peer=%+v", fake.apply, fake.last)
	}
	if err := r.Disable("client-a"); err != nil {
		t.Fatal(err)
	}
	if err := r.SyncClient(context.Background(), "client-a", "psk", fake); err != nil {
		t.Fatal(err)
	}
	if fake.remove != 1 {
		t.Fatalf("remove=%d", fake.remove)
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && stringIndex(s, sub) >= 0
}
func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
