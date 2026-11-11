package singbox

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

func recoveryOperatorTree(t *testing.T, root string) []byte {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		tree[rel] = info.Mode().String()
		if !entry.IsDir() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			tree[rel] += "\n" + string(raw)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestRecoveryOperatorColdConstructionDoesNotWrite(t *testing.T) {
	for _, kind := range []string{"missing", "existing", "implicit-config"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			runtime := filepath.Join(root, "runtime")
			config := filepath.Join(runtime, "config.d")
			if kind != "missing" {
				if err := os.MkdirAll(config, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(config, "00-base.json"), []byte(`{"log":{"level":"debug"},"outbounds":[{"type":"direct","tag":"direct"}]}`), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(runtime, "sing-box.pid"), []byte("protected pid bytes"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(filepath.Join(root, "recovery-journal-proof"), []byte("protected journal bytes"), 0600); err != nil {
				t.Fatal(err)
			}
			before := recoveryOperatorTree(t, root)
			d := OperatorDeps{Dir: runtime, ConfigDir: config, Binary: filepath.Join(root, "not-installed"), DisableNDMSProxy: true}
			if kind == "implicit-config" {
				d.ConfigDir = ""
			}
			op := NewOperatorForRecovery(d)
			defer op.Process().Close()
			if !bytes.Equal(before, recoveryOperatorTree(t, root)) {
				t.Fatal("recovery constructor migrated/reconciled state before recovery")
			}
		})
	}
}

func TestRecoveryOperatorBlocksOrdinaryProcessMutations(t *testing.T) {
	for _, action := range []string{"start", "reload"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			op := NewOperatorForRecovery(OperatorDeps{Dir: filepath.Join(root, "runtime"), ConfigDir: filepath.Join(root, "runtime/config.d"), Binary: filepath.Join(root, "not-installed"), DisableNDMSProxy: true})
			defer op.Process().Close()
			var err error
			if action == "start" {
				err = op.Process().Start()
			} else {
				err = op.Process().Reload()
			}
			if !errors.Is(err, orchestrator.ErrRuntimeRecoveryRequired) {
				t.Fatal("ordinary process mutation did not wait for recovery", err)
			}
		})
	}
}

func TestRecoveryOperatorLegacyConstructorStillInitializes(t *testing.T) {
	root := t.TempDir()
	op := NewOperator(OperatorDeps{Dir: root, ConfigDir: filepath.Join(root, "config.d"), Binary: filepath.Join(root, "not-installed"), DisableNDMSProxy: true})
	defer op.Process().Close()
	if _, err := os.Stat(filepath.Join(root, "config.d/00-base.json")); err != nil {
		t.Fatal("legacy construction lost initialization", err)
	}
	if op.Process().recoveryRequired.Load() {
		t.Fatal("legacy construction gained a recovery barrier")
	}
}
func TestRecoveryOperatorKeepsManualStopIntent(t *testing.T) {
	root := t.TempDir()
	op := NewOperatorForRecovery(OperatorDeps{Dir: root, ConfigDir: filepath.Join(root, "config.d"), Binary: filepath.Join(root, "not-installed"), DisableNDMSProxy: true, InitialManuallyStopped: true})
	defer op.Process().Close()
	if !op.IsManuallyStopped() {
		t.Fatal("cold construction changed persisted manual-stop intent")
	}
}

func TestNativeRecoveryOperatorDoesNotAdoptBeforeRecovery(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_COLD_START_PROOF") != "1" {
		t.Skip("requires explicit native recovery-construction control")
	}
	binary := os.Getenv("SINGBOX_BIN")
	if binary == "" {
		t.Fatal("SINGBOX_BIN required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	root := t.TempDir()
	config := filepath.Join(root, "config.d")
	if err := os.Mkdir(config, 0700); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"log":{"level":"error"},"outbounds":[{"type":"direct","tag":"direct"}],"experimental":{"clash_api":{"external_controller":"127.0.0.1:10999"}}}`)
	if err := os.WriteFile(filepath.Join(config, "00-base.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(ctx, binary, "run", "-C", config)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	if err := os.WriteFile(filepath.Join(root, "sing-box.pid"), []byte(strconv.Itoa(cmd.Process.Pid)), 0600); err != nil {
		t.Fatal(err)
	}
	ready := false
	deadline := time.Now().Add(3 * time.Second)
	client := &http.Client{Timeout: 200 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://127.0.0.1:10999/version")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !ready {
		t.Fatal("owned existing native fixture not ready")
	}
	before := recoveryOperatorTree(t, root)
	op := NewOperatorForRecovery(OperatorDeps{Dir: root, ConfigDir: config, Binary: binary, DisableNDMSProxy: true})
	defer op.Process().Close()
	if op.Process().attached {
		t.Fatal("constructor adopted native engine before journal recovery")
	}
	if !bytes.Equal(before, recoveryOperatorTree(t, root)) {
		t.Fatal("native cold construction changed protected runtime inputs")
	}
	if adopted, _ := op.Process().AttachIfRunning(); adopted {
		t.Fatal("ordinary adoption bypassed recovery barrier")
	}
	if err := op.Process().Reload(); !errors.Is(err, orchestrator.ErrRuntimeRecoveryRequired) {
		t.Fatal("ordinary native reload bypassed recovery barrier", err)
	}
	if running, _ := op.Process().IsRunning(); running {
		t.Fatal("cold inspection claimed an unadopted process")
	}
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatal("cold construction damaged the existing fixture", err)
	}
	t.Log("native recovery constructor preserves running process without adoption; barrier controls passed")
}
