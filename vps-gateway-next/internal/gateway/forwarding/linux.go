package forwarding

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	systemexec "github.com/hoaxisr/awg-manager/internal/sys/exec"
)

var nftApplyMu sync.Mutex

type nftApplyFunc func(string) (*systemexec.Result, error)
type nftTableExistsFunc func() (*systemexec.Result, error)

var errNFTTableMissing = errors.New("gateway nft table missing")

func NewLinuxReconciler() Reconciler {
	return Reconciler{
		SetIPv4Forwarding: func(ctx context.Context, enabled bool) error {
			value := "0"
			if enabled {
				value = "1"
			}
			result, err := systemexec.Run(ctx, "/usr/sbin/sysctl", "-w", "net.ipv4.ip_forward="+value)
			if err != nil && result != nil && result.Stderr != "" {
				if current, readErr := os.ReadFile("/proc/sys/net/ipv4/ip_forward"); readErr == nil && strings.TrimSpace(string(current)) == value {
					return nil
				}
				return fmt.Errorf("%s: %w", result.Stderr, err)
			}
			return err
		},
		ApplyNFT: func(ctx context.Context, script string) error {
			return applyNFTBatch(script, func(batch string) (*systemexec.Result, error) {
				return systemexec.RunWithOptions(ctx, "/usr/sbin/nft", []string{"-f", "-"}, systemexec.Options{Stdin: strings.NewReader(batch)})
			}, func() (*systemexec.Result, error) {
				result, err := systemexec.Run(ctx, "/usr/sbin/nft", "list", "table", "inet", "awgm_gateway")
				if err != nil && result != nil && strings.Contains(result.Stderr, "No such file or directory") {
					return result, errNFTTableMissing
				}
				return result, err
			})
		},
	}
}

func applyNFTBatch(script string, apply nftApplyFunc, exists nftTableExistsFunc) error {
	nftApplyMu.Lock()
	defer nftApplyMu.Unlock()

	batch := script
	result, err := exists()
	if err == nil {
		if result == nil || !strings.Contains(result.Stdout, `comment "AWGM_GATEWAY_OWNER"`) {
			return fmt.Errorf("refusing to replace gateway nftables table: not owned by AWGM")
		}
		batch = "delete table inet awgm_gateway\n" + script
	} else if !errors.Is(err, errNFTTableMissing) {
		return fmt.Errorf("inspect gateway nftables table: %w", err)
	}
	result, err = apply(batch)
	if err != nil {
		if result != nil && result.Stderr != "" {
			return fmt.Errorf("%s: %w", result.Stderr, err)
		}
		return err
	}
	return nil
}
