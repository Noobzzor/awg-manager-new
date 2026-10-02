package server

import (
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/hoaxisr/awg-manager/internal/api"
	"github.com/hoaxisr/awg-manager/internal/auth"
	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
	"github.com/hoaxisr/awg-manager/internal/dnsroute"
	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/singbox"
	singboxorch "github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

type localAWG3Service struct{}

func (localAWG3Service) Apply([]awg3endpoint.Record) error   { return nil }
func (localAWG3Service) SnapshotSlot() ([]byte, bool, error) { return nil, true, nil }
func (localAWG3Service) RestoreSlot([]byte, bool) error      { return nil }

type localStatusProvider struct {
	status     singbox.Status
	activeWork bool
}

func (p localStatusProvider) GetStatus(context.Context) singbox.Status { return p.status }
func (p localStatusProvider) HasActiveWork() bool                      { return p.activeWork }

func newLocalTestHandler(t *testing.T, authEnabled bool, status singbox.Status) http.Handler {
	return newLocalTestHandlerWithProvider(t, authEnabled, localStatusProvider{status: status})
}

func newLocalTestHandlerWithProvider(t *testing.T, authEnabled bool, provider SingboxStatusProvider, gateway ...http.Handler) http.Handler {
	t.Helper()
	settings := storage.NewSettingsStore(t.TempDir())
	if _, err := settings.Load(); err != nil {
		t.Fatal(err)
	}
	if err := settings.Update(func(s *storage.Settings) error {
		s.AuthEnabled = authEnabled
		s.ApiKey = "local-test-key"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessionStore(settings.GetSessionTTL)
	t.Cleanup(sessions.Stop)
	awgStore := awg3endpoint.NewStore(t.TempDir() + "/awg3.json")
	awg3 := api.NewAwg3Handler(awgStore, localAWG3Service{}, nil, nil)
	runtimeDir := t.TempDir()
	op := singbox.NewOperator(singbox.OperatorDeps{
		Dir:              runtimeDir,
		ConfigDir:        runtimeDir + "/config.d",
		Binary:           runtimeDir + "/sing-box",
		DisableNDMSProxy: true,
	})
	bus := events.NewBus()
	singboxHandler := api.NewSingboxHandler(op, bus, nil, nil)
	settingsHandler := api.NewSettingsHandler(settings, nil)
	settingsHandler.SetEventBus(bus)
	dnsStore := dnsroute.NewStore(t.TempDir())
	if _, err := dnsStore.Load(); err != nil {
		t.Fatal(err)
	}
	dnsHandler := api.NewDNSRouteHandler(dnsroute.NewService(dnsStore, nil, nil, nil, nil), nil)
	dnsHandler.SetEventBus(bus)
	var gatewayHandler http.Handler
	if len(gateway) > 0 {
		gatewayHandler = gateway[0]
	}

	frontendFS := fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte("<html>local frontend</html>")},
		"assets/app.js": &fstest.MapFile{Data: []byte("portable-api")},
	}
	s := NewLocal(Config{Version: "test-version", FrontendFS: fs.FS(frontendFS)}, LocalDeps{
		Common: CommonCapabilities{
			Settings:        settings,
			Sessions:        sessions,
			Bus:             bus,
			SettingsHandler: settingsHandler,
		},
		Singbox: SingboxCapabilities{
			Status:  provider,
			Handler: singboxHandler,
			StatusHandler: func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"success":true,"data":{"running":false}}`))
			},
			Awg3Handler: awg3,
		},
		Routing: RoutingCapabilities{
			Gateway:   gatewayHandler,
			DNSRoutes: dnsHandler, DNSRouteBackend: "singbox",
			Tunnels: func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"success":true,"data":[{"id":"ep-1","name":"awg3"}]}`))
			},
			Refresh: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					response.MethodNotAllowed(w)
					return
				}
				_, _ = w.Write([]byte(`{"success":true,"data":{"missing":[]}}`))
			},
		},
	})
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	return s.loggingMiddleware(mux)
}

