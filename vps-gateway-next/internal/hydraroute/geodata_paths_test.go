package hydraroute

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveGeoConfigPath(t *testing.T) {
	tmp := t.TempDir()
	origHR := hrDir
	hrDir = filepath.Join(tmp, "HydraRoute")
	defer func() { hrDir = origHR }()

	if got := resolveGeoConfigPath("geo/geoip.dat"); got != filepath.Join(hrDir, "geo", "geoip.dat") {
		t.Fatalf("relative = %q", got)
	}
	abs := "/opt/etc/HydraRoute/geo/geoip_GA.dat"
	if got := resolveGeoConfigPath(abs); got != abs {
		t.Fatalf("absolute = %q", got)
	}
}

func TestIsManagedPath_AcceptsGeoAndHRDirs(t *testing.T) {
	tmp := t.TempDir()
	origHR := hrDir
	hrDir = filepath.Join(tmp, "HydraRoute")
	defer func() { hrDir = origHR }()

	store := &GeoDataStore{
		geoDir: filepath.Join(tmp, "geo"),
	}
	if !store.isManagedPath(filepath.Join(store.geoDir, "a.dat")) {
		t.Fatal("geoDir path should be managed")
	}
	if !store.isManagedPath(filepath.Join(hrDir, "b.dat")) {
		t.Fatal("hrDir path should be managed")
	}
	if store.isManagedPath(filepath.Join(tmp, "other.dat")) {
		t.Fatal("unrelated path should not be managed")
	}
}

func TestAdoptExternalFiles_KeepsHRPath(t *testing.T) {
	tmp := t.TempDir()
	origHR := hrDir
	hrDir = filepath.Join(tmp, "HydraRoute")
	if err := os.MkdirAll(hrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer func() { hrDir = origHR }()

	hrPath := filepath.Join(hrDir, "geoip_GA.dat")
	if err := os.WriteFile(hrPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &GeoDataStore{
		storagePath: filepath.Join(tmp, "hydraroute-geodata.json"),
		geoDir:      filepath.Join(tmp, "geo"),
		tagCache:    make(map[string][]GeoTag),
	}

	n, err := store.AdoptExternalFiles(&Config{GeoIPFiles: []string{hrPath}})
	if err != nil {
		t.Fatalf("AdoptExternalFiles: %v", err)
	}
	if n != 1 {
		t.Fatalf("adopted = %d, want 1", n)
	}
	if store.entries[0].Path != hrPath {
		t.Fatalf("path = %q, want %q (unchanged)", store.entries[0].Path, hrPath)
	}
	if _, err := os.Stat(hrPath); err != nil {
		t.Fatalf("file should remain in HR dir: %v", err)
	}
	if !store.entries[0].External {
		t.Fatal("expected External=true for HR path")
	}
}

func TestTakeControl_MovesFromHRDir(t *testing.T) {
	tmp := t.TempDir()
	origHR := hrDir
	hrDir = filepath.Join(tmp, "HydraRoute")
	if err := os.MkdirAll(hrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer func() { hrDir = origHR }()

	hrPath := filepath.Join(hrDir, "geosite_GA.dat")
	if err := os.WriteFile(hrPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &GeoDataStore{
		storagePath: filepath.Join(tmp, "hydraroute-geodata.json"),
		geoDir:      filepath.Join(tmp, "geo"),
		tagCache:    make(map[string][]GeoTag),
	}
	if err := os.MkdirAll(store.geoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.entries = []GeoFileEntry{
		{Type: "geosite", Path: hrPath, External: true, URL: GroundZerroGeoSiteURL},
	}
	store.mu.Unlock()

	entry, err := store.TakeControl(hrPath)
	if err != nil {
		t.Fatalf("TakeControl: %v", err)
	}
	if entry.External {
		t.Fatal("expected External=false after take control")
	}
	if !hasPathPrefix(entry.Path, store.geoDir) {
		t.Fatalf("path = %q, want under %q", entry.Path, store.geoDir)
	}
	if _, err := os.Stat(hrPath); !os.IsNotExist(err) {
		t.Fatalf("HR file should be gone: %v", err)
	}
	if _, err := os.Stat(entry.Path); err != nil {
		t.Fatalf("geo dir file missing: %v", err)
	}
}
