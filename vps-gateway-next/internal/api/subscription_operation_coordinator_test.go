package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
)

func TestSubscriptionUpdateCoordinatorUsesRequestContext(t *testing.T) {
	svc, id := seedSubscription(t, 1)
	before, err := svc.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	gate := &heavyop.Gate{}
	svc.SetOperationCoordinator(gate)
	gate.Lock()
	held := true
	defer func() {
		if held {
			gate.Unlock()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	h := NewSubscriptionHandler(svc, nil)
	w := httptest.NewRecorder()
	done := make(chan struct{})
	request := httptest.NewRequest(http.MethodPut, "/api/singbox/subscriptions/update?id="+id, strings.NewReader(`{"label":"queued"}`)).WithContext(ctx)
	go func() { defer close(done); h.Update(w, request) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		gate.Unlock()
		held = false
		<-done
		t.Fatal("HTTP update ignored request cancellation while queued")
	}
	if w.Code != http.StatusRequestTimeout {
		t.Fatalf("expected cancelled queue response, got%d %s", w.Code, w.Body.String())
	}
	after, err := svc.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if before.Label != after.Label {
		t.Fatal("cancelled HTTP update persisted a new label")
	}
}

func TestSubscriptionCoordinatorCancellationErrorContract(t *testing.T) {
	for _, name := range []string{"subscription", "group"} {
		t.Run(name, func(t *testing.T) {
			for _, err := range []error{context.Canceled, fmt.Errorf("queue: %w", context.DeadlineExceeded)} {
				h := NewSubscriptionHandler(nil, nil)
				w := httptest.NewRecorder()
				if name == "group" {
					h.respondGroupServiceError(w, "queued", err)
				} else {
					h.respondServiceError(w, "queued", err)
				}
				if w.Code != http.StatusRequestTimeout || !strings.Contains(w.Body.String(), "REQUEST_CANCELLED") {
					t.Fatalf("cancellation reported as server failure:%d %s", w.Code, w.Body.String())
				}
			}
		})
	}
}
