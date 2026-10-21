package ingress

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/gateway/forwarding"
)

// This is a contract double, not an AWG process or packet fixture.
type preflightContractInterface struct {
	err error
	order *[]string
}
func (m *preflightContractInterface) Name() string { return "awg0" }
func (m *preflightContractInterface) Preflight(context.Context) error { *m.order = append(*m.order, "preflight"); return m.err }
func (m *preflightContractInterface) Ensure(context.Context) error { *m.order = append(*m.order, "ensure"); return nil }
func (m *preflightContractInterface) Close(context.Context) error { *m.order = append(*m.order, "close"); return nil }

func TestGatewayPreflightFailurePreventsForwardingAndInterfaceEnsure(t *testing.T) {
	want := errors.New("incompatible userspace AWG runtime")
	order := []string{}
	iface := &preflightContractInterface{err: want, order: &order}
	r := GatewayRuntime{Interface: iface, Forwarding: forwarding.Reconciler{
		SetIPv4Forwarding: func(context.Context, bool) error { t.Fatal("sysctl after failed preflight"); return nil },
		ApplyNFT: func(context.Context, string) error { t.Fatal("nft after failed preflight"); return nil },
	}}
	if err := r.Bootstrap(context.Background(), netip.MustParsePrefix("10.66.0.0/24"), "eth0"); !errors.Is(err, want) {
		t.Fatalf("preflight failure not preserved: %v", err)
	}
	if len(order) != 1 || order[0] != "preflight" { t.Fatalf("preflight failure leaked later phases: %v", order) }
}
