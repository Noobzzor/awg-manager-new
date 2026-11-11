package singbox

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

// OwnedGenerationProcess is the original local composition's process boundary.
// Canonical fragments are never the live child's input. Each leased activation
// merges/checks a private document and its assets before stopping the old child.
// Public mutations refuse; only the orchestrator's explicit RuntimeLease runs it.
type OwnedGenerationProcess struct {
	p            *Process
	source, root string
	clash        *ClashClient
	verify       func(context.Context) error
	mu           sync.Mutex
	generation   ownedPreparedGeneration
}

type ownedPreparedGeneration struct {
	RuntimeGeneration
	path   string
	assets map[string][sha256.Size]byte
}

func NewOwnedGenerationProcess(p *Process, source, root string, clash *ClashClient, verify func(context.Context) error) (*OwnedGenerationProcess, error) {
	if p == nil || clash == nil || clash.Address() != ClashAddr(DefaultClashPort) || verify == nil || !p.requireSpawnedIdentity || !filepath.IsAbs(source) || !filepath.IsAbs(root) {
		return nil, fmt.Errorf("owned generation requires cold process, absolute roots and real verification")
	}
	if running, _ := p.IsRunning(); running {
		return nil, ErrRuntimeProcessOwnership
	}
	p.logDir = filepath.Join(root, "logs")
	return &OwnedGenerationProcess{p: p, source: source, root: root, clash: clash, verify: verify}, nil
}
func (g *OwnedGenerationProcess) IsRunning() (bool, int) { return g.p.IsRunning() }
func (g *OwnedGenerationProcess) Start() error           { return orchestrator.ErrRuntimeRecoveryRequired }
func (g *OwnedGenerationProcess) Stop() error            { return orchestrator.ErrRuntimeRecoveryRequired }
func (g *OwnedGenerationProcess) Reload() error          { return orchestrator.ErrRuntimeRecoveryRequired }

// Do not clear the underlying cold Process barrier: direct Operator calls must
// remain unavailable in this deliberately restricted original composition.
func (g *OwnedGenerationProcess) SetRuntimeRecoveryRequired(bool) {
	g.p.SetRuntimeRecoveryRequired(true)
}
func (g *OwnedGenerationProcess) AcquireProcessLease(ctx context.Context) (orchestrator.ProcessLease, error) {
	l, err := g.p.AcquireProcessLease(ctx)
	if err != nil {
		return nil, err
	}
	return &ownedGenerationLease{g: g, inner: l}, nil
}

type ownedGenerationLease struct {
	g     *OwnedGenerationProcess
	inner orchestrator.ProcessLease
}

func (l *ownedGenerationLease) IsRunning() (bool, int)      { return l.inner.IsRunning() }
func (l *ownedGenerationLease) Close()                      { l.inner.Close() }
func (l *ownedGenerationLease) Stop() error                 { return l.inner.Stop() }
func (l *ownedGenerationLease) Start() error                { _, err := l.StartSpawned(); return err }
func (l *ownedGenerationLease) Reload() error               { _, err := l.activate(); return err }
func (l *ownedGenerationLease) StartSpawned() (bool, error) { return l.activate() }
func (l *ownedGenerationLease) activate() (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	g := l.g
	if err := g.verify(ctx); err != nil {
		return false, err
	}
	generation, err := g.prepare(ctx)
	if err != nil {
		return false, err
	}
	if err = l.inner.Stop(); err != nil {
		return false, fmt.Errorf("stop previous prepared engine: %w", err)
	}
	// The mutation lease excludes every Process caller; Stop joined the child.
	g.p.configPath = filepath.Dir(generation.path)
	spawned, err := l.inner.StartSpawned()
	if err != nil {
		return false, fmt.Errorf("start prepared engine: %w", err)
	}
	g.mu.Lock()
	g.generation = generation
	g.mu.Unlock()
	if err = g.Confirm(ctx); err != nil {
		return spawned, errors.Join(err, l.inner.Stop())
	}
	return spawned, nil
}

