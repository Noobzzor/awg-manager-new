package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/accesspolicy"
	"github.com/hoaxisr/awg-manager/internal/diagnostics"
)

type fakeStatusStore struct {
	raw []byte
	err error
}

func (f fakeStatusStore) List(ctx context.Context) ([]byte, error) { return f.raw, f.err }

type fakePolicyLister struct {
	policies []accesspolicy.Policy
	err      error
}

func (f fakePolicyLister) List(ctx context.Context) ([]accesspolicy.Policy, error) {
	return f.policies, f.err
}

const twoProxySample = `{"proxy-status":[
 {"proxy-name":"System","proxy-config":"dns_tcp_port = 53\ndns_udp_port = 53\n","proxy-stat":"Total incoming requests: 0\n"},
 {"proxy-name":"Policy1","proxy-config":"dns_tcp_port = 41101\n","proxy-stat":"Total incoming requests: 0\n"}
]}`

func decodeProxies(t *testing.T, body []byte) []diagnostics.DNSProxy {
	t.Helper()
	var env struct {
		Success bool `json:"success"`
		Data    struct {
			Proxies []diagnostics.DNSProxy `json:"proxies"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, body)
	}
	if !env.Success {
		t.Fatalf("expected success envelope, got: %s", body)
	}
	return env.Data.Proxies
}

func TestDnsProxyInfo_EnrichesNames(t *testing.T) {
	h := NewDnsProxyInfoHandler(
		fakeStatusStore{raw: []byte(twoProxySample)},
		fakePolicyLister{policies: []accesspolicy.Policy{{Name: "Policy1", Description: "Netflix"}}},
	)
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/diagnostics/dns-proxy", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	proxies := decodeProxies(t, rr.Body.Bytes())
	if len(proxies) != 2 {
		t.Fatalf("want 2 proxies, got %d", len(proxies))
	}
	if proxies[0].DisplayName != "Системный" {
		t.Errorf("System displayName = %q, want Системный", proxies[0].DisplayName)
	}
	if proxies[1].DisplayName != "Netflix" {
		t.Errorf("Policy1 displayName = %q, want Netflix", proxies[1].DisplayName)
	}
}

func TestDnsProxyInfo_FallbackWhenNoDescription(t *testing.T) {
	h := NewDnsProxyInfoHandler(
		fakeStatusStore{raw: []byte(twoProxySample)},
		fakePolicyLister{policies: []accesspolicy.Policy{{Name: "Policy1", Description: ""}}},
	)
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/diagnostics/dns-proxy", nil))
	proxies := decodeProxies(t, rr.Body.Bytes())
	if proxies[1].DisplayName != "Policy1" {
		t.Errorf("empty description should fall back to raw name, got %q", proxies[1].DisplayName)
	}
}

func TestDnsProxyInfo_PolicyListErrorStillServes(t *testing.T) {
	h := NewDnsProxyInfoHandler(
		fakeStatusStore{raw: []byte(twoProxySample)},
		fakePolicyLister{err: context.DeadlineExceeded},
	)
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/diagnostics/dns-proxy", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (enrichment failure must not fail the request)", rr.Code)
	}
	proxies := decodeProxies(t, rr.Body.Bytes())
	if proxies[1].DisplayName != "Policy1" {
		t.Errorf("policy-list error: want raw name fallback, got %q", proxies[1].DisplayName)
	}
}

func TestDnsProxyInfo_MethodNotAllowed(t *testing.T) {
	h := NewDnsProxyInfoHandler(fakeStatusStore{raw: []byte(twoProxySample)}, fakePolicyLister{})
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodPost, "/api/diagnostics/dns-proxy", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST status = %d, want 405", rr.Code)
	}
}

func TestDnsProxyInfo_StoreErrorIs500(t *testing.T) {
	h := NewDnsProxyInfoHandler(fakeStatusStore{err: context.DeadlineExceeded}, fakePolicyLister{})
	rr := httptest.NewRecorder()
	h.Get(rr, httptest.NewRequest(http.MethodGet, "/api/diagnostics/dns-proxy", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Errorf("store error status = %d, want 500", rr.Code)
	}
}
