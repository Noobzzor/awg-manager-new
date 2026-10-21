//go:build linux

package ingress

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"strings"
	"syscall"
	"testing"
	"time"
)

// A control-plane process/socket fixture, NOT AWG, TUN or packet evidence.
func TestOwnedAWGControlChild(t *testing.T) {
	path := os.Getenv("AWGM_CONTROL_TEST_SOCKET")
	if path == "" { return }
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)
	listener, err := net.Listen("unix", path)
	if err != nil { os.Exit(31) }
	// Match the owned runtime overlay's policy; this helper is still NOT AWG.
	listener.(*net.UnixListener).SetUnlinkOnClose(false)
	<-stop
	_ = listener.Close()
	os.Exit(0)
}

func newAWGControlFixture(t *testing.T, expectedCleanupRefusal ...bool) (*UserspaceAWGInterface, *int) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil { t.Fatal(err) }
	dir := t.TempDir()
	cfg := UserspaceAWGConfig{RuntimeBinary: binary, ToolBinary: binary, IPBinary: binary, InterfaceName: "awg0", ListenPort: 51820, Address: "10.66.0.1/24", KeyDirectory: filepath.Join(dir, "keys"), Parameters: testedAWGParameters()}
	m := NewUserspaceAWGInterface(cfg)
	m.socketPath = filepath.Join(dir, "control.sock")
	m.checkTUN = func() error { return nil }
	m.netdevExists = func(string) (bool, error) { return m.child != nil, nil }
	starts := 0
	m.commandFactory = func(_ string, args ...string) *exec.Cmd {
		if len(args) != 2 || args[0] != "-f" || args[1] != cfg.InterfaceName { t.Fatal("foreground CLI contract lost") }
		starts++
		cmd := exec.Command(binary, "-test.run=^TestOwnedAWGControlChild$")
		cmd.Env = []string{"AWGM_CONTROL_TEST_SOCKET="+m.socketPath, "GOMAXPROCS=2"}
		return cmd
	}
	args, err := cfg.Parameters.SetArguments()
	if err != nil { t.Fatal(err) }
	values := map[string]string{}
	for i := 0; i < len(args); i+=2 { values[args[i]] = args[i+1] }
	m.run = func(_ context.Context, tool string, argv ...string) (string, error) {
		if tool != binary || len(argv) < 3 || argv[1] != cfg.InterfaceName { t.Fatal("unexpected preflight command") }
		if argv[0] == "set" {
			if strings.Join(argv[2:], " ") != strings.Join(args, " ") { t.Fatal("preflight must apply only the public AWG device profile") }
			return "", nil
		}
		if argv[0] == "show" && len(argv) == 3 {
			v, exists := values[argv[2]]
			if !exists { t.Fatal("preflight requested a dump or unknown field") }
			return v+"\n", nil
		}
		t.Fatal("unexpected preflight command")
		return "", nil
	}
	m.statusRun = func(ctx context.Context, tool, name, field, key string) (string, error) { return m.run(ctx, tool, "show", name, field) }
	t.Cleanup(func() {
		if err := m.Close(context.Background()); err != nil && !(len(expectedCleanupRefusal) == 1 && expectedCleanupRefusal[0]) { t.Error(err) }
	})
	return m, &starts
}

// This uses real protected filesystem markers and an injected already-joined
// child record. It proves cleanup aggregation only, not process/AWG ownership.
func TestUserspaceAWGClosePreservesIndependentCleanupRefusals(t *testing.T) {
	dir := t.TempDir()
	keyPath, socketPath := filepath.Join(dir, "foreign-key"), filepath.Join(dir, "foreign-socket")
	for _, path := range []string{keyPath, socketPath} {
		if err := os.WriteFile(path, []byte("foreign fixture marker"), 0o600); err != nil { t.Fatal(err) }
	}
	keyInfo, err := os.Lstat(keyPath)
	if err != nil { t.Fatal(err) }
	socketInfo, err := os.Lstat(socketPath)
	if err != nil { t.Fatal(err) }
	done := make(chan struct{})
	close(done)
	m := NewUserspaceAWGInterface(UserspaceAWGConfig{})
	m.keyPath, m.socketPath = keyPath, socketPath
	// No recorded inode belongs to this manager, so neither marker is removable.
	m.child = &ownedAWGChild{done: done}
	err = m.Close(context.Background())
	if err == nil || !strings.Contains(err.Error(), "AWG temporary key ownership unknown; removal refused") || !strings.Contains(err.Error(), "AWG residual socket ownership unknown; removal refused") {
		t.Fatal("cleanup lost an independent ownership refusal")
	}
	firstMessage := err.Error()
	if m.child != nil || !m.closed || m.ready { t.Fatal("cleanup state was not terminal after joined child") }
	if repeated := m.Close(context.Background()); repeated == nil || repeated.Error() != firstMessage { t.Fatal("repeated Close lost or changed sticky cleanup refusal") }
	for path, original := range map[string]os.FileInfo{keyPath: keyInfo, socketPath: socketInfo} {
		info, err := os.Lstat(path)
		if err != nil || !os.SameFile(info, original) { t.Fatal("cleanup touched a foreign filesystem marker") }
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "foreign fixture marker" { t.Fatal("cleanup changed foreign marker bytes") }
	}
}

