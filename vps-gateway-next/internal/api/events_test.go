package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/events"
)

func TestEventsHandler_ConnectedIncludesInstanceID(t *testing.T) {
	bus := events.NewBus()
	h := NewEventsHandler(bus, "instance-1")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	h.Stream(w, req)

	got := w.Body.String()
	if !strings.Contains(got, "event: connected") {
		t.Fatalf("missing connected event: %q", got)
	}
	if !strings.Contains(got, `data: {"ok":true,"instanceId":"instance-1"}`) {
		t.Fatalf("missing instanceId payload: %q", got)
	}
}
