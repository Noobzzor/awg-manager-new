package routerpolicy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

// This is a real-engine control backend, not the product orchestrator adapter.
// The generated TUN inbound is replaced by a loopback mixed inbound so this
// test needs no network, NET_ADMIN, TUN device or actual client credentials.
type nativeApplyBackend struct {
	dir, binary, fixture        string
	command                     *exec.Cmd
	done                        chan error
	logs                        bytes.Buffer
	failActivation, crashCommit bool
}

func (b *nativeApplyBackend) Validate(ctx context.Context, s ApplyState) error {
	dir, err := os.MkdirTemp(b.dir, "check-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := storage.AtomicWrite(filepath.Join(dir, "00-base.json"), []byte(`{"log":{"level":"error"},"outbounds":[{"type":"direct","tag":"direct"}]}`)); err != nil {
		return err
	}
	if err := storage.AtomicWrite(filepath.Join(dir, "18-z-gateway-policy.json"), s.Runtime); err != nil {
		return err
	}
	if err := exec.CommandContext(ctx, b.binary, "check", "-C", dir).Run(); err != nil {
		return fmt.Errorf("native candidate check rejected: %w", err)
	}
	return nil
}
func (b *nativeApplyBackend) Snapshot(context.Context) (ApplyState, error) {
	editor, err := os.ReadFile(filepath.Join(b.dir, "editor.json"))
	if err != nil {
		return ApplyState{}, err
	}
	runtime, err := os.ReadFile(filepath.Join(b.dir, "config.d/18-z-gateway-policy.json"))
	if err != nil {
		return ApplyState{}, err
	}
	return ApplyState{Editor: editor, Runtime: runtime, Enabled: true}, nil
}
func (b *nativeApplyBackend) Commit(_ context.Context, s ApplyState) error {
	if err := storage.AtomicWritePerm(filepath.Join(b.dir, "editor.json"), s.Editor, 0600); err != nil {
		return err
	}
	if err := storage.AtomicWritePerm(filepath.Join(b.dir, "config.d/18-z-gateway-policy.json"), s.Runtime, 0600); err != nil {
		return err
	}
	if b.crashCommit {
		b.crashCommit = false
		panic("simulated native process loss after durable files")
	}
	return nil
}
func (b *nativeApplyBackend) Restore(ctx context.Context, s ApplyState) error {
	return b.Commit(ctx, s)
}

func (b *nativeApplyBackend) stop() {
	if b.command == nil {
		return
	}
	_ = b.command.Process.Signal(syscall.SIGTERM)
	select {
	case <-b.done:
	case <-time.After(3 * time.Second):
		_ = b.command.Process.Kill()
		<-b.done
	}
	b.command = nil
}
func (b *nativeApplyBackend) Activate(ctx context.Context) error {
	b.stop()
	if b.failActivation {
		b.failActivation = false
		return fixtureApplyError
	}
	b.logs.Reset()
	command := exec.Command(b.binary, "run", "-C", filepath.Join(b.dir, "config.d"))
	command.Stdout = &b.logs
	command.Stderr = &b.logs
	if err := command.Start(); err != nil {
		return err
	}
	b.command = command
	b.done = make(chan error, 1)
	go func() { b.done <- command.Wait() }()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		conn, err := net.DialTimeout("tcp", "127.0.0.1:10880", 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		select {
		case err := <-b.done:
			b.command = nil
			return fmt.Errorf("native engine exited before readiness: %v", err)
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("native engine readiness timed out")
		case <-ticker.C:
		}
	}
}

func (b *nativeApplyBackend) Confirm(ctx context.Context, want ApplyState) error {
	got, err := b.Snapshot(ctx)
	if err != nil {
		return err
	}
	if !bytes.Equal(got.Editor, want.Editor) || !bytes.Equal(got.Runtime, want.Runtime) {
		return errors.New("native files differ from candidate")
	}
	var cfg router.RouterConfig
	if err := json.Unmarshal(want.Editor, &cfg); err != nil {
		return err
	}
	proxy, _ := url.Parse("http://127.0.0.1:10880")
	transport := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.fixture, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if cfg.Route.Final == "block" {
		if resp.StatusCode != http.StatusBadGateway {
			return fmt.Errorf("expected BLOCK, HTTP status=%d", resp.StatusCode)
		}
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 100))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 || string(body) != "owned-apply-fixture" {
		return errors.New("DIRECT did not reach owned fixture")
	}
	return nil
}

