//go:build linux

package ingress

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"strings"
	"sync"
	"syscall"
	"time"
)

type UserspaceAWGConfig struct {
	RuntimeBinary string
	ToolBinary string
	IPBinary string
	InterfaceName string
	ListenPort int
	Address string
	KeyDirectory string
	Parameters AWGParameters
}

type ownedAWGChild struct {
	cmd *exec.Cmd
	done chan struct{}
}

// UserspaceAWGInterface owns one foreground child, never an adopted netdev.
// Its constructor has no effects; register Close before calling Preflight.
// A failed/closed instance cannot restart itself or switch to WireGuard.
type UserspaceAWGInterface struct {
	mu sync.Mutex
	config UserspaceAWGConfig
	socketPath string
	socketInfo os.FileInfo
	child *ownedAWGChild
	ready bool
	closed bool
	closeErr error
	privateKey string
	keyPath string
	keyInfo os.FileInfo
	configured bool
	run CommandRunner
	statusRun func(context.Context, string, string, string, string) (string, error)
	commandFactory func(string, ...string) *exec.Cmd
	checkTUN func() error
	netdevExists func(string) (bool, error)
}

func NewUserspaceAWGInterface(config UserspaceAWGConfig) *UserspaceAWGInterface {
	m := &UserspaceAWGInterface{config: config, socketPath: filepath.Join("/var/run/amneziawg", config.InterfaceName+".sock"), run: runUserspaceAWGCommand}
	m.statusRun = runUserspaceAWGSelectedPublicField
	m.commandFactory = func(path string, args ...string) *exec.Cmd {
		cmd := exec.Command(path, args...)
		cmd.Env = []string{"PATH=/usr/bin:/bin", "LOG_LEVEL=error", "GOMAXPROCS=2", "GOMEMLIMIT=40MiB"}
		return cmd
	}
	m.checkTUN = func() error {
		info, err := os.Stat("/dev/net/tun")
		if err != nil || info.Mode()&os.ModeCharDevice == 0 { return fmt.Errorf("userspace AWG requires the TUN device") }
		f, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
		if err != nil { return fmt.Errorf("userspace AWG TUN access refused") }
		return f.Close()
	}
	m.netdevExists = func(name string) (bool, error) {
		_, err := os.Lstat(filepath.Join("/sys/class/net", name))
		if os.IsNotExist(err) { return false, nil }
		return err == nil, err
	}
	return m
}

func (m *UserspaceAWGInterface) Name() string {
	if m == nil { return "" }
	return m.config.InterfaceName
}

func (m *UserspaceAWGInterface) validate() error {
	if m.run == nil || m.commandFactory == nil || m.checkTUN == nil || m.netdevExists == nil { return fmt.Errorf("userspace AWG dependencies are not configured") }
	c := m.config
	if len(c.InterfaceName) == 0 || len(c.InterfaceName) > 15 { return fmt.Errorf("invalid userspace AWG interface name") }
	for _, r := range c.InterfaceName {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') { return fmt.Errorf("invalid userspace AWG interface name") }
	}
	if c.ListenPort < 1 || c.ListenPort > 65535 { return fmt.Errorf("invalid userspace AWG listen port") }
	address, err := netip.ParsePrefix(c.Address)
	if err != nil || !address.Addr().Is4() { return fmt.Errorf("invalid userspace AWG server address") }
	if !filepath.IsAbs(c.KeyDirectory) || filepath.Clean(c.KeyDirectory) != c.KeyDirectory { return fmt.Errorf("invalid userspace AWG key directory") }
	if err := c.Parameters.Validate(); err != nil { return err }
	for _, path := range []string{c.RuntimeBinary, c.ToolBinary, c.IPBinary} {
		if !filepath.IsAbs(path) || strings.ContainsAny(path, "\r\n") { return fmt.Errorf("userspace AWG requires explicit executable paths") }
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 { return fmt.Errorf("userspace AWG required executable unavailable") }
	}
	return nil
}

