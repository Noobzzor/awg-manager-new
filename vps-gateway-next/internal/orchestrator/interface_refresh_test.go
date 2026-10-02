package orchestrator

import (
	"testing"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/storage"
	"github.com/hoaxisr/awg-manager/internal/tunnel"
)

// On a kernel tunnel's confirmed "running" transition, the orchestrator must
// refresh the NDMS interface cache (issue #328): NDMS iflayerchanged hooks are
// unreliable for OpkgTun, so without a proactive invalidate the cached
// State/IPv4 stays frozen and policy/WAN/all-interface lists show the tunnel
// down forever despite it being up.
func TestUpdateState_RunningKernel_InvalidatesInterfaceCache(t *testing.T) {
	const id = "abc42"
	o := &Orchestrator{state: newState(), bus: events.NewBus(), store: storage.NewAWGTunnelStore(t.TempDir())}
	o.state.tunnels[id] = &tunnelState{ID: id, Name: "vpn", Backend: "kernel", Running: true}

	var got []string
	o.SetInterfaceInvalidator(func(name string) { got = append(got, name) })

	o.updateState(Action{Type: ActionColdStartKernel, Tunnel: id})

	want := tunnel.NewNames(id).NDMSName // "OpkgTun42"
	if len(got) != 1 || got[0] != want {
		t.Fatalf("invalidator calls = %v, want exactly [%q]", got, want)
	}
}

// NativeWG self-invalidates on its own create/start path (and uses a different
// NDMS name), so the orchestrator must NOT invalidate an "OpkgTunN" name for it.
func TestUpdateState_RunningNativeWG_DoesNotInvalidate(t *testing.T) {
	const id = "abc43"
	o := &Orchestrator{state: newState(), bus: events.NewBus(), store: storage.NewAWGTunnelStore(t.TempDir())}
	o.state.tunnels[id] = &tunnelState{ID: id, Name: "vpn", Backend: "nativewg", Running: true}

	var got []string
	o.SetInterfaceInvalidator(func(name string) { got = append(got, name) })

	o.updateState(Action{Type: ActionStartNativeWG, Tunnel: id})

	if len(got) != 0 {
		t.Fatalf("nativewg must not trigger OpkgTun invalidation, got %v", got)
	}
}

// A stop transition must not invalidate (nothing to refresh as up).
func TestUpdateState_StoppedKernel_DoesNotInvalidate(t *testing.T) {
	const id = "abc44"
	o := &Orchestrator{state: newState(), bus: events.NewBus(), store: storage.NewAWGTunnelStore(t.TempDir())}
	o.state.tunnels[id] = &tunnelState{ID: id, Name: "vpn", Backend: "kernel", Running: false}

	var got []string
	o.SetInterfaceInvalidator(func(name string) { got = append(got, name) })

	o.updateState(Action{Type: ActionStopKernel, Tunnel: id})

	if len(got) != 0 {
		t.Fatalf("stop must not invalidate, got %v", got)
	}
}
