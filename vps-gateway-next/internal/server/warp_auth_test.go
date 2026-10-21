package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalWARPAdminRequiresAuthBeforeDispatch(t *testing.T) {
	calls := 0
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(202) })
	h := newLocalTestHandlerWithProvider(t, true, localStatusProvider{}, backend)
	for _, p := range []string{"/api/gateway/warp", "/api/gateway/warp/import"} {
		m := "GET"
		if p == "/api/gateway/warp/import" {
			m = "POST"
		}
		before := calls
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(m, p, nil))
		if w.Code != 401 || calls != before {
			t.Fatal("unauthenticated WARP request dispatched")
		}
		r := httptest.NewRequest(m, p, nil)
		r.Header.Set("Authorization", "Bearer local-test-key")
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 202 {
			t.Fatalf("authenticated status=%d", w.Code)
		}
	}
	if calls != 2 {
		t.Fatal("unexpected dispatch count")
	}
}
