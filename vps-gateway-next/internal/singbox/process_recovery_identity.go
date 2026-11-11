package singbox

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

var ErrRuntimeProcessOwnership = errors.New("runtime process ownership cannot be verified")

// This identity proves a locally spawned child, not its loaded rules/assets.
// The startup factory still needs source/asset and live controller verification.
type spawnedProcessIdentity struct {
	pid        int
	birth      string
	executable os.FileInfo
}

func processBirth(pid int) (string, error) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	// comm is parenthesized and may itself contain spaces or parentheses.
	end := strings.LastIndex(string(raw), ") ")
	if end < 0 {
		return "", ErrRuntimeProcessOwnership
	}
	fields := strings.Fields(string(raw[end+2:]))
	if len(fields) < 20 || fields[0] == "Z" || fields[0] == "X" {
		return "", ErrRuntimeProcessOwnership
	}
	birth, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil || birth == 0 {
		return "", ErrRuntimeProcessOwnership
	}
	return fields[19], nil
}

func (p *Process) captureSpawnedIdentity(pid int) (*spawnedProcessIdentity, error) {
	if pid <= 1 || !filepath.IsAbs(p.binary) || !filepath.IsAbs(p.configPath) {
		return nil, ErrRuntimeProcessOwnership
	}
	birth, err := processBirth(pid)
	if err != nil {
		return nil, ErrRuntimeProcessOwnership
	}
	info, err := os.Stat(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return nil, ErrRuntimeProcessOwnership
	}
	identity := &spawnedProcessIdentity{pid: pid, birth: birth, executable: info}
	if !p.verifySpawnedIdentity(identity, pid) {
		return nil, ErrRuntimeProcessOwnership
	}
	return identity, nil
}

func (p *Process) verifySpawnedIdentity(identity *spawnedProcessIdentity, pid int) bool {
	if identity == nil || identity.pid != pid {
		return false
	}
	birth, err := processBirth(pid)
	if err != nil || birth != identity.birth {
		return false
	}
	actual, err := os.Stat(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil || !os.SameFile(actual, identity.executable) {
		return false
	}
	expected, err := os.Stat(p.binary)
	if err != nil || !os.SameFile(actual, expected) {
		return false
	}
	argv, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil || !bytes.Equal(argv, []byte(strings.Join([]string{p.binary, "run", "-C", p.configPath, ""}, "\x00"))) {
		return false
	}
	// Repeat birth inspection after the separate /proc reads.
	after, err := processBirth(pid)
	return err == nil && after == birth
}

func (p *Process) matchesSpawnedIdentity(pid int) bool {
	return p.verifySpawnedIdentity(p.spawnedIdentity.Load(), pid)
}

// Bind a signal to a kernel process handle before checking /proc identity.
// If the child exits after verification, the handle cannot signal a recycled
// numeric PID. Unsupported kernels refuse this mode rather than falling back.
func (p *Process) signalSpawnedProcess(pid int, sig syscall.Signal) error {
	fd, err := unix.PidfdOpen(pid, 0)
	if err != nil {
		return fmt.Errorf("%w: open process handle", ErrRuntimeProcessOwnership)
	}
	defer unix.Close(fd)
	if !p.matchesSpawnedIdentity(pid) {
		return ErrRuntimeProcessOwnership
	}
	return unix.PidfdSendSignal(fd, unix.Signal(sig), nil, 0)
}

// Called under startMu before logs, PID files or lifecycle actions change.
// A missing/dead PID permits a fresh start; unreadable or malformed state and
// every unverified live PID refuse. Never erase evidence to make admission pass.
func (p *Process) checkRecoveryProcessAdmission() error {
	if !p.requireSpawnedIdentity {
		return nil
	}
	// A lost/replaced PID record cannot erase a child we still know is live.
	// In particular, do not spawn a duplicate over that child's active config.
	liveChild := 0
	if identity := p.spawnedIdentity.Load(); identity != nil {
		probe := syscall.Kill(identity.pid, 0)
		if probe == nil || errors.Is(probe, syscall.EPERM) {
			liveChild = identity.pid
		} else if !errors.Is(probe, syscall.ESRCH) {
			return ErrRuntimeProcessOwnership
		}
	}
	info, err := os.Lstat(p.pidPath)
	if os.IsNotExist(err) {
		if liveChild != 0 {
			return ErrRuntimeProcessOwnership
		}
		return nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 32 {
		return ErrRuntimeProcessOwnership
	}
	pid, err := p.readPID()
	if err != nil || pid <= 0 {
		return ErrRuntimeProcessOwnership
	}
	if liveChild != 0 && pid != liveChild {
		return ErrRuntimeProcessOwnership
	}
	aliveErr := syscall.Kill(pid, 0)
	if errors.Is(aliveErr, syscall.ESRCH) {
		return nil
	}
	if aliveErr != nil && !errors.Is(aliveErr, syscall.EPERM) {
		return ErrRuntimeProcessOwnership
	}
	if !p.matchesSpawnedIdentity(pid) {
		return ErrRuntimeProcessOwnership
	}
	return nil
}
