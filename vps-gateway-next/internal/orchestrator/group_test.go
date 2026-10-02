package orchestrator

import "testing"

func TestGroupContiguousByTunnel_NonContiguousSplitsIntoMultipleGroups(t *testing.T) {
	// Documents the known limitation: interleaved per-tunnel actions (as
	// decideWANDown can emit) produce a separate group each time the Tunnel
	// value changes — so tunnel "a" here yields two groups, not one.
	actions := []Action{
		{Type: ActionSuspendProxy, Tunnel: "a"},
		{Type: ActionSuspendProxy, Tunnel: "b"},
		{Type: ActionStartNativeWG, Tunnel: "a"},
		{Type: ActionStartNativeWG, Tunnel: "b"},
	}

	groups := groupContiguousByTunnel(actions)

	if len(groups) != 4 {
		t.Fatalf("non-contiguous input must split per change of Tunnel: want 4 groups, got %d", len(groups))
	}
}

func TestGroupContiguousByTunnel(t *testing.T) {
	actions := []Action{
		{Type: ActionReconcileNativeWG, Tunnel: "a"},
		{Type: ActionApplyStaticRoutes, Tunnel: "a"},
		{Type: ActionReconcileNativeWG, Tunnel: "b"},
		{Type: ActionReconcileStaticRoutes, Tunnel: ""},
		{Type: ActionReconcileDNSRoutes, Tunnel: ""},
	}

	groups := groupContiguousByTunnel(actions)

	if len(groups) != 3 {
		t.Fatalf("want 3 groups, got %d", len(groups))
	}
	if len(groups[0]) != 2 || groups[0][0].Tunnel != "a" {
		t.Errorf("group 0 should be the two 'a' actions, got %+v", groups[0])
	}
	if len(groups[1]) != 1 || groups[1][0].Tunnel != "b" {
		t.Errorf("group 1 should be the single 'b' action, got %+v", groups[1])
	}
	if len(groups[2]) != 2 || groups[2][0].Tunnel != "" {
		t.Errorf("group 2 should be the two tunnel-less route actions, got %+v", groups[2])
	}
}
