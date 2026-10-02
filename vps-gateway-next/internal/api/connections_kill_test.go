package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConnectionsKill_ValidatesParams(t *testing.T) {
	h := NewConnectionsHandler(nil) // svc не нужен: Kill не ходит в Service

	cases := []struct {
		name  string
		query string
		want  int
	}{
		{"no params", "", http.StatusBadRequest},
		{"bad protocol", "src=1.1.1.1&dst=2.2.2.2&srcPort=1&dstPort=2&protocol=icmp", http.StatusBadRequest},
		{"bad port", "src=1.1.1.1&dst=2.2.2.2&srcPort=x&dstPort=2&protocol=tcp", http.StatusBadRequest},
		{"port range", "src=1.1.1.1&dst=2.2.2.2&srcPort=70000&dstPort=2&protocol=tcp", http.StatusBadRequest},
		{"bad ip", "src=nope&dst=2.2.2.2&srcPort=1&dstPort=2&protocol=tcp", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodDelete, "/api/connections?"+tc.query, nil)
			w := httptest.NewRecorder()
			h.Kill(w, r)
			if w.Code != tc.want {
				t.Errorf("code = %d, want %d", w.Code, tc.want)
			}
		})
	}
}
