package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDownloadHandler_ListOutbounds_DefaultDirect(t *testing.T) {
	h := NewDownloadHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/download/outbounds", nil)
	rr := httptest.NewRecorder()

	h.ListOutbounds(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `"tag":"direct"`) {
		t.Fatalf("expected direct tag in response: %s", body)
	}
	if !strings.Contains(body, `"available":true`) {
		t.Fatalf("expected direct available=true in response: %s", body)
	}
}
