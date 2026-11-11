package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/auth"
	"github.com/hoaxisr/awg-manager/internal/presets"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

func localPresetHarness(t *testing.T, catalog *presets.Catalog, authEnabled bool) http.Handler {
	t.Helper()
	settings := storage.NewSettingsStore(t.TempDir())
	if _, err := settings.Load(); err != nil {
		t.Fatal(err)
	}
	if err := settings.Update(func(s *storage.Settings) error {
		s.AuthEnabled = authEnabled
		s.ApiKey = "catalog-fixture-key"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessionStore(settings.GetSessionTTL)
	t.Cleanup(sessions.Stop)
	s := NewLocal(Config{}, LocalDeps{Common: CommonCapabilities{Settings: settings, Sessions: sessions}})
	if catalog != nil {
		s.SetPresetCatalog(catalog)
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	return mux
}

func TestLocalPresetCatalogPreservesOriginalFullCatalogAndIcons(t *testing.T) {
	dir := t.TempDir()
	catalog := presets.NewCatalog(presets.NewStore(dir))
	want, err := catalog.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(want) < 10 {
		t.Fatal("expected the complete original multi-service catalog")
	}
	h := localPresetHarness(t, catalog, false)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/presets", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("original catalog route status=%d, want200", rr.Code)
	}
	var got struct {
		Success bool `json:"success"`
		Data    struct {
			Presets []presets.Preset `json:"presets"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Success || !reflect.DeepEqual(got.Data.Presets, want) {
		t.Fatal("portable route must expose the unfiltered original catalog, including IDs, icons and engine definitions")
	}
	if _, err := os.Stat(filepath.Join(dir, "presets.user.json")); !os.IsNotExist(err) {
		t.Fatal("reading builtin catalog must not create an overlay file")
	}
}

func TestLocalPresetCatalogUsesExistingUserOverlay(t *testing.T) {
	store := presets.NewStore(t.TempDir())
	overlay := &presets.Overlay{DisabledBuiltins: []string{"youtube"}, Presets: []presets.Preset{{ID: "catalog-user-fixture", Name: "User fixture", IconSlug: "test"}}}
	if err := store.Save(overlay); err != nil {
		t.Fatal(err)
	}
	catalog := presets.NewCatalog(store)
	want, err := catalog.List()
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	localPresetHarness(t, catalog, false).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/presets", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("overlay catalog route status=%d, want200", rr.Code)
	}
	var got struct {
		Data struct {
			Presets []presets.Preset `json:"presets"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Data.Presets, want) {
		t.Fatal("existing overlay merge semantics changed")
	}
}

func TestLocalPresetCatalogRetainsAuthenticationAndMethodGuards(t *testing.T) {
	catalog := presets.NewCatalog(presets.NewStore(t.TempDir()))
	h := localPresetHarness(t, catalog, true)
	for _, tc := range []struct {
		name, method, token string
		want                int
	}{
		{"unauthenticated", http.MethodGet, "", http.StatusUnauthorized},
		{"wrong credential", http.MethodGet, "Bearer wrong", http.StatusUnauthorized},
		{"authenticated", http.MethodGet, "Bearer catalog-fixture-key", http.StatusOK},
		{"read only", http.MethodPost, "Bearer catalog-fixture-key", http.StatusMethodNotAllowed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/api/presets", nil)
			if tc.token != "" {
				req.Header.Set("Authorization", tc.token)
			}
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			if rr.Code != tc.want {
				t.Fatalf("catalog status=%d, want%d", rr.Code, tc.want)
			}
		})
	}
}

func TestLocalPresetCatalogAbsentRemainsUnsupported(t *testing.T) {
	rr := httptest.NewRecorder()
	localPresetHarness(t, nil, false).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/presets", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("absent catalog status=%d, want404", rr.Code)
	}
}
