package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeClashServer returns an httptest.Server that responds to a single
// GET /proxies with the provided body. Caller closes it.
func fakeClashServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/proxies") || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
}

func TestSingboxProxiesHandler_List_FiltersToCompositeGroups(t *testing.T) {
	upstream := fakeClashServer(t, `{
        "proxies": {
            "veesp-fast": {"name":"veesp-fast","type":"Selector","now":"vless-1","all":["vless-1","vless-2"],"history":[]},
            "auto":      {"name":"auto","type":"URLTest","now":"vless-2","all":["vless-1","vless-2"],"history":[]},
            "vless-1":   {"name":"vless-1","type":"VLESS","history":[{"delay":45}]},
            "vless-2":   {"name":"vless-2","type":"VLESS","history":[{"delay":78}]},
            "GLOBAL":    {"name":"GLOBAL","type":"Selector","now":"auto","all":["auto","veesp-fast","vless-1","vless-2"]},
            "DIRECT":    {"name":"DIRECT","type":"Direct"}
        }
    }`)
	t.Cleanup(upstream.Close)

	known := map[string]struct{}{"veesp-fast": {}, "auto": {}}
	h := &SingboxProxiesHandler{
		clashBaseURL:    func() string { return upstream.URL },
		knownComposites: func() map[string]struct{} { return known },
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/singbox/router/proxies/list", nil)
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Success bool `json:"success"`
		Data    struct {
			Groups []SingboxProxyGroup `json:"groups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Success || len(env.Data.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d: %s", len(env.Data.Groups), rec.Body.String())
	}
	tags := map[string]bool{env.Data.Groups[0].Tag: true, env.Data.Groups[1].Tag: true}
	if !tags["veesp-fast"] || !tags["auto"] {
		t.Errorf("expected veesp-fast and auto, got %v", tags)
	}
	for _, g := range env.Data.Groups {
		if g.Tag == "veesp-fast" {
			if g.Type != "selector" || g.Now != "vless-1" {
				t.Errorf("veesp-fast: %+v", g)
			}
			if len(g.Members) != 2 || g.Members[0].LastDelay != 45 {
				t.Errorf("veesp-fast members: %+v", g.Members)
			}
		}
	}
}

func TestSingboxProxiesHandler_Select_HappyPath(t *testing.T) {
	var captured struct {
		method string
		path   string
		body   string
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		captured.body = string(body)
		// /proxies (GET) for group lookup
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/proxies") {
			_, _ = w.Write([]byte(`{"proxies":{"veesp-fast":{"name":"veesp-fast","type":"Selector","now":"vless-1","all":["vless-1","vless-2"]}}}`))
			return
		}
		// /proxies/<group> (PUT) for member switch
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(upstream.Close)

	known := map[string]struct{}{"veesp-fast": {}}
	h := &SingboxProxiesHandler{
		clashBaseURL:    func() string { return upstream.URL },
		knownComposites: func() map[string]struct{} { return known },
		httpClient:      upstream.Client(),
	}
	body := strings.NewReader(`{"group":"veesp-fast","member":"vless-2"}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/singbox/router/proxies/select", body)
	h.Select(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if captured.method != http.MethodPut {
		t.Errorf("expected upstream PUT, got %s", captured.method)
	}
	if !strings.Contains(captured.body, `"vless-2"`) {
		t.Errorf("upstream body missing member: %s", captured.body)
	}
}

func TestSingboxProxiesHandler_Select_GroupNotSelector(t *testing.T) {
	upstream := fakeClashServer(t, `{"proxies":{"auto":{"name":"auto","type":"URLTest","now":"vless-1","all":["vless-1"]}}}`)
	t.Cleanup(upstream.Close)
	known := map[string]struct{}{"auto": {}}
	h := &SingboxProxiesHandler{
		clashBaseURL:    func() string { return upstream.URL },
		knownComposites: func() map[string]struct{} { return known },
		httpClient:      upstream.Client(),
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/singbox/router/proxies/select",
		strings.NewReader(`{"group":"auto","member":"vless-1"}`))
	h.Select(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "GROUP_NOT_SELECTOR") {
		t.Errorf("expected GROUP_NOT_SELECTOR code, got %s", rec.Body.String())
	}
}

func TestSingboxProxiesHandler_Test_HappyPath(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// /group/<name>/delay
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/group/") && strings.HasSuffix(r.URL.Path, "/delay") {
			_, _ = w.Write([]byte(`{"vless-1":45,"vless-2":78,"vless-3":0}`))
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(upstream.Close)

	known := map[string]struct{}{"auto": {}}
	h := &SingboxProxiesHandler{
		clashBaseURL:    func() string { return upstream.URL },
		knownComposites: func() map[string]struct{} { return known },
		httpClient:      upstream.Client(),
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/singbox/router/proxies/test",
		strings.NewReader(`{"group":"auto"}`))
	h.Test(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Success bool `json:"success"`
		Data    struct {
			Delays map[string]int `json:"delays"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data.Delays["vless-1"] != 45 || env.Data.Delays["vless-3"] != 0 {
		t.Errorf("unexpected delays: %+v", env.Data.Delays)
	}
}
