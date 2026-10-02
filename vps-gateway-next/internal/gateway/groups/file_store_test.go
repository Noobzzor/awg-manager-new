package groups

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStorePrivateVersionedRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "groups.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	want := []Group{{
		ID: "group-1", Name: "Family", ClientIDs: []string{"phone", "tablet"},
		CreatedAt: created, UpdatedAt: created.Add(time.Minute),
	}}
	if err := store.Save(want); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("store mode = %04o, want 0600", got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "groups.json" {
		t.Fatalf("temporary files were not cleaned up: %v", entries)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].ID != want[0].ID || loaded[0].Name != want[0].Name || len(loaded[0].ClientIDs) != 2 || loaded[0].ClientIDs[0] != "phone" || !loaded[0].UpdatedAt.Equal(want[0].UpdatedAt) {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
}