func TestLocalSubscriptionRouteIsPublicButAdminRouteRequiresAuth(t *testing.T) {
	gateway := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(r.URL.Path))
	})
	h := newLocalTestHandlerWithProvider(t, true, localStatusProvider{}, gateway)

	public := httptest.NewRecorder()
	h.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/s/test-capability-token", nil))
	if public.Code != http.StatusAccepted || public.Body.String() != "/s/test-capability-token" {
		t.Fatalf("public subscription route = %d %q", public.Code, public.Body.String())
	}

	admin := httptest.NewRecorder()
	h.ServeHTTP(admin, httptest.NewRequest(http.MethodGet, "/api/gateway/subscriptions", nil))
	if admin.Code != http.StatusUnauthorized {
		t.Fatalf("admin subscription route without authentication = %d, want %d", admin.Code, http.StatusUnauthorized)
	}
}

func TestLocalRoutes_HealthReadinessFrontendAndAbsentKeenetic(t *testing.T) {
	h := newLocalTestHandler(t, false, singbox.Status{Installed: true, Running: false})
	cases := []struct {
		path     string
		wantCode int
		wantBody string
	}{
		{"/healthz", http.StatusOK, `"status":"ok"`},
		{"/api/health", http.StatusOK, `"instanceId"`},
		{"/api/auth/status", http.StatusOK, `"authDisabled":true`},
		{"/readyz", http.StatusOK, `"running":false`},
		{"/", http.StatusOK, "local frontend"},
		{"/some/client/route", http.StatusOK, "local frontend"},
		{"/api/singbox/status", http.StatusOK, `"success":true`},
		{"/api/awg3-endpoints", http.StatusOK, `"data":[]`},
		{"/api/tunnels/all", http.StatusNotImplemented, `UNSUPPORTED_DOCKER_CAPABILITY`},
		{"/api/servers/all", http.StatusNotImplemented, `UNSUPPORTED_DOCKER_CAPABILITY`},
		{"/api/proxyrt/instances", http.StatusOK, `"instances":[]`},
		{"/api/proxyrt/install/status?subsystem=wdtt", http.StatusOK, `"serverSupported":false`},
		{"/api/proxyrt/install/status?subsystem=freeturn", http.StatusOK, `"serverSupported":false`},
		{"/api/managed/drift", http.StatusNotImplemented, `UNSUPPORTED_DOCKER_CAPABILITY`},
		{"/api/logs", http.StatusNotImplemented, `UNSUPPORTED_DOCKER_CAPABILITY`},
		{"/api/logs/subgroups?group=app", http.StatusNotImplemented, `UNSUPPORTED_DOCKER_CAPABILITY`},
		{"/api/monitoring/matrix", http.StatusNotImplemented, `UNSUPPORTED_DOCKER_CAPABILITY`},
		{"/api/connections", http.StatusNotImplemented, `UNSUPPORTED_DOCKER_CAPABILITY`},
		{"/api/capabilities", http.StatusOK, `"dnsRouteBackend":"singbox"`},
		{"/api/system/info", http.StatusOK, `"activeBackend":"singbox"`},
		{"/api/routing/dns-routes", http.StatusOK, `"data":[]`},
		{"/api/routing/tunnels", http.StatusOK, `"name":"awg3"`},
		{"/api/routing/refresh", http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED"},
		{"/api/system/opkg/installed", http.StatusNotFound, "NOT_FOUND"},
		{"/api/hook/ndms", http.StatusNotFound, "NOT_FOUND"},
		{"/api/servers", http.StatusNotFound, "NOT_FOUND"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rr.Code != tc.wantCode || !strings.Contains(rr.Body.String(), tc.wantBody) {
				t.Fatalf("GET %s = %d %q, want %d containing %q", tc.path, rr.Code, rr.Body.String(), tc.wantCode, tc.wantBody)
			}
		})
	}
}

