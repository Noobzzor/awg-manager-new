package singbox

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
)

func TestNativeRecoveryProcessIdentity(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_COLD_START_PROOF") != "1" {
		t.Skip("requires isolated real engine identity control")
	}
	binary := os.Getenv("SINGBOX_BIN")
	if binary == "" {
		t.Fatal("SINGBOX_BIN required")
	}
	root := t.TempDir()
	config := filepath.Join(root, "config.d")
	if err := os.Mkdir(config, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "00-base.json"), []byte(`{"log":{"level":"error"},"outbounds":[{"type":"direct","tag":"direct"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	op := NewOperatorForRecovery(OperatorDeps{Dir: root, ConfigDir: config, Binary: binary, DisableNDMSProxy: true})
	p := op.Process()
	p.logDir = root
	p.ReloadNeedsRestart = func() bool { return true }
	lease, err := p.AcquireProcessLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	defer func() { _ = lease.Stop(); p.Close() }()
	if err := lease.Start(); err != nil {
		t.Fatal("owned cold spawn", err)
	}
	running, pid := p.IsRunning()
	if !running {
		t.Fatal("spawned native runtime not recognized")
	}
	if spawned, err := lease.StartSpawned(); err != nil || spawned {
		t.Fatal("owned process duplicated", spawned, err)
	}
	if err := lease.Reload(); err != nil {
		t.Fatal("owned restart", err)
	}
	running, next := p.IsRunning()
	if !running || next == pid {
		t.Fatal("owned restart failed", pid, next)
	}
	// A second manager looking at precisely the same binary/config/PID cannot
	// claim this process just because /proc and the PID file look familiar.
	other := NewOperatorForRecovery(OperatorDeps{Dir: root, ConfigDir: config, Binary: binary, DisableNDMSProxy: true})
	defer other.Process().Close()
	second, err := other.Process().AcquireProcessLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []func() error{second.Start, second.Reload, second.Stop} {
		if err := action(); !errors.Is(err, ErrRuntimeProcessOwnership) {
			t.Fatal("second manager claimed existing native process", err)
		}
	}
	second.Close()
	if running, actual := p.IsRunning(); !running || actual != next {
		t.Fatal("second manager damaged owned process")
	}
	// Kernel birth time and executable identity are independently checked.
	original := p.spawnedIdentity.Load()
	defer p.spawnedIdentity.Store(original)
	changed := *original
	changed.birth = "0"
	p.spawnedIdentity.Store(&changed)
	if running, _ := p.IsRunning(); running {
		t.Fatal("changed birth identity accepted")
	}
	if err := lease.Stop(); !errors.Is(err, ErrRuntimeProcessOwnership) {
		t.Fatal("changed identity allowed stop", err)
	}
	p.spawnedIdentity.Store(original)
	if err := lease.Stop(); err != nil {
		t.Fatal("verified native stop", err)
	}
	if running, _ := p.IsRunning(); running {
		t.Fatal("native runtime survived stop")
	}
	t.Log("native recovery identity: owned spawn/restart/stop; second-manager and changed-birth refusal passed")
}

func TestNativeRecoveryProcessLostPID(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_COLD_START_PROOF") != "1" {
		t.Skip("requires isolated real engine identity control")
	}
	binary := os.Getenv("SINGBOX_BIN")
	if binary == "" {
		t.Fatal("SINGBOX_BIN required")
	}
	root := t.TempDir()
	config := filepath.Join(root, "config.d")
	if err := os.Mkdir(config, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "00-base.json"), []byte(`{"log":{"level":"error"},"outbounds":[{"type":"direct","tag":"direct"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	op := NewOperatorForRecovery(OperatorDeps{Dir: root, ConfigDir: config, Binary: binary, DisableNDMSProxy: true})
	p := op.Process()
	p.logDir = root
	lease, err := p.AcquireProcessLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Retain every actually spawned identity for cleanup even on the RED
	// implementation, which can overwrite its identity and leak the first child.
	var children []int
	defer func() {
		if current := p.spawnedIdentity.Load(); current != nil {
			children = append(children, current.pid)
		}
		for _, pid := range children {
			child, err := os.FindProcess(pid)
			if err == nil {
				_ = child.Kill()
				_ = child.Release()
			}
		}
		p.Close()
		lease.Close()
	}()
	if err := lease.Start(); err != nil {
		t.Fatal(err)
	}
	original := p.spawnedIdentity.Load()
	children = append(children, original.pid)
	if err := os.Remove(p.pidPath); err != nil {
		t.Fatal(err)
	}
	err = lease.Start()
	if !errors.Is(err, ErrRuntimeProcessOwnership) {
		t.Error("missing PID record permitted duplicate native spawn", err)
	}
	if _, err := os.Lstat(p.pidPath); !os.IsNotExist(err) {
		t.Error("missing PID record was rewritten before recovery")
	}
	if current := p.spawnedIdentity.Load(); current != nil && current.pid != original.pid {
		children = append(children, current.pid)
		child, _ := os.FindProcess(current.pid)
		_ = child.Kill()
		_ = child.Release()
	}
	p.spawnedIdentity.Store(original)
	if err := os.WriteFile(p.pidPath, []byte(strconv.Itoa(original.pid)), 0600); err != nil {
		t.Fatal(err)
	}
	if err := lease.Stop(); err != nil {
		t.Fatal("restored owned record could not stop native process", err)
	}
	if !t.Failed() {
		t.Log("native recovery lost PID: no duplicate spawn, record preserved until explicit restoration")
	}
}

func TestNativeRecoveryProcessSignalFailure(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_COLD_START_PROOF") != "1" {
		t.Skip("requires isolated real engine identity control")
	}
	binary := os.Getenv("SINGBOX_BIN")
	if binary == "" {
		t.Fatal("SINGBOX_BIN required")
	}
	root := t.TempDir()
	config := filepath.Join(root, "config.d")
	if err := os.Mkdir(config, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "00-base.json"), []byte(`{"log":{"level":"error"},"outbounds":[{"type":"direct","tag":"direct"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	op := NewOperatorForRecovery(OperatorDeps{Dir: root, ConfigDir: config, Binary: binary, DisableNDMSProxy: true})
	p := op.Process()
	p.logDir = root
	lease, err := p.AcquireProcessLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var pid int
	defer func() {
		p.signalFn = nil
		if pid > 0 {
			_ = os.WriteFile(p.pidPath, []byte(strconv.Itoa(pid)), 0600)
		}
		_ = lease.Stop()
		p.Close()
		lease.Close()
	}()
	if err := lease.Start(); err != nil {
		t.Fatal(err)
	}
	_, pid = p.IsRunning()
	p.signalFn = func(int, syscall.Signal) error { return syscall.EPERM }
	if err := lease.Stop(); !errors.Is(err, syscall.EPERM) {
		t.Error("signal refusal reported successful stop", err)
	}
	if got, err := os.ReadFile(p.pidPath); err != nil || string(got) != strconv.Itoa(pid) {
		t.Error("signal refusal erased owned PID record")
	}
	if running, actual := p.IsRunning(); !running || actual != pid {
		t.Error("signal refusal lost still-running child identity")
	}
	if p.curGen.deliberate.Load() {
		t.Error("denied signal marked a future crash as deliberate")
	}
	if !t.Failed() {
		t.Log("native recovery denied signal: error, PID record, live identity and exit intent preserved")
	}
}

// A same-name live process is not evidence that this manager owns it. Use
// signalFn only as a tripwire: an unsafe implementation must not kill a test
// runner or its fixture while demonstrating the failure.
func TestRecoveryProcessRejectsUnownedLivePID(t *testing.T) {
	for _, action := range []string{"start", "reload", "stop", "attach"} {
		t.Run(action, func(t *testing.T) {
			foreign := exec.Command("/bin/sleep", "20")
			if err := foreign.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = foreign.Process.Kill(); _ = foreign.Wait() }()
			root := t.TempDir()
			pidPath := filepath.Join(root, "sing-box.pid")
			record := []byte(strconv.Itoa(foreign.Process.Pid))
			if err := os.WriteFile(pidPath, record, 0600); err != nil {
				t.Fatal(err)
			}
			op := NewOperatorForRecovery(OperatorDeps{Dir: root, ConfigDir: filepath.Join(root, "config.d"), Binary: "/bin/sleep", DisableNDMSProxy: true})
			p := op.Process()
			defer p.Close()
			signals, spawns := 0, 0
			p.signalFn = func(int, syscall.Signal) error { signals++; return syscall.ESRCH }
			p.startCmd = func(string, ...string) (*exec.Cmd, error) { spawns++; return nil, syscall.EPERM }
			lease, err := p.AcquireProcessLease(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			switch action {
			case "start":
				err = lease.Start()
			case "reload":
				err = lease.Reload()
			case "stop":
				err = lease.Stop()
			case "attach":
				lease.Close()
				p.SetRuntimeRecoveryRequired(false)
				adopted, _ := p.AttachIfRunning()
				if adopted {
					t.Error("adopted unowned same-binary process")
				}
			}
			lease.Close()
			if action != "attach" && err == nil {
				t.Error("accepted unowned live PID")
			}
			if signals != 0 || spawns != 0 {
				t.Errorf("unowned process mutation: signals=%d spawns=%d", signals, spawns)
			}
			if running, _ := p.IsRunning(); running {
				t.Error("unowned process reported as managed runtime")
			}
			got, err := os.ReadFile(pidPath)
			if err != nil || string(got) != string(record) {
				t.Error("unowned PID record changed")
			}
			if err := foreign.Process.Signal(syscall.Signal(0)); err != nil {
				t.Fatal("foreign fixture damaged", err)
			}
		})
	}
}

func TestRecoveryProcessRejectsInvalidPIDRecord(t *testing.T) {
	for _, data := range []string{"garbage", "0", "-1"} {
		t.Run(data, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "sing-box.pid")
			if err := os.WriteFile(path, []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			op := NewOperatorForRecovery(OperatorDeps{Dir: root, ConfigDir: filepath.Join(root, "config.d"), Binary: "/bin/false", DisableNDMSProxy: true})
			p := op.Process()
			defer p.Close()
			spawns := 0
			p.startCmd = func(string, ...string) (*exec.Cmd, error) { spawns++; return nil, syscall.EPERM }
			lease, err := p.AcquireProcessLease(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			if err := lease.Start(); err == nil {
				t.Error("invalid PID record accepted")
			}
			if spawns != 0 {
				t.Error("invalid PID record allowed spawn")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != data {
				t.Error("invalid PID record rewritten")
			}
		})
	}
}