func nativeApplyState(t *testing.T, final string) ApplyState {
	t.Helper()
	cfg := router.NewEmptyConfig()
	cfg.Route.Final = final
	editor, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Render("awg0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	var mixed map[string]any
	if err := json.Unmarshal(raw, &mixed); err != nil {
		t.Fatal(err)
	}
	mixed["inbounds"] = []any{map[string]any{"type": "mixed", "tag": "awg-gateway-in", "listen": "127.0.0.1", "listen_port": 10880}}
	raw, err = json.Marshal(mixed)
	if err != nil {
		t.Fatal(err)
	}
	return ApplyState{Editor: editor, Runtime: raw, Enabled: true}
}

// Journal control through the real orchestrator lifecycle. The storage and
// HTTP confirmation remain explicit test fixtures, not the product adapter.
type nativeLeasedApplyBackend struct {
	*nativeApplyBackend
	orch  *orchestrator.Orchestrator
	lease *orchestrator.RuntimeLease
}

func (b *nativeLeasedApplyBackend) AcquireApplyLease(ctx context.Context) (func(), error) {
	lease, err := b.orch.AcquireRuntimeLease(ctx)
	if err != nil {
		return nil, err
	}
	b.lease = lease
	return func() { lease.Close(); b.lease = nil }, nil
}
func (b *nativeLeasedApplyBackend) Activate(context.Context) error {
	if b.lease == nil {
		return errors.New("native activation outside runtime lease")
	}
	return b.lease.ReloadNow()
}

func leasedNativeBackend(t *testing.T, ctx context.Context, base *nativeApplyBackend) ApplyBackend {
	t.Helper()
	o := orchestrator.NewWithAppliedPath(filepath.Join(base.dir, "config.d"), nativeOwnerProcess{backend: base, ctx: ctx}, filepath.Join(base.dir, "orchestrator-applied.json"))
	t.Cleanup(o.Close)
	for _, meta := range orchestrator.KnownSlots() {
		if err := o.Register(meta); err != nil {
			t.Fatal(err)
		}
	}
	if err := o.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	o.SetValidator(nativeOwnerValidator{base.binary})
	return &nativeLeasedApplyBackend{nativeApplyBackend: base, orch: o}
}

func TestNativeConfirmedApplyAndRecovery(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_APPLY_PROOF") != "1" {
		t.Skip("requires explicit isolated real sing-box control")
	}
	binary := os.Getenv("SINGBOX_BIN")
	if binary == "" {
		t.Fatal("SINGBOX_BIN required")
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "owned-apply-fixture") }))
	defer fixture.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	base := filepath.Join(dir, "config.d/00-base.json")
	if err := storage.AtomicWrite(base, []byte(`{"log":{"level":"error"},"outbounds":[{"type":"direct","tag":"direct"}]}`)); err != nil {
		t.Fatal(err)
	}
	b := &nativeApplyBackend{dir: dir, binary: binary, fixture: fixture.URL}
	defer func() { b.stop() }()
	direct, block := nativeApplyState(t, "direct"), nativeApplyState(t, "block")
	if err := b.Commit(ctx, direct); err != nil {
		t.Fatal(err)
	}
	if err := b.Activate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := b.Confirm(ctx, direct); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_APPLY baseline DIRECT reached fixture")
	path := filepath.Join(dir, "router-apply.json")
	guard := &sync.Mutex{}
	a, err := NewApplier(path, leasedNativeBackend(t, ctx, b), guard)
	if err != nil {
		t.Fatal(err)
	}
	invalid := nativeApplyState(t, "missing-outbound-fixture")
	if out, err := a.Apply(ctx, invalid); err == nil || out.Applied {
		t.Fatal("native validation failure was acknowledged")
	}
	if err := b.Confirm(ctx, direct); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_APPLY invalid candidate rejected; DIRECT retained")
	b.failActivation = true
	if out, err := a.Apply(ctx, block); !errors.Is(err, fixtureApplyError) || out.Applied {
		t.Fatal("failed activation was acknowledged", err)
	}
	if err := b.Confirm(ctx, direct); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_APPLY activation failure restored DIRECT")
	b.crashCommit = true
	crashed := false
	func() {
		defer func() {
			if recover() != nil {
				crashed = true
			}
		}()
		_, _ = a.Apply(ctx, block)
	}()
	if !crashed {
		t.Fatal("prepared interruption did not run")
	}
	b.stop()
	b = &nativeApplyBackend{dir: dir, binary: binary, fixture: fixture.URL}
	a, err = NewApplier(path, leasedNativeBackend(t, ctx, b), guard)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if err := b.Confirm(ctx, direct); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_APPLY prepared journal/fresh backend restored DIRECT")
	a.journal = journalFixture{applyJournalStore: a.journal, afterStore: func(j applyJournal) {
		if j.Phase == "accepted" {
			panic("simulated loss after native acceptance")
		}
	}}
	crashed = false
	func() {
		defer func() {
			if recover() != nil {
				crashed = true
			}
		}()
		_, _ = a.Apply(ctx, block)
	}()
	if !crashed {
		t.Fatal("accepted interruption did not run")
	}
	b.stop()
	b = &nativeApplyBackend{dir: dir, binary: binary, fixture: fixture.URL}
	a, err = NewApplier(path, leasedNativeBackend(t, ctx, b), guard)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if err := b.Confirm(ctx, block); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_APPLY accepted journal/fresh backend retained BLOCK")
	if out, err := a.Apply(ctx, direct); err != nil || !out.Applied {
		t.Fatal("DIRECT restoration failed", err)
	}
	if err := b.Confirm(ctx, direct); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_APPLY final DIRECT restored; all controls passed")
	t.Log("native journal recovery through actual orchestrator runtime lease passed")
}
