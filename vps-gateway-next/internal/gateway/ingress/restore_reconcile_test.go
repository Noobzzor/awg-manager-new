package ingress

import (
	"context"
	"net/netip"
	"path/filepath"
	"testing"
)

func TestLoadPreservesRevokedStatusAndRejectsEnable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clients.json")
	allocator, _ := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	registry := NewRegistry(allocator, func(string) (string, string, error) { return "private", "public", nil })
	client, err := registry.Create("revoked-client", "Revoked")
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Revoke(client.ID); err != nil {
		t.Fatal(err)
	}
	if err := Save(registry, path); err != nil {
		t.Fatal(err)
	}

	restoredAllocator, _ := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	restored, err := Load(path, restoredAllocator, nil)
	if err != nil {
		t.Fatal(err)
	}
	loaded, ok := restored.Get(client.ID)
	if !ok || loaded.Status != StatusRevoked {
		t.Fatalf("restored client=%+v, exists=%v", loaded, ok)
	}
	if err := restored.Enable(client.ID); err == nil {
		t.Fatal("revoked client was enabled after restore")
	}
}

func TestRegistryReconcilePeersAppliesActiveAndRemovesInactive(t *testing.T) {
	allocator, _ := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	registry := NewRegistry(allocator, func(id string) (string, string, error) { return "private-" + id, "public-" + id, nil })
	active, err := registry.Create("active", "Active")
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := registry.Create("disabled", "Disabled")
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Disable(disabled.ID); err != nil {
		t.Fatal(err)
	}
	revoked, err := registry.Create("revoked", "Revoked")
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Revoke(revoked.ID); err != nil {
		t.Fatal(err)
	}

	applier := &fakePeerApplier{}
	if err := registry.ReconcilePeers(context.Background(), applier); err != nil {
		t.Fatal(err)
	}
	if applier.apply != 1 || applier.last.PublicKey != active.PublicKey {
		t.Fatalf("active applies=%d last=%+v", applier.apply, applier.last)
	}
	if applier.remove != 2 {
		t.Fatalf("inactive removals=%d, want 2", applier.remove)
	}
}
