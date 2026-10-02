package api

import (
	"testing"

	"github.com/hoaxisr/awg-manager/internal/tunnel"
)

func TestStateToStatus(t *testing.T) {
	cases := []struct {
		state tunnel.State
		want  string
	}{
		{tunnel.StateNotCreated, "not_created"},
		{tunnel.StateStopped, "stopped"},
		{tunnel.StateStarting, "starting"},
		{tunnel.StateStopping, "stopping"},
		{tunnel.StateRunning, "running"},
		{tunnel.StateBroken, "broken"},
		{tunnel.StateNeedsStart, "needs_start"},
		{tunnel.StateNeedsStop, "needs_stop"},
		{tunnel.StateDisabled, "disabled"},
		{tunnel.StateUnknown, "stopped"},
	}

	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := stateToStatus(tc.state); got != tc.want {
				t.Fatalf("stateToStatus(%v) = %q, want %q", tc.state, got, tc.want)
			}
		})
	}
}
