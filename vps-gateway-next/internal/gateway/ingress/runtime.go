package ingress

import (
	"context"
	"fmt"

	"github.com/hoaxisr/awg-manager/internal/sys/exec"
)

type CommandRunner func(ctx context.Context, name string, args ...string) (string, error)

type CommandApplier struct {
	binary        string
	interfaceName string
	run           CommandRunner
}

func NewCommandApplier(binary, interfaceName string, run CommandRunner) *CommandApplier {
	return &CommandApplier{binary: binary, interfaceName: interfaceName, run: run}
}

func NewAWGCommandApplier(interfaceName string) *CommandApplier {
	return NewLinuxCommandApplier("/opt/sbin/awg", interfaceName)
}

func NewLinuxCommandApplier(binary, interfaceName string) *CommandApplier {
	return NewCommandApplier(binary, interfaceName, func(ctx context.Context, name string, args ...string) (string, error) {
		result, err := exec.Run(ctx, name, args...)
		if err != nil {
			if result != nil {
				return result.Stderr, err
			}
			return "", err
		}
		return result.Stdout, nil
	})
}

func (a *CommandApplier) ApplyPeer(ctx context.Context, peer PeerConfig) error {
	if a == nil || a.run == nil || a.binary == "" || a.interfaceName == "" {
		return fmt.Errorf("awg command applier is not configured")
	}
	if peer.PublicKey == "" || !peer.AllowedIP.IsValid() || !peer.AllowedIP.Addr().Is4() || peer.AllowedIP.Bits() != 32 {
		return fmt.Errorf("invalid peer config")
	}
	args := []string{"set", a.interfaceName, "peer", peer.PublicKey}
	if peer.presharedKey != "" {
		args = append(args, "preshared-key", peer.presharedKey)
	}
	args = append(args, "allowed-ips", peer.AllowedIP.String())
	_, err := a.run(ctx, a.binary, args...)
	if err != nil {
		return fmt.Errorf("apply AWG peer: %w", err)
	}
	return nil
}

func (a *CommandApplier) RemovePeer(ctx context.Context, publicKey string) error {
	if a == nil || a.run == nil || a.binary == "" || a.interfaceName == "" {
		return fmt.Errorf("awg command applier is not configured")
	}
	if publicKey == "" {
		return fmt.Errorf("public key is required")
	}
	_, err := a.run(ctx, a.binary, "set", a.interfaceName, "peer", publicKey, "remove")
	if err != nil {
		return fmt.Errorf("remove AWG peer: %w", err)
	}
	return nil
}
