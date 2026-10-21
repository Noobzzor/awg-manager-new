package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
	gatewaywarp "github.com/hoaxisr/awg-manager/internal/gateway/warp"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type warpHTTPBackend struct {
	calls int
	err   error
	p     gatewaywarp.Profile
}

func (b *warpHTTPBackend) Import(p gatewaywarp.Profile) error {
	b.calls++
	if b.err != nil {
		return b.err
	}
	b.p = p
	return nil
}
func (b *warpHTTPBackend) Describe() (gatewaywarp.Description, error) {
	if b.err != nil {
		return gatewaywarp.Description{}, b.err
	}
	if b.calls == 0 {
		return gatewaywarp.Description{}, nil
	}
	return b.p.Description(), nil
}
func warpHTTPProfile() gatewaywarp.Profile {
	return gatewaywarp.Profile{Endpoint: "162.159.193.10:2408", IPv4: "172.16.0.2", MTU: 1280, PrivateKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32)), PublicKey: base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{2}, 32)), Reserved: []int{0, 127, 255}}
}
func warpHTTP(t *testing.T, b gatewayWARPBackend) *gatewayClientHTTP {
	t.Helper()
	a, e := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	if e != nil {
		t.Fatal(e)
	}
	h := newGatewayClientHTTP(gatewayingress.NewRegistry(a, nil), t.TempDir()+"/clients.json")
	h.SetGatewayWARP(b)
	return h
}
func warpRequest(h http.Handler, m, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(m, path, strings.NewReader(body)))
	return w
}
func TestWARPHTTPImportDescriptionAndRedaction(t *testing.T) {
	b := &warpHTTPBackend{}
	h := warpHTTP(t, b)
	p := warpHTTPProfile()
	raw, _ := json.Marshal(p)
	r := warpRequest(h, "POST", "/api/gateway/warp/import", string(raw))
	if r.Code != 200 || b.calls != 1 || !strings.Contains(r.Body.String(), `"configured":true`) {
		t.Fatalf("import status %d", r.Code)
	}
	for _, m := range []string{"GET", "POST"} {
		path := "/api/gateway/warp"
		body := ""
		if m == "POST" {
			path += "/import"
			body = string(raw)
		}
		r = warpRequest(h, m, path, body)
		for _, secret := range []string{p.PrivateKey, p.PublicKey, "reserved", "privateKey", "publicKey", "connected"} {
			if strings.Contains(r.Body.String(), secret) {
				t.Fatal("response leaks secret or claims connectivity")
			}
		}
	}
	b.err = errors.New("SECRET_BACKEND")
	for _, path := range []string{"/api/gateway/warp", "/api/gateway/warp/import"} {
		m := "GET"
		if strings.HasSuffix(path, "import") {
			m = "POST"
		}
		r = warpRequest(h, m, path, string(raw))
		if r.Code != 500 || strings.Contains(r.Body.String(), "SECRET_BACKEND") {
			t.Fatal("backend failure not sanitized")
		}
	}
}
func TestWARPHTTPRejectsMalformedMethodsAndPaths(t *testing.T) {
	b := &warpHTTPBackend{}
	h := warpHTTP(t, b)
	raw, _ := json.Marshal(warpHTTPProfile())
	for _, c := range []struct {
		m, path, body string
		code          int
	}{{"GET", "/api/gateway/warp/import", "", 405}, {"POST", "/api/gateway/warp", "", 405}, {"POST", "/api/gateway/warp/import", "{}", 400}, {"POST", "/api/gateway/warp/import", string(raw) + " {}", 400}, {"POST", "/api/gateway/warp/import", strings.Repeat(" ", gatewaywarp.MaxProfileBytes+1), 400}, {"POST", "/api/gateway/warp/import", `{"privateKey":"SECRET_INPUT"}`, 400}, {"POST", "/api/gateway/warp/unknown", string(raw), 404}, {"POST", "/s/warp/import", string(raw), 404}} {
		t.Run(c.m+c.path+string(rune(c.code)), func(t *testing.T) {
			r := warpRequest(h, c.m, c.path, c.body)
			if r.Code != c.code || strings.Contains(r.Body.String(), "SECRET_INPUT") {
				t.Fatalf("status=%d want=%d", r.Code, c.code)
			}
		})
	}
	if b.calls != 0 {
		t.Fatal("invalid request called backend")
	}
	r := warpRequest(warpHTTP(t, nil), "GET", "/api/gateway/warp", "")
	if r.Code != 503 {
		t.Fatal("unavailable dependency not rejected")
	}
}
func TestWARPHTTPRealStoreSlotAndStartupRestore(t *testing.T) {
	dir := t.TempDir()
	orch := orchestrator.NewWithAppliedPath(filepath.Join(dir, "config"), nil, filepath.Join(dir, "applied.json"))
	t.Cleanup(orch.Close)
	for _, meta := range orchestrator.KnownSlots() {
		if err := orch.Register(meta); err != nil {
			t.Fatal(err)
		}
	}
	if err := orch.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	release := orch.HoldReloads()
	defer release()
	a := &app{dataDir: dir, sbOrch: orch}
	s, err := a.setupGatewayWARP()
	if err != nil {
		t.Fatal(err)
	}
	h := warpHTTP(t, s)
	p := warpHTTPProfile()
	raw, _ := json.Marshal(p)
	r := warpRequest(h, "POST", "/api/gateway/warp/import", string(raw))
	if r.Code != 200 {
		t.Fatalf("real backend import=%d", r.Code)
	}
	stored, err := os.ReadFile(filepath.Join(dir, "gateway-warp.json"))
	if err != nil || bytes.Contains(stored, []byte(p.PrivateKey)) || bytes.Contains(stored, []byte(p.PublicKey)) {
		t.Fatal("stored profile not encrypted")
	}
	slot, enabled, err := orch.SnapshotSlot(orchestrator.SlotWARP)
	if err != nil || !enabled || !bytes.Contains(slot, []byte(p.PrivateKey)) {
		t.Fatal("committed runtime slot missing")
	}
	restored, err := a.setupGatewayWARP()
	if err != nil {
		t.Fatal(err)
	}
	d, err := restored.Describe()
	if err != nil || !d.Configured || d.Endpoint != p.Endpoint {
		t.Fatal("fresh startup helper did not restore imported profile")
	}
	for _, name := range []string{"gateway-warp.json", "config/16-z-warp.json"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("imported secret file not private")
		}
	}
}
