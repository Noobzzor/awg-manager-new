package server

import (
	"context"
	"github.com/hoaxisr/awg-manager/internal/singbox"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type readinessProbeProvider struct {
	localStatusProvider
	responsive bool
	calls      int
}

func (p *readinessProbeProvider) IsResponsive(ctx context.Context) bool {
	p.calls++
	return p.responsive && ctx.Err() == nil
}

type readinessNoProbe struct{ state singbox.Status }

func (p readinessNoProbe) GetStatus(context.Context) singbox.Status { return p.state }
func (readinessNoProbe) HasActiveWork() bool                        { return true }
func TestReadinessRejectsUnresponsiveRunningEngine(t *testing.T) {
	p := &readinessProbeProvider{localStatusProvider: localStatusProvider{status: singbox.Status{Installed: true, Running: true}, activeWork: true}}
	h := newLocalTestHandlerWithProvider(t, false, p)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), `"reason":"unresponsive"`) {
		t.Fatalf("unresponsive active engine readyz=%d %s", rr.Code, rr.Body.String())
	}
	p.responsive = true
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/readyz", nil))
	if rr.Code != 200 || p.calls != 2 {
		t.Fatalf("responsive recovery missing: %d calls%d", rr.Code, p.calls)
	}
}
func TestReadinessRequiresProbeForActiveWork(t *testing.T) {
	h := newLocalTestHandlerWithProvider(t, false, readinessNoProbe{singbox.Status{Installed: true, Running: true}})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/readyz", nil))
	if rr.Code != 503 || !strings.Contains(rr.Body.String(), `"reason":"health_unavailable"`) {
		t.Fatalf("missing probe accepted: %d %s", rr.Code, rr.Body.String())
	}
}
func TestIdleReadinessDoesNotProbe(t *testing.T) {
	p := &readinessProbeProvider{localStatusProvider: localStatusProvider{status: singbox.Status{Installed: true}}}
	h := newLocalTestHandlerWithProvider(t, false, p)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/readyz", nil))
	if rr.Code != 200 || p.calls != 0 {
		t.Fatal("idle requires unneeded engine probe")
	}
}

func (p localStatusProvider) IsResponsive(context.Context) bool { return true }