func (m *UserspaceAWGInterface) Preflight(ctx context.Context) (err error) {
	if m == nil { return fmt.Errorf("userspace AWG interface is not configured") }
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed { return fmt.Errorf("userspace AWG interface is closed") }
	// Make every failure terminal, including missing binaries before Start.
	defer func() {
		if err != nil {
			if closeErr := m.closeLocked(); closeErr != nil { err = errors.Join(err, fmt.Errorf("AWG preflight owned cleanup incomplete: %w", closeErr)) }
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil { return err }
	if err := m.validate(); err != nil { return err }
	if m.ready {
		select { case <-m.child.done: return fmt.Errorf("owned AWG child exited; restart refused"); default: }
		info, err := os.Lstat(m.socketPath)
		if err != nil || !os.SameFile(info, m.socketInfo) { return fmt.Errorf("owned AWG socket changed; adoption refused") }
		return m.readbackParametersLocked(ctx)
	}
	if err := m.checkTUN(); err != nil { return err }
	foreign, err := m.netdevExists(m.Name())
	if err != nil || foreign { return fmt.Errorf("foreign or unknown AWG netdev refused") }
	if _, err := os.Lstat(m.socketPath); !os.IsNotExist(err) { return fmt.Errorf("foreign or unknown AWG socket refused") }
	if info, err := os.Lstat(filepath.Dir(m.socketPath)); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o022 != 0 { return fmt.Errorf("unsafe AWG socket directory") }
	} else if !os.IsNotExist(err) { return fmt.Errorf("unknown AWG socket directory") }
	cmd := m.commandFactory(m.config.RuntimeBinary, "-f", m.Name())
	if cmd == nil { return fmt.Errorf("userspace AWG command is not configured") }
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	// Ownership becomes reachable before Start, including its error paths.
	child := &ownedAWGChild{cmd: cmd, done: make(chan struct{})}
	m.child = child
	if err := cmd.Start(); err != nil { close(child.done); m.child = nil; return fmt.Errorf("userspace AWG foreground start failed") }
	go func() { _ = cmd.Wait(); close(child.done) }()
	ticker := time.NewTicker(20*time.Millisecond)
	defer ticker.Stop()
	for {
		select { case <-child.done: return fmt.Errorf("userspace AWG exited before readiness"); default: }
		info, statErr := os.Lstat(m.socketPath)
		if statErr == nil {
			if info.Mode()&os.ModeSocket == 0 { return fmt.Errorf("userspace AWG created an invalid control socket") }
			m.socketInfo = info
			break
		}
		if !os.IsNotExist(statErr) { return fmt.Errorf("userspace AWG socket readiness unknown") }
		select { case <-ctx.Done(): return ctx.Err(); case <-child.done: return fmt.Errorf("userspace AWG exited before readiness"); case <-ticker.C: }
	}
	args, err := m.config.Parameters.SetArguments()
	if err != nil { return err }
	if _, err := m.run(ctx, m.config.ToolBinary, append([]string{"set", m.Name()}, args...)...); err != nil { return fmt.Errorf("userspace AWG public parameter apply failed") }
	if err := m.readbackParametersLocked(ctx); err != nil { return err }
	select { case <-child.done: return fmt.Errorf("userspace AWG exited during preflight"); default: }
	m.ready = true
	return nil
}

func (m *UserspaceAWGInterface) readbackParametersLocked(ctx context.Context) error {
	args, err := m.config.Parameters.SetArguments()
	if err != nil { return err }
	for i := 0; i < len(args); i += 2 {
		output, err := m.run(ctx, m.config.ToolBinary, "show", m.Name(), args[i])
		if err != nil || strings.TrimSpace(output) != args[i+1] { return fmt.Errorf("userspace AWG parameter readback mismatch: %s", args[i]) }
	}
	return nil
}

var _ ManagedInterface = (*UserspaceAWGInterface)(nil)

// The semantic gate must precede identity installation. Do not put the key
// or raw tool output in errors, logs or command arguments.
func (m *UserspaceAWGInterface) SetPrivateKey(key string) error {
	if m == nil { return fmt.Errorf("userspace AWG interface is not configured") }
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.ownedReadyLocked(); err != nil { return err }
	decoded, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(decoded) != 32 || base64.StdEncoding.EncodeToString(decoded) != key || bytes.Equal(decoded, make([]byte, 32)) { return fmt.Errorf("invalid userspace AWG private key") }
	if m.privateKey != "" && m.privateKey != key { return fmt.Errorf("userspace AWG private key replacement refused") }
	m.privateKey = key
	return nil
}

func (m *UserspaceAWGInterface) ownedReadyLocked() error {
	if m.closed || !m.ready || m.child == nil { return fmt.Errorf("userspace AWG semantic preflight required") }
	select { case <-m.child.done: return fmt.Errorf("owned AWG child exited"); default: }
	info, err := os.Lstat(m.socketPath)
	if err != nil || !os.SameFile(info, m.socketInfo) { return fmt.Errorf("owned AWG socket changed; adoption refused") }
	return nil
}

func (m *UserspaceAWGInterface) Ensure(ctx context.Context) (err error) {
	if m == nil { return fmt.Errorf("userspace AWG interface is not configured") }
	m.mu.Lock()
	defer m.mu.Unlock()
	defer func() { if err != nil { if cleanup := m.closeLocked(); cleanup != nil { err = errors.Join(err, fmt.Errorf("AWG Ensure owned cleanup incomplete: %w", cleanup)) } } }()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil { return err }
	if err := m.ownedReadyLocked(); err != nil { return err }
	if m.configured { return nil }
	if m.privateKey == "" { return fmt.Errorf("userspace AWG server identity required") }
	// Refuse symlinked state roots before mkdir/write, including ancestors.
	for path := m.config.KeyDirectory; path != filepath.Dir(path); path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) { return fmt.Errorf("unsafe userspace AWG key directory") }
		if err != nil && !os.IsNotExist(err) { return fmt.Errorf("unknown userspace AWG key directory") }
	}
	if err := os.MkdirAll(m.config.KeyDirectory, 0o700); err != nil { return fmt.Errorf("userspace AWG key directory unavailable") }
	info, err := os.Lstat(m.config.KeyDirectory)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o022 != 0 { return fmt.Errorf("unsafe userspace AWG key directory permissions") }
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != uint32(os.Geteuid()) { return fmt.Errorf("userspace AWG key directory owner refused") }
	f, err := os.CreateTemp(m.config.KeyDirectory, "awg-server-")
	if err != nil { return fmt.Errorf("protected userspace AWG key file unavailable") }
	m.keyPath = f.Name()
	m.keyInfo, err = f.Stat()
	if err != nil || !m.keyInfo.Mode().IsRegular() || m.keyInfo.Mode().Perm() != 0o600 { _ = f.Close(); return fmt.Errorf("protected userspace AWG key file verification failed") }
	_, writeErr := f.WriteString(m.privateKey+"\n")
	closeErr := f.Close()
	if writeErr != nil || closeErr != nil { return fmt.Errorf("protected userspace AWG key file write failed") }
	if _, err := m.run(ctx, m.config.ToolBinary, "set", m.Name(), "private-key", m.keyPath, "listen-port", strconv.Itoa(m.config.ListenPort)); err != nil { return fmt.Errorf("userspace AWG server identity apply failed") }
	if err := m.removeKeyLocked(); err != nil { return err }
	if _, err := m.run(ctx, m.config.IPBinary, "address", "add", m.config.Address, "dev", m.Name()); err != nil { return fmt.Errorf("userspace AWG owned address apply failed") }
	if _, err := m.run(ctx, m.config.IPBinary, "link", "set", "up", "dev", m.Name()); err != nil { return fmt.Errorf("userspace AWG owned interface up failed") }
	m.configured = true
	return nil
}