func TestLocalRoutes_CapabilitiesReflectRegisteredPortableHandlers(t *testing.T) {
	h := newLocalTestHandler(t, false, singbox.Status{Installed: true})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/capabilities", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("capabilities status = %d: %s", rr.Code, rr.Body.String())
	}

	var envelope struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode capabilities: %v; body=%s", err, rr.Body.String())
	}
	if !envelope.Success {
		t.Fatalf("capabilities success=false: %s", rr.Body.String())
	}
	want := map[string]any{
		"singboxStatus":       true,
		"awg3":                true,
		"singboxConfigEditor": false,
		"subscriptions":       false,
		"inbounds":            false,
		"dnsRoutes":           true,
		"dnsRouteBackend":     "singbox",
		"proxyInbound":        false,
		"logs":                false,
		"ndms":                false,
		"singboxRouter":       false,
		"hydraroute":          false,
		"tunnelDiagnostics":   false,
		"updates":             false,
		"daemonRestart":       false,
		"ndmsProxy":           false,
		"gateway":             false,
	}
	for key, expected := range want {
		actual, ok := envelope.Data[key]
		if !ok || actual != expected {
			t.Errorf("capability %q = %#v (present=%v), want %#v", key, actual, ok, expected)
		}
	}
}

func TestLocalRoutes_CapabilitiesDoNotAdvertiseMissingHandlers(t *testing.T) {
	settings := storage.NewSettingsStore(t.TempDir())
	if _, err := settings.Load(); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessionStore(settings.GetSessionTTL)
	t.Cleanup(sessions.Stop)
	s := NewLocal(Config{}, LocalDeps{Common: CommonCapabilities{
		Settings: settings,
		Sessions: sessions,
	}})
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/capabilities", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("capabilities status = %d: %s", rr.Code, rr.Body.String())
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"singboxStatus", "awg3", "singboxConfigEditor", "subscriptions", "inbounds", "dnsRoutes", "proxyInbound", "logs", "ndms", "singboxRouter", "hydraroute", "tunnelDiagnostics", "updates", "daemonRestart", "ndmsProxy", "gateway"} {
		if value, ok := envelope.Data[key]; !ok || value != false {
			t.Errorf("missing handler capability %q = %#v (present=%v), want false", key, value, ok)
		}
	}
	if value := envelope.Data["dnsRouteBackend"]; value != "" {
		t.Errorf("dnsRouteBackend = %#v, want empty without DNS handler", value)
	}
}

func TestLocalRoutes_MountPortableSingboxSurface(t *testing.T) {
	settings := storage.NewSettingsStore(t.TempDir())
	if _, err := settings.Load(); err != nil {
		t.Fatal(err)
	}
	sessions := auth.NewSessionStore(settings.GetSessionTTL)
	t.Cleanup(sessions.Stop)
	runtimeDir := t.TempDir()
	op := singbox.NewOperator(singbox.OperatorDeps{
		Dir:              runtimeDir,
		ConfigDir:        filepath.Join(runtimeDir, "config.d"),
		Binary:           filepath.Join(runtimeDir, "sing-box"),
		DisableNDMSProxy: true,
	})
	orch := singboxorch.NewWithAppliedPath(op.ConfigDir(), op.Process(), filepath.Join(runtimeDir, "applied.json"))
	t.Cleanup(orch.Close)
	for _, meta := range singboxorch.KnownSlots() {
		if err := orch.Register(meta); err != nil {
			t.Fatal(err)
		}
	}
	if err := orch.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	op.SetOrch(orch)

	s := NewLocal(Config{}, LocalDeps{Common: CommonCapabilities{
		Settings: settings,
		Sessions: sessions,
	}})
	s.SetSingboxConfigHandler(api.NewSingboxConfigHandler(orch.ConfigDir))
	s.SetSingboxConfigEditorHandler(api.NewSingboxConfigEditorHandler(orch, nil))
	s.SetSingboxInboundsHandler(api.NewSingboxInboundsHandler(api.SingboxInboundsDeps{
		ConfigDir:        orch.ConfigDir,
		NDMSProxyEnabled: func() bool { return false },
	}))
	s.SetClashProxy(api.NewClashProxy(op))
	s.SetSingboxProxiesHandler(api.NewSingboxProxiesHandler(
		func() string { return "" },
		func() map[string]struct{} { return map[string]struct{}{} },
		nil,
	))
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	for _, tc := range []struct {
		path string
		want int
	}{
		{path: "/api/singbox/config-preview", want: http.StatusOK},
		{path: "/api/singbox/config/slots", want: http.StatusOK},
		{path: "/api/singbox/inbounds", want: http.StatusOK},
		{path: "/api/singbox/router/status", want: http.StatusNotFound},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if rr.Code != tc.want {
				t.Fatalf("GET %s = %d %q, want %d", tc.path, rr.Code, rr.Body.String(), tc.want)
			}
		})
	}
}

