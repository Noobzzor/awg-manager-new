package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/gateway/routerpolicy"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

func TestGatewayOwnerStartupRefusesBeforeRuntimeConstruction(t *testing.T) {
	for _, state := range []string{"original", "unknown", "unclaimed"} {
		t.Run(state, func(t *testing.T) {
			root := t.TempDir()
			dir := routerpolicy.OriginalStateDir(root)
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if state != "unclaimed" {
				if err := os.WriteFile(filepath.Join(dir, "owner.json"), []byte(`{"version":1,"mode":"`+state+`"}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			settings := storage.NewSettingsStore(root)
			logs := logging.NewService(settings)
			defer logs.Stop()
			runtimeDir := filepath.Join(root, "engine")
			core, err := buildSingboxCore(singboxCoreDeps{settings: settings, appLog: logs, bus: events.NewBus(), bootLog: logging.NewScopedLogger(logs, logging.GroupSystem, logging.SubCleanup), dataDir: root, dir: runtimeDir, configDir: filepath.Join(runtimeDir, "config.d"), binary: filepath.Join(root, "not-installed"), disableNDMSProxy: true, skipNDMSCleanup: true, ndmsProxyEnabled: func() bool { return false }, initialManuallyStopped: true})
			if core.orch != nil {
				defer core.orch.Close()
			}
			if core.op != nil {
				defer core.op.Process().Close()
			}
			if !errors.Is(err, routerpolicy.ErrGatewayPolicyOwnership) {
				t.Fatal("legacy startup bypassed original/unknown owner before runtime construction", err)
			}
			if _, err := os.Stat(runtimeDir); !os.IsNotExist(err) {
				t.Fatal("refused startup already reconciled or created runtime state")
			}
		})
	}
}
