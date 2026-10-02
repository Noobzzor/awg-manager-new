package forwarding

import (
	"context"
	"fmt"
	"net/netip"
)

type Reconciler struct {
	SetIPv4Forwarding func(context.Context, bool) error
	ApplyNFT          func(context.Context, string) error
}

func (r Reconciler) Apply(ctx context.Context, clientPool netip.Prefix, wanInterface string, interfaceNames ...string) error {
	if r.SetIPv4Forwarding == nil || r.ApplyNFT == nil {
		return fmt.Errorf("forwarding reconciler is not configured")
	}
	script, err := RenderRuleset(clientPool, wanInterface, interfaceNames...)
	if err != nil {
		return err
	}
	if err := r.ApplyNFT(ctx, script); err != nil {
		return fmt.Errorf("apply gateway nftables: %w", err)
	}
	if err := r.SetIPv4Forwarding(ctx, true); err != nil {
		return fmt.Errorf("enable IPv4 forwarding: %w", err)
	}
	return nil
}

func (r Reconciler) ApplyPolicy(ctx context.Context, policy Policy, clientPool netip.Prefix, wanInterface string) error {
	if r.SetIPv4Forwarding == nil || r.ApplyNFT == nil {
		return fmt.Errorf("forwarding reconciler is not configured")
	}
	script, err := RenderPolicyRuleset(policy, clientPool, wanInterface)
	if err != nil {
		return err
	}
	if err := r.ApplyNFT(ctx, script); err != nil {
		return fmt.Errorf("apply gateway policy nftables: %w", err)
	}
	if err := r.SetIPv4Forwarding(ctx, true); err != nil {
		return fmt.Errorf("enable IPv4 forwarding: %w", err)
	}
	return nil
}