func TestLocalRoutes_NoAuthLoginLogoutBootstrapWithoutRouterCredentials(t *testing.T) {
	h := newLocalTestHandler(t, false, singbox.Status{})

	login := httptest.NewRecorder()
	h.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"login":"ignored","password":"must-not-be-checked"}`)))
	if login.Code != http.StatusOK || !strings.Contains(login.Body.String(), `"login":"local"`) {
		t.Fatalf("local no-auth login = %d %q", login.Code, login.Body.String())
	}

	logout := httptest.NewRecorder()
	h.ServeHTTP(logout, httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil))
	if logout.Code != http.StatusOK || !strings.Contains(logout.Body.String(), `"success":true`) {
		t.Fatalf("local logout = %d %q", logout.Code, logout.Body.String())
	}
}

func TestLocalRoutes_ProtectedModeLoginDoesNotFallThroughToKeeneticOrEntware(t *testing.T) {
	h := newLocalTestHandler(t, true, singbox.Status{})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"login":"admin","password":"secret"}`)))
	if rr.Code != http.StatusNotImplemented || !strings.Contains(rr.Body.String(), "LOCAL_LOGIN_UNAVAILABLE") {
		t.Fatalf("protected local login = %d %q, want bounded local-only rejection", rr.Code, rr.Body.String())
	}
}

func TestLocalRoutes_HealthReportsFatalWithoutLeakingDetail(t *testing.T) {
	secret := `generated config FATAL private_key=TOPSECRET`
	h := newLocalTestHandlerWithProvider(t, false, localStatusProvider{
		status:     singbox.Status{Installed: true, Running: true, LastError: secret},
		activeWork: true,
	})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, `"status":"degraded"`) || !strings.Contains(body, `"reason":"fatal"`) {
		t.Fatalf("healthz = %d %q, want liveness 200 with degraded/fatal", rr.Code, body)
	}
	for _, forbidden := range []string{"TOPSECRET", "private_key", "lastError"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("healthz leaked %q: %s", forbidden, body)
		}
	}
}

func TestLocalRoutes_ReadinessMatrix(t *testing.T) {
	tests := []struct {
		name       string
		provider   SingboxStatusProvider
		wantStatus int
		wantReason string
	}{
		{"missing provider", nil, http.StatusServiceUnavailable, "provider_unavailable"},
		{"missing binary", localStatusProvider{status: singbox.Status{}}, http.StatusServiceUnavailable, "binary_missing"},
		{"fatal while idle", localStatusProvider{status: singbox.Status{Installed: true, LastError: "fatal"}}, http.StatusServiceUnavailable, "fatal"},
		{"idle stopped", localStatusProvider{status: singbox.Status{Installed: true}}, http.StatusOK, "idle"},
		{"idle running", localStatusProvider{status: singbox.Status{Installed: true, Running: true}}, http.StatusOK, "idle"},
		{"active stopped", localStatusProvider{status: singbox.Status{Installed: true}, activeWork: true}, http.StatusServiceUnavailable, "not_running"},
		{"active running", localStatusProvider{status: singbox.Status{Installed: true, Running: true}, activeWork: true}, http.StatusOK, "ready"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newLocalTestHandlerWithProvider(t, false, tt.provider)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if rr.Code != tt.wantStatus || !strings.Contains(rr.Body.String(), `"reason":"`+tt.wantReason+`"`) {
				t.Fatalf("readyz = %d %q, want %d reason %q", rr.Code, rr.Body.String(), tt.wantStatus, tt.wantReason)
			}
		})
	}
}

