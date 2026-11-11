package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/api"
	"github.com/hoaxisr/awg-manager/internal/auth"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

type portableRouteStatusFixture struct{ router.Service }

func (portableRouteStatusFixture) GetStatus(context.Context) (router.Status, error) {
	return router.Status{Active: false, Final: "direct"}, nil
}

func localRouterRouteHarness(t *testing.T, present bool) http.Handler {
	t.Helper()
	settings := storage.NewSettingsStore(t.TempDir())
	if _, err := settings.Load(); err != nil {
		t.Fatal(err)
	}
	if err := settings.Update(func(s *storage.Settings) error { s.AuthEnabled = true; s.ApiKey = "router-fixture-key"; return nil }); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessionStore(settings.GetSessionTTL)
	t.Cleanup(sessions.Stop)
	s := NewLocal(Config{}, LocalDeps{Common: CommonCapabilities{Settings: settings, Sessions: sessions}})
	if present {
		s.SetSingboxRouterHandler(api.NewSingboxRouterHandler(portableRouteStatusFixture{}, nil))
	}
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	return mux
}

func TestLocalOriginalRouterRoutesConditionalAndGuarded(t *testing.T) {
	absent := localRouterRouteHarness(t, false)
	present := localRouterRouteHarness(t, true)
	// Exercise every editor family using the original handler method guards.
	// TRACE must reach the handler without invoking platform operations.
	for _, path := range []string{
		"status", "mode", "settings", "rules/list", "rules/add", "rules/move",
		"rules/bulk-outbound", "rulesets/list", "rulesets/add", "rulesets/bulk-detour",
		"outbounds/list", "outbounds/add", "presets/list", "presets/apply",
		"policies", "wan-interfaces", "bindable-interfaces", "policy-devices",
		"dns/servers/list", "dns/servers/add", "dns/servers/move",
		"dns/rules/list", "dns/rules/add", "dns/rules/move", "dns/globals", "dns/chain-preset",
		"route/final", "inspect", "inspect-dns", "inspect/stream",
		"staging", "staging/apply", "staging/discard",
	} {
		t.Run(path, func(t *testing.T) {
			url := "/api/singbox/router/" + path
			for _, tc := range []struct {
				h     http.Handler
				token string
				want  int
			}{
				{absent, "", http.StatusNotFound},
				{present, "", http.StatusUnauthorized},
				{present, "Bearer router-fixture-key", http.StatusMethodNotAllowed},
			} {
				req := httptest.NewRequest(http.MethodTrace, url, nil)
				req.Header.Set("Authorization", tc.token)
				rr := httptest.NewRecorder()
				tc.h.ServeHTTP(rr, req)
				if rr.Code != tc.want {
					t.Errorf("route %s status=%d want%d", url, rr.Code, tc.want)
				}
			}
		})
	}
}

func TestLocalOriginalRouterStatusUsesExplicitHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/singbox/router/status", nil)
	req.Header.Set("Authorization", "Bearer router-fixture-key")
	rr := httptest.NewRecorder()
	localRouterRouteHarness(t, true).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d want200", rr.Code)
	}
	var got struct {
		Data router.Status `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Data.Active || got.Data.Final != "direct" {
		t.Fatalf("status did not preserve runtime result: %+v", got.Data)
	}
}
