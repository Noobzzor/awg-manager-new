package subscription

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFileStoreUsesPrivateAtomicVersionedRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "subscriptions.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	expires := created.Add(24 * time.Hour)
	want := []StoredSubscription{{
		Metadata:  Metadata{ID: "sub-1", ClientID: "phone-1", CreatedAt: created, ExpiresAt: expires},
		TokenHash: strings.Repeat("a", 64),
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
	if len(entries) != 1 || entries[0].Name() != "subscriptions.json" {
		t.Fatalf("temporary files were not cleaned up: %v", entries)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].ID != "sub-1" || loaded[0].ClientID != "phone-1" || loaded[0].TokenHash != want[0].TokenHash || !loaded[0].ExpiresAt.Equal(expires) {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
}

func TestFileStoreTreatsMissingFileAsEmptyAndRejectsUnknownVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "subscriptions.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil || len(loaded) != 0 {
		t.Fatalf("missing-file Load() = %v, %v", loaded, err)
	}
	if err := os.WriteFile(path, []byte(`{"version":99,"subscriptions":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(); err == nil {
		t.Fatal("Load() accepted an unsupported version")
	}
}
