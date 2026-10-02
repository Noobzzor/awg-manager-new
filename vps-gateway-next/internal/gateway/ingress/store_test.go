package ingress

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
)

func TestRegistryStoreRoundTripPreservesAddressStatusAndSecretsInternally(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clients.json")
	allocator, _ := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	r := NewRegistry(allocator, func(id string) (string, string, error) { return "private-" + id, "public-" + id, nil })
	created, err := r.Create("client-a", "Phone")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Disable("client-a"); err != nil {
		t.Fatal(err)
	}
	if err := Save(r, path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("store mode = %v, err=%v", info.Mode().Perm(), err)
	}

	restoredAllocator, _ := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	restored, err := Load(path, restoredAllocator, func(id string) (string, string, error) { return "private-" + id, "public-" + id, nil })
	if err != nil {
		t.Fatal(err)
	}
	got, ok := restored.Get("client-a")
	if !ok || got.Address != created.Address || got.Status != StatusDisabled {
		t.Fatalf("restored client = %+v, ok=%v", got, ok)
	}
	if err := restored.Enable("client-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := restored.Create("client-b", "Tablet"); err != nil {
		t.Fatalf("allocator was not rebuilt from persisted client: %v", err)
	}
}
