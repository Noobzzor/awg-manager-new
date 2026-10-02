package ingress

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/hoaxisr/awg-manager/internal/gateway/forwarding"
)

type GatewayRuntime struct {
	Interface       *InterfaceManager
	Forwarding      forwarding.Reconciler
	PolicyInterface string
}

func (r GatewayRuntime) Bootstrap(ctx context.Context, clientPool netip.Prefix, wanInterface string) error {
	if r.Interface == nil {
		return fmt.Errorf("gateway interface is not configured")
	}
	policyInterface := r.PolicyInterface
	if policyInterface == "" {
		policyInterface = "awgm0"
	}
	if err := r.Forwarding.Apply(ctx, clientPool, wanInterface, r.Interface.name, policyInterface); err != nil {
		return err
	}
	if err := r.Interface.Ensure(ctx); err != nil {
		return err
	}
	return nil
}