// Real live child/socket replacement, with injected AWG/TUN observations only.
// The selected runtime's IPC package is tested independently in the owned overlay.
func TestUserspaceAWGLiveSocketReplacementPreserved(t *testing.T) {
	for _, action := range []string{"Close", "Preflight"} {
		t.Run(action, func(t *testing.T) {
			m, starts := newAWGControlFixture(t, true)
			if err := m.Preflight(context.Background()); err != nil { t.Fatal(err) }
			child := m.child
			if err := os.Rename(m.socketPath, m.socketPath+".owned"); err != nil { t.Fatal(err) }
			foreign, err := net.ListenUnix("unix", &net.UnixAddr{Name: m.socketPath, Net: "unix"})
			if err != nil { t.Fatal(err) }
			foreign.SetUnlinkOnClose(false)
			defer foreign.Close()
			original, err := os.Lstat(m.socketPath)
			if err != nil { t.Fatal(err) }
			if action == "Close" { err = m.Close(context.Background()) } else { err = m.Preflight(context.Background()) }
			info, statErr := os.Lstat(m.socketPath)
			if statErr != nil || !os.SameFile(info, original) { t.Fatal("live child shutdown deleted or changed foreign replacement socket") }
			if err == nil || !strings.Contains(err.Error(), "AWG residual socket ownership unknown; removal refused") { t.Fatal("foreign replacement must cause cleanup refusal") }
			if *starts != 1 || m.child != nil { t.Fatal("cleanup did not join the sole owned child") }
			select { case <-child.done: default: t.Fatal("child Wait not observed") }
			cleanup := m.Close(context.Background())
			if cleanup == nil || !strings.Contains(cleanup.Error(), "AWG residual socket ownership unknown; removal refused") { t.Fatal("cleanup refusal not sticky") }
			if repeated := m.Close(context.Background()); repeated == nil || repeated.Error() != cleanup.Error() { t.Fatal("terminal cleanup refusal changed") }
			conn, err := net.DialTimeout("unix", m.socketPath, 250*time.Millisecond)
			if err != nil { t.Fatal("foreign replacement socket no longer reachable") }
			_ = conn.Close()
		})
	}
}

func TestUserspaceAWGUnkeyedPreflightOwnsAndJoinsOneChild(t *testing.T) {
	m, starts := newAWGControlFixture(t)
	if err := m.Preflight(context.Background()); err != nil { t.Fatal(err) }
	if _, err := os.Stat(m.config.KeyDirectory); !os.IsNotExist(err) { t.Fatal("unkeyed preflight wrote key state") }
	if err := m.Preflight(context.Background()); err != nil { t.Fatal(err) }
	if *starts != 1 || m.Name() != "awg0" { t.Fatal("preflight must retain exactly one owned foreground child") }
	if err := m.Close(context.Background()); err != nil { t.Fatal(err) }
	if err := m.Close(context.Background()); err != nil { t.Fatal("Close must be idempotent") }
	if _, err := os.Lstat(m.socketPath); !os.IsNotExist(err) { t.Fatal("owned control socket survived join") }
	if err := m.Preflight(context.Background()); err == nil { t.Fatal("closed manager silently restarted a child") }
}

func TestUserspaceAWGPublicOutputBoundSurvivesIOCopy(t *testing.T) {
	output := &boundedAWGOutput{}
	// LimitedReader forces io.Copy's destination ReaderFrom optimization.
	_, err := io.Copy(output, io.LimitReader(strings.NewReader(strings.Repeat("x", 4097)), 4097))
	if err != nil { t.Fatal(err) }
	if output.Len() != 4096 || !output.overflow { t.Fatal("public output collector bypassed its fixed memory bound") }
}

