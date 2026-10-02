package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/hoaxisr/awg-manager/internal/singbox"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

type oneShotHooks struct {
	localCleanup  func(runtimeSettings) error
	legacyCleanup func(string) error
	legacyService func(string, string)
}

// runOneShot keeps local one-shot commands on the portable side of the
// deployment boundary. In particular, --service is an Entware init-script
// compatibility surface and is deliberately unavailable in local mode.
func runOneShot(mode deploymentMode, runtime runtimeSettings, cleanup bool, service string, h oneShotHooks) (bool, error) {
	if cleanup {
		if mode == deploymentModeSingbox {
			return true, h.localCleanup(runtime)
		}
		return true, h.legacyCleanup(runtime.dataDir)
	}
	if service != "" {
		if mode == deploymentModeSingbox {
			return true, fmt.Errorf("--service is unsupported with AWG_MODE=singbox; use the container/process supervisor")
		}
		h.legacyService(service, runtime.dataDir)
		return true, nil
	}
	return false, nil
}

// runLocalCleanup stops only the sing-box process identified by the local
// managed PID/argv contract, then removes the owned runtime directory. An
// explicitly supplied binary outside that directory is image/user-owned and
// is not removed.
func runLocalCleanup(runtime runtimeSettings) error {
	return runLocalCleanupWith(runtime, func() error {
		runtimeDir := filepath.Join(runtime.dataDir, "sing-box")
		op := singbox.NewOperator(singbox.OperatorDeps{
			Dir:                runtimeDir,
			ConfigDir:          runtime.singboxConfigDir,
			Binary:             runtime.singboxBinary,
			DisableNDMSProxy:   true,
			IsNDMSProxyEnabled: func() bool { return false },
		})
		return op.Uninstall(context.Background())
	})
}

// The callback isolates destructive uninstall from preflight regression tests.
func runLocalCleanupWith(runtime runtimeSettings, uninstall func() error) error {
	configDir := singbox.EffectiveConfigDir(filepath.Join(runtime.dataDir, "sing-box"), runtime.singboxConfigDir)
	if err := orchestrator.ValidateStateDirectories(configDir); err != nil {
		return fmt.Errorf("cleanup preflight: %w", err)
	}
	return uninstall()
}
