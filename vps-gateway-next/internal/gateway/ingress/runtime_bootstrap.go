package ingress

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/hoaxisr/awg-manager/internal/gateway/forwarding"
)

type GatewayRuntime struct {
	Interface       ManagedInterface
	Forwarding      forwarding.Reconciler
	PolicyInterface string
}

func (r GatewayRuntime) Bootstrap(ctx context.Context, clientPool netip.Prefix, wanInterface string) error {
	if r.Interface == nil {
		return fmt.Errorf("gateway interface is not configured")
	}
	if err := r.Interface.Preflight(ctx); err != nil {
		return fmt.Errorf("gateway ingress preflight: %w", err)
	}
	policyInterface := r.PolicyInterface
	if policyInterface == "" {
		policyInterface = "awgm0"
	}
	if err := r.Forwarding.Apply(ctx, clientPool, wanInterface, r.Interface.Name(), policyInterface); err != nil {
		return err
	}
	if err := r.Interface.Ensure(ctx); err != nil {
		return err
	}
	return nil
}
