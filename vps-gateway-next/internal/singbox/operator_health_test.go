package singbox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOperatorResponsivenessRejectsMissingCanceledAndInvalid(t *testing.T) {
	if (&Operator{}).IsResponsive(context.Background()) {
		t.Fatal("missing health client accepted")
	}
	for _, payload := range []string{`{}`, `{"version":""}`, `invalid`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(payload)) }))
		op := &Operator{clash: NewClashClient(strings.TrimPrefix(server.URL, "http://"))}
		if op.IsResponsive(context.Background()) {
			t.Fatal("invalid health payload accepted")
		}
		server.Close()
	}
}
func TestOperatorResponsivenessRecoversAndHonorsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"version":"sing-box test"}`)) }))
	defer server.Close()
	op := &Operator{clash: NewClashClient(strings.TrimPrefix(server.URL, "http://"))}
	if !op.IsResponsive(context.Background()) {
		t.Fatal("responsive version denied")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if op.IsResponsive(ctx) {
		t.Fatal("canceled request accepted")
	}
}
func TestOperatorResponsivenessDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	op := &Operator{clash: NewClashClient(strings.TrimPrefix(server.URL, "http://"))}
	begun := time.Now()
	if op.IsResponsive(context.Background()) {
		t.Fatal("hung API accepted")
	}
	if time.Since(begun) > 2*time.Second {
		t.Fatal("health probe exceeded bounded deadline")
	}
}
