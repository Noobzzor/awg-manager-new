package forwarding

import (
	"context"
	"errors"
	"net/netip"
	"reflect"
	"testing"
)

func TestReconcilerAppliesForwardingAndNftRuleset(t *testing.T) {
	forwarding := false
	var script string
	r := Reconciler{
		SetIPv4Forwarding: func(_ context.Context, enabled bool) error { forwarding = enabled; return nil },
		ApplyNFT:          func(_ context.Context, got string) error { script = got; return nil },
	}
	if err := r.Apply(context.Background(), netip.MustParsePrefix("10.66.0.0/24"), "eth0"); err != nil {
		t.Fatal(err)
	}
	if !forwarding || script == "" {
		t.Fatalf("forwarding=%v script=%q", forwarding, script)
	}
}

func TestReconcilerInstallsFirewallBeforeEnablingForwarding(t *testing.T) {
	var calls []string
	r := Reconciler{
		SetIPv4Forwarding: func(_ context.Context, enabled bool) error {
			if enabled {
				calls = append(calls, "forwarding-on")
			} else {
				calls = append(calls, "forwarding-off")
			}
			return nil
		},
		ApplyNFT: func(_ context.Context, _ string) error {
			calls = append(calls, "firewall")
			return nil
		},
	}
	if err := r.Apply(context.Background(), netip.MustParsePrefix("10.66.0.0/24"), "eth0"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"firewall", "forwarding-on"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("reconcile order = %v, want %v", calls, want)
	}
}

func TestReconcilerDoesNotEnableForwardingWhenFirewallApplyFails(t *testing.T) {
	wantErr := errors.New("nft batch rejected")
	forwardingEnabled := false
	r := Reconciler{
		SetIPv4Forwarding: func(_ context.Context, enabled bool) error {
			forwardingEnabled = enabled
			return nil
		},
		ApplyNFT: func(_ context.Context, _ string) error { return wantErr },
	}
	if err := r.Apply(context.Background(), netip.MustParsePrefix("10.66.0.0/24"), "eth0"); !errors.Is(err, wantErr) {
		t.Fatalf("Apply error = %v, want wrapped %v", err, wantErr)
	}
	if forwardingEnabled {
		t.Fatal("IPv4 forwarding was enabled despite firewall apply failure")
	}
}

func TestPolicyReconcilerInstallsFirewallBeforeEnablingForwarding(t *testing.T) {
	var calls []string
	policy, err := NewPolicy(netip.MustParseAddr("1.1.1.1"), 1420)
	if err != nil {
		t.Fatal(err)
	}
	r := Reconciler{
		SetIPv4Forwarding: func(_ context.Context, enabled bool) error {
			if enabled {
				calls = append(calls, "forwarding-on")
			}
			return nil
		},
		ApplyNFT: func(_ context.Context, _ string) error {
			calls = append(calls, "firewall")
			return nil
		},
	}
	if err := r.ApplyPolicy(context.Background(), policy, netip.MustParsePrefix("10.66.0.0/24"), "eth0"); err != nil {
		t.Fatal(err)
	}
	if want := []string{"firewall", "forwarding-on"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("policy reconcile order = %v, want %v", calls, want)
	}
}
