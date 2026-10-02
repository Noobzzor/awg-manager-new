package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreSavesValidProfilesPrivatelyAndLoadsThem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "profiles.json")
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Profile{{
		ID:            "phone-default",
		Name:          "Phone default",
		DefaultAction: ActionVPN,
		Rules: []Rule{{
			ID:             "work-sites",
			Action:         ActionWARP,
			Priority:       10,
			ClientIDs:      []string{"phone"},
			DomainSuffixes: []string{"example.com"},
			Outbound:       "warp",
			Enabled:        true,
		}},
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
	if len(entries) != 1 || entries[0].Name() != "profiles.json" {
		t.Fatalf("temporary files were not cleaned up: %v", entries)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != len(want) || loaded[0].ID != want[0].ID || loaded[0].Name != want[0].Name || loaded[0].DefaultAction != want[0].DefaultAction {
		t.Fatalf("profile round trip mismatch: %+v", loaded)
	}
	if len(loaded[0].Rules) != 1 || loaded[0].Rules[0].ID != "work-sites" || !loaded[0].Rules[0].Enabled || loaded[0].Rules[0].DomainSuffixes[0] != "example.com" {
		t.Fatalf("rule round trip mismatch: %+v", loaded[0].Rules)
	}
}

func TestFileStoreMigratesV1RulesAsEnabledToPreserveExistingBehavior(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	legacy := `{"version":1,"profiles":[{"id":"p","name":"P","defaultAction":"direct","rules":[{"id":"r","action":"block","enabled":false}]}]}`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	profiles, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 1 || len(profiles[0].Rules) != 1 || !profiles[0].Rules[0].Enabled {
		t.Fatalf("legacy active rule was not preserved: %+v", profiles)
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload fileStorePayload
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Version != fileStoreVersion || !payload.Profiles[0].Rules[0].Enabled {
		t.Fatalf("store was not migrated: %+v", payload)
	}
}