// No external input files, background downloads or mutable cache references in
// the first local slice. Unsupported candidates fail before process mutation.
func assetFreeGeneration(v any) bool {
	switch x := v.(type) {
	case map[string]any:
		for k, c := range x {
			switch k {
			case "path", "certificate_path", "key_path", "output":
				if s, ok := c.(string); ok && s != "" {
					return false
				}
			case "rule_set":
				// References in matchers are allowed. Declarations are
				// separately admitted and removed from this scan.
				if a, ok := c.([]any); ok {
					for _, ref := range a {
						if _, ok := ref.(string); !ok {
							return false
						}
					}
				}
			case "cache_file":
				if m, ok := c.(map[string]any); ok {
					if enabled, _ := m["enabled"].(bool); enabled {
						return false
					}
				}
			}
			if !assetFreeGeneration(c) {
				return false
			}
		}
	case []any:
		for _, c := range x {
			if !assetFreeGeneration(c) {
				return false
			}
		}
	}
	return true
}
func (g *OwnedGenerationProcess) prepare(ctx context.Context) (ownedPreparedGeneration, error) {
	fail := func() (ownedPreparedGeneration, error) {
		return ownedPreparedGeneration{}, fmt.Errorf("prepare owned runtime generation failed")
	}
	if err := os.MkdirAll(g.root, 0700); err != nil {
		return fail()
	}
	dir, err := os.MkdirTemp(g.root, "generation-")
	if err != nil {
		return fail()
	}
	ready := false
	defer func() {
		if !ready {
			_ = os.RemoveAll(dir)
		}
	}()
	mergeFile := filepath.Join(dir, "merged.tmp")
	cmd := exec.CommandContext(ctx, g.p.binary, "merge", mergeFile, "-C", g.source)
	if err = cmd.Run(); err != nil {
		return fail()
	}
	raw, err := os.ReadFile(mergeFile)
	if err != nil {
		return fail()
	}
	doc, err := decodeGenerationJSON(raw)
	if err != nil {
		return fail()
	}
	assets, err := g.copyRuleSetAssets(doc, dir)
	if err != nil {
		return ownedPreparedGeneration{}, err
	}
	raw, err = json.Marshal(doc)
	if err != nil {
		return fail()
	}
	generation, err := PrepareRuntimeGeneration(raw)
	if err != nil {
		return fail()
	}
	// Distinct witness even when a prior configuration is applied again.
	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		return fail()
	}
	digest := sha256.Sum256(append(append([]byte{}, generation.Config...), nonce...))
	tag := runtimeGenerationPrefix + hex.EncodeToString(digest[:])
	generation.Config = bytes.Replace(generation.Config, []byte(generation.Tag), []byte(tag), 1)
	generation.Tag = tag
	file := filepath.Join(dir, "config.json")
	if err := writeOwnedGenerationFile(file, generation.Config); err != nil {
		return fail()
	}
	if err = os.Remove(mergeFile); err != nil {
		return fail()
	}
	check := exec.CommandContext(ctx, g.p.binary, "check", "-c", file)
	if err = check.Run(); err != nil {
		return fail()
	}
	if err = syncOwnedGenerationDir(dir); err != nil {
		return fail()
	}
	ready = true
	return ownedPreparedGeneration{RuntimeGeneration: generation, path: file, assets: assets}, nil
}

// Confirm binds the live controller to the exact child and immutable document,
// not just a successful reload or the presence of a marker on an arbitrary port.
func (g *OwnedGenerationProcess) Confirm(ctx context.Context) error {
	if err := g.verify(ctx); err != nil {
		return err
	}
	g.mu.Lock()
	generation := g.generation
	g.mu.Unlock()
	path := generation.path
	running, pid := g.p.IsRunning()
	if !running || !g.p.matchesSpawnedIdentity(pid) || path == "" {
		return fmt.Errorf("confirm owned prepared child: %w", ErrRuntimeProcessOwnership)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return ErrRuntimeProcessOwnership
	}
	raw, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(raw, generation.Config) {
		return fmt.Errorf("owned prepared config changed")
	}
	if err := generation.verifyAssets(); err != nil {
		return err
	}
	if err = ownedControllerSocket(pid); err != nil {
		return err
	}
	if err = g.clash.VerifyRuntimeGeneration(ctx, generation.Tag); err != nil {
		return err
	}
	if !g.p.matchesSpawnedIdentity(pid) {
		return ErrRuntimeProcessOwnership
	}
	if err := generation.verifyAssets(); err != nil {
		return err
	}
	return ownedControllerSocket(pid)
}

func ownedControllerSocket(pid int) error {
	// This restricted composition fixes the controller to the standard loopback
	// endpoint; it never probes or authenticates an unrelated configurable URL.
	raw, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return ErrRuntimeProcessOwnership
	}
	address := fmt.Sprintf("0100007F:%04X", DefaultClashPort)
	inode := ""
	for _, line := range strings.Split(string(raw), "\n") {
		f := strings.Fields(line)
		if len(f) > 9 && f[1] == address && f[3] == "0A" {
			if inode != "" {
				return ErrRuntimeProcessOwnership
			}
			inode = f[9]
		}
	}
	if inode == "" {
		return fmt.Errorf("prepared controller listener absent: %w", ErrRuntimeProcessOwnership)
	}
	fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
	if err != nil {
		return ErrRuntimeProcessOwnership
	}
	for _, fd := range fds {
		target, _ := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "fd", fd.Name()))
		if target == "socket:["+inode+"]" {
			return nil
		}
	}
	return fmt.Errorf("controller listener not owned by prepared child: %w", ErrRuntimeProcessOwnership)
}

// Refuse surviving engines before original startup writes any state. Fresh
// mode does not adopt or signal any pre-existing process, even the same binary.
func RefuseExistingEngine(binary string) error {
	expected, err := os.Stat(binary)
	if err != nil || !filepath.IsAbs(binary) {
		return ErrRuntimeProcessOwnership
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return ErrRuntimeProcessOwnership
	}
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		actual, err := os.Stat(filepath.Join("/proc", entry.Name(), "exe"))
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("inspect existing engine processes: %w", ErrRuntimeProcessOwnership)
		}
		if err == nil && os.SameFile(actual, expected) {
			return fmt.Errorf("existing engine process: %w", ErrRuntimeProcessOwnership)
		}
	}
	return nil
}
