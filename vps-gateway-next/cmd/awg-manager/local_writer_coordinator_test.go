package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

func TestLocalWriterCoordinatorReachesConstructedCore(t *testing.T) {
	dataDir := t.TempDir()
	runtimeDir := t.TempDir()
	settings := storage.NewSettingsStore(dataDir)
	logs := logging.NewService(settings)
	t.Cleanup(logs.Stop)
	gate := &heavyop.Gate{}
	gate.Lock()
	defer gate.Unlock()
	core, err := buildSingboxCore(singboxCoreDeps{operationCoordinator: gate, settings: settings, appLog: logs, bus: events.NewBus(), bootLog: logging.NewScopedLogger(logs, logging.GroupSystem, logging.SubCleanup), dataDir: dataDir, dir: runtimeDir, configDir: filepath.Join(runtimeDir, "config.d"), binary: filepath.Join(runtimeDir, "not-installed"), ndmsProxyEnabled: func() bool { return false }, skipNDMSCleanup: true, disableNDMSProxy: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(core.op.Process().Close)
	t.Cleanup(core.orch.Close)
	for _, name := range []string{"operator", "orchestrator"} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
			defer cancel()
			var err error
			if name == "operator" {
				err = core.op.Control(ctx, "stop")
			} else {
				err = core.orch.ReloadContext(ctx)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("constructed core missed shared writer admission:%v", err)
			}
			if core.op.IsManuallyStopped() {
				t.Fatal("cancelled constructor-wired control persisted stop intent")
			}
		})
	}
}
