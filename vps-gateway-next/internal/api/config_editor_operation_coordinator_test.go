package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

func TestConfigEditorOperationCoordinatorCancelledQueuePreservesSlotAndDraft(t *testing.T) {
	for _, action := range []string{"put", "check", "apply", "discard", "enable"} {
		t.Run(action, func(t *testing.T) {
			h, o, _ := newEditorHandler(t)
			if _, err := o.SaveAndValidate(orchestrator.SlotUser, []byte(`{}`)); err != nil {
				t.Fatal(err)
			}
			if err := o.SetEnabled(orchestrator.SlotUser, false); err != nil {
				t.Fatal(err)
			}
			draft := []byte(`{"route":{}}`)
			if err := o.SaveDraft(orchestrator.SlotUser, draft); err != nil {
				t.Fatal(err)
			}
			prior, enabled, err := o.SnapshotSlot(orchestrator.SlotUser)
			if err != nil {
				t.Fatal(err)
			}
			gate := &heavyop.Gate{}
			h.SetOperationCoordinator(gate)
			gate.Lock()
			defer gate.Unlock()
			body := ""
			method := http.MethodPost
			handler := h.ApplyUserConfig
			switch action {
			case "put":
				body = `{"dns":{}}`
				method = http.MethodPut
				handler = h.PutUserConfig
			case "check":
				handler = h.CheckUserConfig
			case "discard":
				handler = h.DiscardUserConfig
			case "enable":
				body = `{"enabled":true}`
				handler = h.EnableUserConfig
			}
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			req := httptest.NewRequest(method, "/api/singbox/config/user/"+action, strings.NewReader(body)).WithContext(ctx)
			w := httptest.NewRecorder()
			handler(w, req)
			if w.Code != http.StatusRequestTimeout {
				t.Fatalf("expert operation escaped writer coordinator: %d %s", w.Code, w.Body.String())
			}
			after, afterEnabled, err := o.SnapshotSlot(orchestrator.SlotUser)
			if err != nil {
				t.Fatal(err)
			}
			afterDraft, err := o.LoadDraft(orchestrator.SlotUser)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(prior, after) || enabled != afterEnabled || !bytes.Equal(draft, afterDraft) {
				t.Fatal("cancelled expert operation changed slot, enable state or draft")
			}
		})
	}
}

type coordinatedEditorValidator struct {
	gate *heavyop.Gate
	held bool
}

func (v *coordinatedEditorValidator) Validate(context.Context, string) error {
	if v.gate.TryLock() {
		v.gate.Unlock()
	} else {
		v.held = true
	}
	return nil
}
func TestConfigEditorOperationCoordinatorRetainedThroughNativeValidation(t *testing.T) {
	h, o, _ := newEditorHandler(t)
	gate := &heavyop.Gate{}
	h.SetOperationCoordinator(gate)
	if err := o.SaveDraft(orchestrator.SlotUser, []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	v := &coordinatedEditorValidator{gate: gate}
	o.SetValidator(v)
	w := httptest.NewRecorder()
	h.ApplyUserConfig(w, httptest.NewRequest(http.MethodPost, "/api/singbox/config/user/apply", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("apply failed: %d %s", w.Code, w.Body.String())
	}
	if !v.held {
		t.Fatal("expert apply released writer coordinator before native validation")
	}
	if !gate.TryLock() {
		t.Fatal("completed expert apply retained writer coordinator")
	}
	gate.Unlock()
}

func TestConfigEditorOperationCoordinatorReadsRequestBeforeLock(t *testing.T) {
	for _, action := range []string{"put", "check", "enable"} {
		t.Run(action, func(t *testing.T) {
			h, _, _ := newEditorHandler(t)
			gate := &heavyop.Gate{}
			h.SetOperationCoordinator(gate)
			method := http.MethodPost
			body := `{}`
			handler := h.CheckUserConfig
			switch action {
			case "put":
				method = http.MethodPut
				handler = h.PutUserConfig
			case "enable":
				body = `{"enabled":true}`
				handler = h.EnableUserConfig
			}
			probe := &coordinatorProbeReader{Reader: strings.NewReader(body), gate: gate}
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest(method, "/api/singbox/config/user/"+action, probe))
			if w.Code != http.StatusOK {
				t.Fatalf("request failed: %d %s", w.Code, w.Body.String())
			}
			if !probe.read || probe.blocked {
				t.Fatal("request body was read while holding shared writer coordinator")
			}
		})
	}
}

func TestConfigEditorOperationCoordinatorKeepsPreviewsReadable(t *testing.T) {
	for _, action := range []string{"list", "slot"} {
		t.Run(action, func(t *testing.T) {
			h, _, _ := newEditorHandler(t)
			gate := &heavyop.Gate{}
			h.SetOperationCoordinator(gate)
			gate.Lock()
			defer gate.Unlock()
			handler := h.ListSlots
			path := "/api/singbox/config/slots"
			if action == "slot" {
				handler = h.GetSlot
				path = "/api/singbox/config/slot?name=user"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx))
			if w.Code != http.StatusOK {
				t.Fatalf("preview blocked behind writer: %d %s", w.Code, w.Body.String())
			}
		})
	}
}