func TestUserspaceAWGStaticFailureCannotRestartOrWriteKeyState(t *testing.T) {
	binary, err := os.Executable()
	if err != nil { t.Fatal(err) }
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing-runtime")
	m := NewUserspaceAWGInterface(UserspaceAWGConfig{RuntimeBinary: missing, ToolBinary: binary, IPBinary: binary, InterfaceName: "awg0", ListenPort: 51820, Address: "10.66.0.1/24", KeyDirectory: filepath.Join(dir, "keys"), Parameters: testedAWGParameters()})
	m.socketPath = filepath.Join(dir, "control.sock")
	m.checkTUN = func() error { return nil }
	m.netdevExists = func(string) (bool, error) { return false, nil }
	starts := 0
	m.commandFactory = func(string, ...string) *exec.Cmd { starts++; return exec.Command(binary, "-test.run=^TestOwnedAWGControlChild$") }
	t.Cleanup(func() { _ = m.Close(context.Background()) })
	if err := m.Preflight(context.Background()); err == nil { t.Fatal("missing runtime accepted") }
	if _, err := os.Stat(filepath.Join(dir, "keys")); !os.IsNotExist(err) { t.Fatal("failed preflight wrote key state") }
	if err := os.Symlink(binary, missing); err != nil { t.Fatal(err) }
	if err := m.Preflight(context.Background()); err == nil { t.Fatal("failed instance restarted") }
	if starts != 0 { t.Fatal("failed preflight automatically launched a later replacement runtime") }
}

func TestUserspaceAWGEnsureUsesOnlyAnOwnedProtectedTemporaryKeyFile(t *testing.T) {
	m, starts := newAWGControlFixture(t)
	// Public synthetic fixture, not a user/server credential.
	key := "AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	if err := m.SetPrivateKey(key); err == nil { t.Fatal("key accepted before semantic preflight") }
	if err := m.Ensure(context.Background()); err == nil { t.Fatal("Ensure started an unvalidated interface") }
	if *starts != 0 { t.Fatal("Ensure must never launch a process") }
	m, starts = newAWGControlFixture(t)
	if err := m.Preflight(context.Background()); err != nil { t.Fatal(err) }
	baseRun := m.run
	keyApplies, addressAdds, linkUps := 0, 0, 0
	m.run = func(ctx context.Context, binary string, args ...string) (string, error) {
		if len(args) == 6 && args[0] == "set" && args[2] == "private-key" {
			if args[1] != m.Name() || args[4] != "listen-port" || args[5] != "51820" { t.Fatal("key setter changed public interface identity") }
			info, err := os.Lstat(args[3])
			if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 { t.Fatal("key file is not a protected regular file") }
			if filepath.Dir(args[3]) != m.config.KeyDirectory { t.Fatal("key file escaped its private directory") }
			body, err := os.ReadFile(args[3])
			if err != nil || string(body) != key+"\n" { t.Fatal("key file does not match the synthetic fixture") }
			keyApplies++
			return "", nil
		}
		if strings.Join(args, " ") == "address add 10.66.0.1/24 dev awg0" { addressAdds++; return "", nil }
		if strings.Join(args, " ") == "link set up dev awg0" { linkUps++; return "", nil }
		return baseRun(ctx, binary, args...)
	}
	if err := m.SetPrivateKey(key); err != nil { t.Fatal(err) }
	if err := m.Ensure(context.Background()); err != nil { t.Fatal(err) }
	if err := m.Ensure(context.Background()); err != nil { t.Fatal(err) }
	if *starts != 1 || keyApplies != 1 || addressAdds != 1 || linkUps != 1 { t.Fatal("Ensure is not idempotent or launched a competing implementation") }
	entries, err := os.ReadDir(m.config.KeyDirectory)
	if err != nil || len(entries) != 0 { t.Fatal("temporary key file survived command completion") }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.Close(ctx); err != nil { t.Fatal("cancelled caller prevented child cleanup") }
}

func TestUserspaceAWGDefensivePreflightRejectsChangedWGCompatibleHeaders(t *testing.T) {
	m, _ := newAWGControlFixture(t)
	if err := m.Preflight(context.Background()); err != nil { t.Fatal(err) }
	baseRun := m.run
	m.run = func(ctx context.Context, binary string, args ...string) (string, error) {
		if strings.Join(args, " ") == "show awg0 h1" { return "1\n", nil }
		return baseRun(ctx, binary, args...)
	}
	if err := m.Preflight(context.Background()); err == nil { t.Fatal("defensive preflight accepted a changed WG-compatible header") }
	if m.child != nil { t.Fatal("failed semantic revalidation did not join the owned child") }
	if _, err := os.Stat(m.config.KeyDirectory); !os.IsNotExist(err) { t.Fatal("failed semantic revalidation wrote key state") }
}

