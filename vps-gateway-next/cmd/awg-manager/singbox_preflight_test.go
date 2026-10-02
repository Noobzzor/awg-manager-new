package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

func TestSetupSingbox_PreflightBeforeAnyMigration(t *testing.T) {
	for _, mode := range []string{"runtime", "local"} {
		for _, location := range []string{"root", "disabled", "pending"} {
			t.Run(mode+"/"+location, func(t *testing.T) {
				external := t.TempDir()
				original := map[string]string{
					"20-router.json":    `{"route":{"rule_set":[{"type":"remote","tag":"fixture","url":"https://github.com/vernette/rulesets/raw/master/fixture.srs"}]}}`,
					"00-base.json":      `{"log":{"level":"debug"}}`,
					"sentinel.tmp.keep": "unchanged external file",
				}
				for name, data := range original {
					if err := os.WriteFile(filepath.Join(external, name), []byte(data), 0600); err != nil {
						t.Fatal(err)
					}
				}
				configDir := filepath.Join(t.TempDir(), "config.d")
				link := configDir
				if location != "root" {
					if err := os.Mkdir(configDir, 0755); err != nil {
						t.Fatal(err)
					}
					link = filepath.Join(configDir, location)
				}
				if err := os.Symlink(external, link); err != nil {
					t.Fatal(err)
				}
				dataDir := t.TempDir()
				settings := storage.NewSettingsStore(dataDir)
				logs := logging.NewService(settings)
				t.Cleanup(logs.Stop)
				a := &app{
					dataDir: dataDir, singboxDir: t.TempDir(), singboxConfigDir: configDir,
					settingsStore: settings, settings: &storage.Settings{}, loggingService: logs,
					bootLog:  logging.NewScopedLogger(logs, logging.GroupSystem, logging.SubCleanup),
					eventBus: events.NewBus(), proxyAddr: "127.0.0.1:1080",
				}
				t.Cleanup(a.runOnExit)
				var err error
				if mode == "local" {
					err = a.setupLocal()
				} else {
					err = a.setupSingboxRuntime()
				}
				if err == nil {
					t.Error("startup accepted a linked state directory")
				}
				if a.sbOrch != nil {
					t.Cleanup(a.sbOrch.Close)
					t.Error("failed preflight exposed an orchestrator")
				}
				if a.singboxOp != nil {
					t.Error("failed preflight exposed an operator")
				}
				entries, err := os.ReadDir(external)
				if err != nil || len(entries) != len(original) {
					t.Fatalf("external file set changed: %v", err)
				}
				for name, want := range original {
					data, err := os.ReadFile(filepath.Join(external, name))
					if err != nil || string(data) != want {
						t.Errorf("external %s modified before preflight refusal: %v", name, err)
					}
				}
			})
		}
	}
}
