package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

func TestSetupSingbox_RejectsFailedGatewayMigration(t *testing.T) {
	for _, local := range []bool{false, true} {
		name := "runtime"
		if local {
			name = "local"
		}
		t.Run(name, func(t *testing.T) {
			dataDir := t.TempDir()
			configDir := t.TempDir()
			settings := storage.NewSettingsStore(dataDir)
			logs := logging.NewService(settings)
			t.Cleanup(logs.Stop)
			const policy = `{"route":{"final":"direct"}}`
			for _, name := range []string{"19-gateway-policy.json", "18-z-gateway-policy.json"} {
				if err := os.WriteFile(filepath.Join(configDir, name), []byte(policy), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(configDir, ".legacy-gateway-policy"), []byte("archive blocker"), 0600); err != nil {
				t.Fatal(err)
			}
			a := &app{
				dataDir: dataDir, singboxDir: t.TempDir(), singboxConfigDir: configDir,
				settingsStore: settings, settings: &storage.Settings{}, loggingService: logs,
				bootLog:  logging.NewScopedLogger(logs, logging.GroupSystem, logging.SubCleanup),
				eventBus: events.NewBus(), proxyAddr: "127.0.0.1:1080",
			}
			t.Cleanup(a.runOnExit)
			if local {
				if err := a.setupLocal(); err == nil {
					t.Error("portable startup accepted failed migration")
				}
			} else {
				if err := a.setupSingboxRuntime(); err == nil {
					t.Error("runtime startup accepted failed migration")
				}
			}
			if a.sbOrch != nil {
				t.Cleanup(a.sbOrch.Close)
				t.Error("failed migration exposed an orchestrator to producers")
			}
			if a.singboxOp != nil {
				t.Error("failed migration exposed an operator")
			}
			for _, name := range []string{"19-gateway-policy.json", "18-z-gateway-policy.json"} {
				if data, err := os.ReadFile(filepath.Join(configDir, name)); err != nil || string(data) != policy {
					t.Fatalf("failure changed policy bytes: %v", err)
				}
			}
		})
	}
}