func TestLocalRoutes_ReadinessNeverLeaksFatalStderrOrStatusSecrets(t *testing.T) {
	secret := `FATAL private_key=TOPSECRET header_protection_key=HIDDEN currentSha256=abc requiredSha256=def`
	h := newLocalTestHandler(t, false, singbox.Status{Installed: true, LastError: secret})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	body := rr.Body.String()
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(body, `"reason":"fatal"`) {
		t.Fatalf("status/body = %d %q", rr.Code, body)
	}
	for _, forbidden := range []string{"TOPSECRET", "HIDDEN", "private_key", "header_protection_key", "currentSha256", "requiredSha256", "lastFatal", "lastError"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("readiness leaked %q: %s", forbidden, body)
		}
	}
}

func TestLocalRoutes_PortableInventoryAndExplicitNDMSAbsences(t *testing.T) {
	h := newLocalTestHandler(t, false, singbox.Status{})
	cases := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodGet, "/api/settings/get", http.StatusOK},
		{http.MethodGet, "/api/settings/update", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/settings/regenerate-api-key", http.StatusMethodNotAllowed},
		{http.MethodGet, "/api/singbox/uninstall", http.StatusMethodNotAllowed},
		// The embedded installer has no portable amd64 BinarySpec and uses
		// router disk/opkg migration hooks, so local install/update stay absent.
		{http.MethodPost, "/api/singbox/install", http.StatusNotFound},
		{http.MethodPost, "/api/singbox/update", http.StatusNotFound},
		// These operations are inherently NDMS/Entware-specific.
		{http.MethodGet, "/api/system/opkg/installed", http.StatusNotFound},
		{http.MethodPost, "/api/hook/ndms", http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, httptest.NewRequest(tc.method, tc.path, nil))
			if rr.Code != tc.want {
				t.Fatalf("%s %s = %d %q, want %d", tc.method, tc.path, rr.Code, rr.Body.String(), tc.want)
			}
		})
	}
}

func TestLocalRoutes_PortableAPIsUseAuthGuard(t *testing.T) {
	h := newLocalTestHandler(t, true, singbox.Status{})
	for _, tc := range []struct {
		path       string
		authedCode int
	}{
		{"/api/settings/get", http.StatusOK},
		{"/api/singbox/status", http.StatusOK},
		{"/api/singbox/uninstall", http.StatusMethodNotAllowed},
		{"/api/awg3-endpoints", http.StatusOK},
	} {
		path := tc.path
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated GET %s = %d", path, rr.Code)
		}

		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer local-test-key")
		rr = httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != tc.authedCode {
			t.Fatalf("authenticated GET %s = %d: %s (want %d)", path, rr.Code, rr.Body.String(), tc.authedCode)
		}
	}
}

func TestLocalServer_UsesExactAddressAndShutsDown(t *testing.T) {
	s := NewLocal(Config{FrontendFS: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}}, LocalDeps{})
	s.SetListenAddr("127.0.0.1:0")
	done := make(chan error, 1)
	go func() { done <- s.Start() }()

	deadline := time.Now().Add(3 * time.Second)
	for {
		s.listen.mu.Lock()
		n := len(s.listen.listeners)
		s.listen.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("exact listener did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != http.ErrServerClosed {
			t.Fatalf("Start returned %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after Shutdown")
	}
}

func TestLocalServer_InvalidExactAddressDoesNotFallback(t *testing.T) {
	s := NewLocal(Config{}, LocalDeps{})
	s.SetListenAddr("not a valid address")
	err := s.Start()
	if err == nil || !strings.Contains(err.Error(), "not a valid address") {
		t.Fatalf("Start error = %v, want exact-address bind failure", err)
	}
}
