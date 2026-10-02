package orchestrator

import (
	"testing"
	"time"
)

func TestQuiescentUntil_ReturnsSetValue(t *testing.T) {
	o := &Orchestrator{state: newState()}
	want := time.Unix(5000, 0)
	o.state.tunnels["awg0"] = &tunnelState{ID: "awg0", Backend: "nativewg", quiescentUntil: want}

	got := o.QuiescentUntil("awg0")

	if !got.Equal(want) {
		t.Fatalf("QuiescentUntil(awg0) = %v, want %v", got, want)
	}
}

func TestQuiescentUntil_UnknownTunnelReturnsZero(t *testing.T) {
	o := &Orchestrator{state: newState()}

	got := o.QuiescentUntil("missing")

	if !got.IsZero() {
		t.Fatalf("QuiescentUntil(missing) = %v, want zero time", got)
	}
}