// Regression coverage for already-present refusal/cleanup branches.
func TestUserspaceAWGRefusesForeignResourcesWithoutMutation(t *testing.T) {
	for _, scenario := range []string{"foreign-socket", "foreign-netdev", "unknown-netdev", "missing-tun"} {
		t.Run(scenario, func(t *testing.T) {
			m, starts := newAWGControlFixture(t)
			sentinel := "foreign-resource-must-survive"
			switch scenario {
			case "foreign-socket":
				if err := os.WriteFile(m.socketPath, []byte(sentinel), 0o600); err != nil { t.Fatal(err) }
			case "foreign-netdev": m.netdevExists = func(string) (bool, error) { return true, nil }
			case "unknown-netdev": m.netdevExists = func(string) (bool, error) { return false, fmt.Errorf("unknown") }
			case "missing-tun": m.checkTUN = func() error { return fmt.Errorf("missing") }
			}
			if err := m.Preflight(context.Background()); err == nil { t.Fatal("unsafe resource state accepted") }
			if err := m.Close(context.Background()); err != nil { t.Fatal(err) }
			if *starts != 0 { t.Fatal("foreign/unknown state launched a runtime") }
			if _, err := os.Stat(m.config.KeyDirectory); !os.IsNotExist(err) { t.Fatal("refusal wrote key state") }
			if scenario == "foreign-socket" {
				body, err := os.ReadFile(m.socketPath)
				if err != nil || string(body) != sentinel { t.Fatal("foreign socket leaf was mutated or removed") }
			}
		})
	}
}

func TestUserspaceAWGKeyApplyFailureDoesNotLeakAndJoinsChild(t *testing.T) {
	m, _ := newAWGControlFixture(t)
	if err := m.Preflight(context.Background()); err != nil { t.Fatal(err) }
	key := "AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	if err := m.SetPrivateKey(key); err != nil { t.Fatal(err) }
	baseRun := m.run
	m.run = func(ctx context.Context, binary string, args ...string) (string, error) {
		if len(args) == 6 && args[0] == "set" && args[2] == "private-key" { return key, fmt.Errorf("synthetic tool failure: %s", key) }
		return baseRun(ctx, binary, args...)
	}
	err := m.Ensure(context.Background())
	if err == nil || strings.Contains(err.Error(), key) { t.Fatal("key-apply failure was hidden or leaked its sensitive operand") }
	if m.child != nil { t.Fatal("key-apply failure did not join child") }
	if _, err := os.Lstat(m.socketPath); !os.IsNotExist(err) { t.Fatal("owned socket survived failed Ensure") }
	entries, err := os.ReadDir(m.config.KeyDirectory)
	if err != nil || len(entries) != 0 { t.Fatal("protected temporary key survived failure") }
}

func TestUserspaceAWGEnsurePreservesPrimaryErrorAlongsideCleanupRefusal(t *testing.T) {
	m, _ := newAWGControlFixture(t, true)
	if err := m.Preflight(context.Background()); err != nil { t.Fatal(err) }
	key := "AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" // public synthetic fixture
	if err := m.SetPrivateKey(key); err != nil { t.Fatal(err) }
	baseRun := m.run
	foreignPath := ""
	m.run = func(ctx context.Context, binary string, args ...string) (string, error) {
		if len(args) == 6 && args[0] == "set" && args[2] == "private-key" {
			foreignPath = args[3]
			if err := os.Rename(foreignPath, foreignPath+".owned"); err != nil { t.Fatal(err) }
			if err := os.WriteFile(foreignPath, []byte("foreign-leaf"), 0o600); err != nil { t.Fatal(err) }
			return key, fmt.Errorf("synthetic tool failure: %s", key)
		}
		return baseRun(ctx, binary, args...)
	}
	err := m.Ensure(context.Background())
	if err == nil || !strings.Contains(err.Error(), "server identity apply failed") || !strings.Contains(err.Error(), "temporary key ownership unknown") { t.Fatal("cleanup replaced the sanitized primary failure instead of preserving both causes") }
	if strings.Contains(err.Error(), key) { t.Fatal("aggregated error leaked a sensitive tool operand") }
	if m.child != nil { t.Fatal("double-fault cleanup did not join its owned child") }
	body, readErr := os.ReadFile(foreignPath)
	if readErr != nil || string(body) != "foreign-leaf" { t.Fatal("cleanup modified the foreign key replacement") }
	if err := m.Close(context.Background()); err == nil { t.Fatal("unknown cleanup ownership was not sticky") }
}
