package hydraroute

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReconcile_FixesStaleExternalOnAWGMPaths(t *testing.T) {
	store := newTestGeoStore(t)
	awgmPath := filepath.Join(store.geoDir, "geoip_GA.dat")
	if err := os.WriteFile(awgmPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	store.entries = []GeoFileEntry{
		{Type: "geoip", Path: awgmPath, External: true, URL: GroundZerroGeoIPURL},
	}
	store.mu.Unlock()

	store.mu.Lock()
	changed := store.reconcileUnlocked()
	store.mu.Unlock()
	if !changed {
		t.Fatal("expected reconcile to fix external flag")
	}
	if store.entries[0].External {
		t.Fatal("AWGM path should not be external after reconcile")
	}
}

func TestReconcile_PruneMissingFiles(t *testing.T) {
	store := newTestGeoStore(t)
	store.mu.Lock()
	store.entries = []GeoFileEntry{
		{Type: "geoip", Path: filepath.Join(store.geoDir, "missing.dat"), External: false},
	}
	changed := store.reconcileUnlocked()
	store.mu.Unlock()
	if !changed {
		t.Fatal("expected reconcile to drop missing file")
	}
	if len(store.entries) != 0 {
		t.Fatalf("entries = %+v, want empty", store.entries)
	}
}

func TestReconcile_KeepsBothAWGMAndHRWhenBothExist(t *testing.T) {
	tmp := t.TempDir()
	origHR := hrDir
	hrDir = filepath.Join(tmp, "HydraRoute")
	if err := os.MkdirAll(hrDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer func() { hrDir = origHR }()

	store := newTestGeoStore(t)
	awgmPath := filepath.Join(store.geoDir, "geoip_GA.dat")
	hrPath := filepath.Join(hrDir, "geofile", "geoip_GA.dat")
	if err := os.MkdirAll(filepath.Dir(hrPath), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{awgmPath, hrPath} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	store.mu.Lock()
	store.entries = []GeoFileEntry{
		{Type: "geoip", Path: hrPath, External: false},
		{Type: "geoip", Path: awgmPath, External: true},
	}
	changed := store.reconcileUnlocked()
	store.mu.Unlock()
	if !changed {
		t.Fatal("expected reconcile to fix external flags")
	}
	if len(store.entries) != 2 {
		t.Fatalf("entries = %d, want 2 (AWGM + HR)", len(store.entries))
	}
	byPath := map[string]GeoFileEntry{}
	for _, e := range store.entries {
		byPath[e.Path] = e
	}
	if byPath[awgmPath].External {
		t.Fatal("AWGM entry should not be external")
	}
	if !byPath[hrPath].External {
		t.Fatal("HR entry should be external")
	}
}

func TestLoad_ReconcilesPersistedJSON(t *testing.T) {
	tmp := t.TempDir()
	geoDir := filepath.Join(tmp, "geo")
	if err := os.MkdirAll(geoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	awgmPath := filepath.Join(geoDir, "geoip_GA.dat")
	if err := os.WriteFile(awgmPath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	storagePath := filepath.Join(tmp, "hydraroute-geodata.json")
	doc := `{"files":[{"type":"geoip","path":"` + awgmPath + `","external":true,"url":"https://example.com/x.dat"}]}`
	if err := os.WriteFile(storagePath, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}

	store := &GeoDataStore{
		storagePath: storagePath,
		geoDir:      geoDir,
		tagCache:    make(map[string][]GeoTag),
	}
	if err := store.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(store.entries) != 1 || store.entries[0].External {
		t.Fatalf("after load: %+v", store.entries)
	}
	raw, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) == doc {
		t.Fatal("expected load to persist reconciled external flag")
	}
}

func TestList_ReconcilesMissingFileOnDisk(t *testing.T) {
	store := newTestGeoStore(t)
	path := filepath.Join(store.geoDir, "vanished.dat")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	store.mu.Lock()
	store.entries = []GeoFileEntry{{Type: "geoip", Path: path}}
	store.mu.Unlock()

	if len(store.List()) != 1 {
		t.Fatal("expected one entry before manual delete")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if entries := store.List(); len(entries) != 0 {
		t.Fatalf("List after manual delete: %+v, want empty", entries)
	}
}

func TestDedupeGeoPaths(t *testing.T) {
	in := []string{"/a/x.dat", "/b/x.dat", "/a/x.dat"}
	out := dedupeGeoPaths(in)
	if len(out) != 2 || out[0] != "/a/x.dat" || out[1] != "/b/x.dat" {
		t.Fatalf("got %v", out)
	}
}

// TestRescanAndSync_PrunesMissingHRPathsFromCatalog reproduces the user report:
// hrneo.conf lists AWGM + HR paths, stale JSON marks everything external, but HR
// files are missing on disk. Reconcile drops HR ghosts; sync writes only AWGM paths.
func TestRescanAndSync_PrunesMissingHRPathsFromCatalog(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "awg-manager")
	awgmGeo := filepath.Join(dataDir, "geo")
	if err := os.MkdirAll(awgmGeo, 0o755); err != nil {
		t.Fatal(err)
	}

	origHR := hrDir
	origConf := hrConfPath
	hrDir = filepath.Join(root, "HydraRoute")
	hrConfPath = filepath.Join(hrDir, "hrneo.conf")
	t.Cleanup(func() {
		hrDir = origHR
		hrConfPath = origConf
	})
	if err := os.MkdirAll(hrDir, 0o755); err != nil {
		t.Fatal(err)
	}

	awgmGeoIP := filepath.Join(awgmGeo, "geoip_GA.dat")
	awgmGeoSite := filepath.Join(awgmGeo, "geosite_GA.dat")
	hrGeoIP := filepath.Join(hrDir, "geofile", "geoip_GA.dat")
	hrGeoSite := filepath.Join(hrDir, "geofile", "geosite_GA.dat")

	for _, p := range []string{awgmGeoIP, awgmGeoSite} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	conf := `AutoStart=false
GeoIPFile=` + awgmGeoIP + `
GeoIPFile=` + hrGeoIP + `
GeoSiteFile=` + awgmGeoSite + `
GeoSiteFile=` + hrGeoSite + `
`
	if err := os.WriteFile(hrConfPath, []byte(conf), 0o644); err != nil {
		t.Fatal(err)
	}

	// Stale catalog: four entries, all external (legacy adopt).
	storagePath := filepath.Join(dataDir, geoDataFile)
	stale := `{
  "files": [
    {"type":"geoip","path":"` + awgmGeoIP + `","external":true,"url":"` + GroundZerroGeoIPURL + `"},
    {"type":"geosite","path":"` + awgmGeoSite + `","external":true,"url":"` + GroundZerroGeoSiteURL + `"},
    {"type":"geoip","path":"` + hrGeoIP + `","external":true,"url":"` + GroundZerroGeoIPURL + `"},
    {"type":"geosite","path":"` + hrGeoSite + `","external":true,"url":"` + GroundZerroGeoSiteURL + `"}
  ]
}`
	if err := os.WriteFile(storagePath, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}

	store := NewGeoDataStore(dataDir)
	if len(store.List()) != 2 {
		t.Fatalf("after load reconcile: entries = %+v, want 2 AWGM", store.List())
	}
	for _, e := range store.List() {
		if e.External {
			t.Fatalf("AWGM entry should not be external: %+v", e)
		}
	}

	svc := &Service{}
	svc.SetStatusForTest(true)
	svc.SetGeoDataStore(store)

	if _, err := svc.RescanGeoFiles(); err != nil {
		t.Fatalf("RescanGeoFiles: %v", err)
	}
	if svc.restartTimer != nil {
		svc.restartTimer.Stop()
		svc.restartTimer = nil
	}

	if err := svc.SyncGeoFilesToConfig(); err != nil {
		t.Fatalf("SyncGeoFilesToConfig: %v", err)
	}
	if svc.restartTimer != nil {
		svc.restartTimer.Stop()
	}

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if len(cfg.GeoIPFiles) != 1 || len(cfg.GeoSiteFiles) != 1 {
		t.Fatalf("geo paths in conf: geoip=%v geosite=%v", cfg.GeoIPFiles, cfg.GeoSiteFiles)
	}
	if cfg.GeoIPFiles[0] != awgmGeoIP || cfg.GeoSiteFiles[0] != awgmGeoSite {
		t.Fatalf("want AWGM paths only, got geoip=%q geosite=%q", cfg.GeoIPFiles[0], cfg.GeoSiteFiles[0])
	}

	raw := mustRead(t, hrConfPath)
	if strContains(raw, "geofile") {
		t.Fatalf("hrneo.conf still references HR geofile paths:\n%s", raw)
	}
	for _, must := range []string{
		"GeoIPFile=" + awgmGeoIP,
		"GeoSiteFile=" + awgmGeoSite,
	} {
		if !strContains(raw, must) {
			t.Fatalf("missing %q in hrneo.conf:\n%s", must, raw)
		}
	}

	entries := store.List()
	if len(entries) != 2 {
		t.Fatalf("catalog after sync: %+v", entries)
	}
}

// TestSync_WritesBothAWGMAndHRPathsWhenBothExistOnDisk ensures coexistence:
// two different paths with the same basename both stay in the catalog and hrneo.conf.
func TestSync_WritesBothAWGMAndHRPathsWhenBothExistOnDisk(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "awg-manager")
	awgmGeo := filepath.Join(dataDir, "geo")
	if err := os.MkdirAll(awgmGeo, 0o755); err != nil {
		t.Fatal(err)
	}

	origHR := hrDir
	origConf := hrConfPath
	hrDir = filepath.Join(root, "HydraRoute")
	hrConfPath = filepath.Join(hrDir, "hrneo.conf")
	t.Cleanup(func() {
		hrDir = origHR
		hrConfPath = origConf
	})

	awgmGeoIP := filepath.Join(awgmGeo, "geoip_GA.dat")
	hrGeoIP := filepath.Join(hrDir, "geofile", "geoip_GA.dat")
	if err := os.MkdirAll(filepath.Dir(hrGeoIP), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{awgmGeoIP, hrGeoIP} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	store := NewGeoDataStore(dataDir)
	store.mu.Lock()
	store.entries = []GeoFileEntry{
		{Type: "geoip", Path: awgmGeoIP, URL: GroundZerroGeoIPURL},
		{Type: "geoip", Path: hrGeoIP, External: true, URL: GroundZerroGeoIPURL},
	}
	store.mu.Unlock()

	svc := &Service{}
	svc.SetStatusForTest(true)
	svc.SetGeoDataStore(store)

	if err := svc.SyncGeoFilesToConfig(); err != nil {
		t.Fatalf("SyncGeoFilesToConfig: %v", err)
	}
	if svc.restartTimer != nil {
		svc.restartTimer.Stop()
	}

	cfg, err := ReadConfig()
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if len(cfg.GeoIPFiles) != 2 {
		t.Fatalf("GeoIPFiles = %v, want both AWGM and HR", cfg.GeoIPFiles)
	}
	got := map[string]bool{cfg.GeoIPFiles[0]: true, cfg.GeoIPFiles[1]: true}
	if !got[awgmGeoIP] || !got[hrGeoIP] {
		t.Fatalf("conf paths = %v, want %q and %q", cfg.GeoIPFiles, awgmGeoIP, hrGeoIP)
	}

	entries := store.List()
	if len(entries) != 2 {
		t.Fatalf("catalog: %+v", entries)
	}
}