func (m *UserspaceAWGInterface) removeKeyLocked() error {
	if m.keyPath == "" { return nil }
	info, err := os.Lstat(m.keyPath)
	if os.IsNotExist(err) { m.keyPath, m.keyInfo = "", nil; return nil }
	if err != nil || m.keyInfo == nil || !os.SameFile(info, m.keyInfo) { return fmt.Errorf("AWG temporary key ownership unknown; removal refused") }
	if err := os.Remove(m.keyPath); err != nil { return fmt.Errorf("owned AWG temporary key removal failed") }
	m.keyPath, m.keyInfo = "", nil
	return nil
}

func (m *UserspaceAWGInterface) Close(context.Context) error {
	if m == nil { return nil }
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeLocked()
}

// Cleanup has its own finite join budget, not a cancelled HTTP/request context.
func (m *UserspaceAWGInterface) closeLocked() error {
	if m.closed && m.child == nil { return m.closeErr }
	m.closed, m.ready = true, false
	m.privateKey = ""
	if err := m.removeKeyLocked(); err != nil { m.closeErr = errors.Join(m.closeErr, err) }
	child := m.child
	if child == nil { return m.closeErr }
	select {
	case <-child.done:
	default:
		_ = child.cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-child.done:
		case <-time.After(2*time.Second):
			_ = child.cmd.Process.Kill()
			select { case <-child.done: m.closeErr = errors.Join(m.closeErr, fmt.Errorf("owned AWG required forced termination")); case <-time.After(2*time.Second): m.closeErr = errors.Join(m.closeErr, fmt.Errorf("owned AWG join incomplete")); return m.closeErr }
		}
	}
	m.child = nil
	info, err := os.Lstat(m.socketPath)
	if err == nil {
		if m.socketInfo == nil || !os.SameFile(info, m.socketInfo) { m.closeErr = errors.Join(m.closeErr, fmt.Errorf("AWG residual socket ownership unknown; removal refused")) } else if err := os.Remove(m.socketPath); err != nil { m.closeErr = errors.Join(m.closeErr, fmt.Errorf("owned AWG socket removal failed")) }
	} else if !os.IsNotExist(err) { m.closeErr = errors.Join(m.closeErr, fmt.Errorf("AWG socket cleanup state unknown")) }
	return m.closeErr
}

type boundedAWGOutput struct {
	buffer bytes.Buffer
	overflow bool
}

func (b *boundedAWGOutput) Len() int { return b.buffer.Len() }
func (b *boundedAWGOutput) String() string { return b.buffer.String() }

func (b *boundedAWGOutput) Write(p []byte) (int, error) {
	n := len(p)
	remaining := 4096-b.Len()
	if n > remaining { b.overflow = true; p = p[:remaining] }
	_, _ = b.buffer.Write(p)
	return n, nil
}

// Capture bounded public output only; never retain raw stderr or key dumps.
func runUserspaceAWGCommand(parent context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "LANG=C"}
	cmd.WaitDelay = 250*time.Millisecond
	output := &boundedAWGOutput{}
	cmd.Stdout, cmd.Stderr = output, io.Discard
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil { return "", ctx.Err() }
		return "", fmt.Errorf("userspace AWG command failed")
	}
	if output.overflow { return "", fmt.Errorf("userspace AWG public output exceeded bound") }
	return output.String(), nil
}


