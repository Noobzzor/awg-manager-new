package ingress

import (
	"context"
	"net/netip"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/gateway/forwarding"
)

func TestGatewayRuntimeBootstrapAppliesFirewallBeforeInterface(t *testing.T) {
	order := []string{}
	manager := NewInterfaceManager("ip", "awg", "awg0", 51820, "private", func(_ context.Context, _ string, args ...string) (string, error) {
		if len(args) > 0 {
			order = append(order, args[0])
		}
		return "", nil
	})
	manager.SetAddress("10.66.0.1/24")
	runtime := GatewayRuntime{Interface: manager, Forwarding: forwarding.Reconciler{
		SetIPv4Forwarding: func(_ context.Context, _ bool) error { order = append(order, "sysctl"); return nil },
		ApplyNFT:          func(_ context.Context, _ string) error { order = append(order, "nft"); return nil },
	}}
	if err := runtime.Bootstrap(context.Background(), netip.MustParsePrefix("10.66.0.0/24"), "eth0"); err != nil {
		t.Fatal(err)
	}
	if len(order) < 6 || order[0] != "nft" || order[1] != "sysctl" || order[2] != "link" || order[3] != "address" || order[4] != "set" || order[5] != "link" {
		t.Fatalf("firewall must be applied before AWG interface setup, order=%v", order)
	}
}

func TestGatewayRuntimeAppliesRestrictiveFirewallBeforeCreatingInterface(t *testing.T) {
	order := []string{}
	manager := NewInterfaceManager("ip", "awg", "awg0", 51820, "private", func(_ context.Context, _ string, args ...string) (string, error) {
		if len(args) > 0 {
			order = append(order, "interface:"+args[0])
		}
		return "", nil
	})
	manager.SetAddress("10.66.0.1/24")
	runtime := GatewayRuntime{Interface: manager, Forwarding: forwarding.Reconciler{
		SetIPv4Forwarding: func(_ context.Context, _ bool) error { order = append(order, "sysctl"); return nil },
		ApplyNFT:          func(_ context.Context, _ string) error { order = append(order, "nft"); return nil },
	}}
	if err := runtime.Bootstrap(context.Background(), netip.MustParsePrefix("10.66.0.0/24"), "eth0"); err != nil {
		t.Fatal(err)
	}
	if len(order) < 3 || order[0] != "nft" || order[1] != "sysctl" || order[len(order)-1] != "interface:link" {
		t.Fatalf("firewall must be installed before AWG is exposed, order=%v", order)
	}
}
