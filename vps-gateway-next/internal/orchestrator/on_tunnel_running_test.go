package orchestrator

import (
	"testing"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

// Переход туннеля в running должен дёрнуть onTunnelRunning с его ID
// (колбэк рестартит hr-neo — fwmark раздаётся только при старте hr-neo, #247).
func TestUpdateState_Running_InvokesOnTunnelRunning(t *testing.T) {
	const id = "abc42"
	o := &Orchestrator{state: newState(), bus: events.NewBus(), store: storage.NewAWGTunnelStore(t.TempDir())}
	o.state.tunnels[id] = &tunnelState{ID: id, Name: "vpn", Backend: "kernel", Running: true}

	var got []string
	o.SetOnTunnelRunning(func(tid string) { got = append(got, tid) })

	o.updateState(Action{Type: ActionColdStartKernel, Tunnel: id})

	if len(got) != 1 || got[0] != id {
		t.Fatalf("onTunnelRunning calls = %v, want exactly [%q]", got, id)
	}
}
