package singbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

var errLeaseSpawnFixture = errors.New("owned fixture refused spawn")

func processLeaseFixture(t *testing.T) *Process {
	t.Helper()
	dir := t.TempDir()
	p := NewProcess("/owned/fixture/sing-box", dir, filepath.Join(dir, "engine.pid"))
	p.logDir = dir
	p.startCmd = func(string, ...string) (*exec.Cmd, error) { return nil, errLeaseSpawnFixture }
	return p
}

func TestProcessLeaseBlocksPublicMutations(t *testing.T) {
	for _, operation := range []string{"start", "spawn", "stop", "reload", "attach"} {
		t.Run(operation, func(t *testing.T) {
			p := processLeaseFixture(t)
			lease, err := p.AcquireProcessLease(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			entered := make(chan struct{})
			done := make(chan error, 1)
			go func() {
				close(entered)
				var err error
				switch operation {
				case "start":
					err = p.Start()
				case "spawn":
					_, err = p.StartSpawned()
				case "stop":
					err = p.Stop()
				case "reload":
					err = p.Reload()
				case "attach":
					p.AttachIfRunning()
				}
				done <- err
			}()
			<-entered
			select {
			case err := <-done:
				t.Fatal("public process mutation escaped held lease", err)
			case <-time.After(50 * time.Millisecond):
			}
			lease.Close()
			select {
			case err := <-done:
				if operation == "stop" || operation == "attach" {
					if err != nil {
						t.Fatal(err)
					}
				} else if !errors.Is(err, errLeaseSpawnFixture) {
					t.Fatal("queued mutation did not reach fixture", err)
				}
			case <-time.After(time.Second):
				t.Fatal("public process mutation did not resume")
			}
		})
	}
}

func TestProcessPublicMutationDoesNotReacquireGlobalGate(t *testing.T) {
	heavyop.Default.Lock()
	defer heavyop.Default.Unlock()
	p := processLeaseFixture(t)
	done := make(chan error, 1)
	go func() { done <- p.Start() }()
	select {
	case err := <-done:
		if !errors.Is(err, errLeaseSpawnFixture) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("legacy global-gate caller deadlocked on process start")
	}
}

func TestOperatorStartWaitCancellationWhileProcessLeased(t *testing.T) {
	p := processLeaseFixture(t)
	first, err := p.AcquireProcessLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	op := &Operator{proc: p}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { _, err := op.startAndWait(ctx); done <- err }()
	defer func() {
		first.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("cancelled waiting start lost context", err)
		}
		done <- err
	case <-time.After(100 * time.Millisecond):
		t.Fatal("cancelled Operator start remained blocked on process gate")
	}
}

func TestOperatorStartWaitRetainsRecoveryBarrier(t *testing.T) {
	p := processLeaseFixture(t)
	p.SetRuntimeRecoveryRequired(true)
	op := &Operator{proc: p}
	if spawned, err := op.startAndWait(context.Background()); spawned || !errors.Is(err, orchestrator.ErrRuntimeRecoveryRequired) {
		t.Fatal("ordinary readiness startup bypassed recovery barrier", err)
	}
}

func TestOperatorReadinessRetainsProcessLease(t *testing.T) {
	child := exec.Command("sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = child.Process.Kill(); _ = child.Wait() })
	p := processLeaseFixture(t)
	p.matchBinaryFn = func(pid int) bool { return pid == child.Process.Pid }
	if err := p.writePID(child.Process.Pid); err != nil {
		t.Fatal(err)
	}
	entered, release := make(chan struct{}), make(chan struct{})
	var enteredOnce, releaseOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		enteredOnce.Do(func() { close(entered) })
		<-release
		_, _ = io.WriteString(w, `{"version":"fixture"}`)
	}))
	defer server.Close()
	config := t.TempDir()
	if err := os.WriteFile(filepath.Join(config, "00-base.json"), []byte(`{"experimental":{"clash_api":{"external_controller":"127.0.0.1:9099"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	op := &Operator{proc: p, configPath: config, clash: NewClashClient(strings.TrimPrefix(server.URL, "http://")), log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	readyDone := make(chan struct{})
	var readyError error
	go func() { _, readyError = op.startAndWait(ctx); close(readyDone) }()
	defer func() {
		releaseOnce.Do(func() { close(release) })
		cancel()
		select {
		case <-readyDone:
		case <-time.After(2 * time.Second):
		}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("readiness fixture not reached")
	}
	type leaseResult struct {
		lease orchestrator.ProcessLease
		err   error
	}
	acquired := make(chan leaseResult, 1)
	go func() { lease, err := p.AcquireProcessLease(ctx); acquired <- leaseResult{lease, err} }()
	select {
	case result := <-acquired:
		if result.lease != nil {
			result.lease.Close()
		}
		t.Fatal("process transaction entered while older startup readiness was unresolved", result.err)
	case <-time.After(50 * time.Millisecond):
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-readyDone:
		if readyError != nil {
			t.Fatal(readyError)
		}
	case <-time.After(time.Second):
		t.Fatal("startup readiness did not finish")
	}
	select {
	case result := <-acquired:
		if result.err != nil {
			t.Fatal(result.err)
		}
		result.lease.Close()
	case <-time.After(time.Second):
		t.Fatal("lease not released after readiness")
	}
}

func TestProcessLeaseExplicitMutationAndExpiry(t *testing.T) {
	p := processLeaseFixture(t)
	lease, err := p.AcquireProcessLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	done := make(chan error, 1)
	go func() { done <- lease.Start() }()
	select {
	case err := <-done:
		if !errors.Is(err, errLeaseSpawnFixture) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("leased start waited on its own mutation gate")
	}
	lease.Close()
	lease.Close()
	if err := lease.Stop(); err == nil {
		t.Fatal("expired lease authorized process stop")
	}
}

func TestProcessLeaseAcquisitionCancellation(t *testing.T) {
	p := processLeaseFixture(t)
	first, err := p.AcquireProcessLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if lease, err := p.AcquireProcessLease(ctx); !errors.Is(err, context.Canceled) || lease != nil {
		if lease != nil {
			lease.Close()
		}
		t.Fatal("cancelled process lease accepted", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		lease, err := p.AcquireProcessLease(ctx)
		if lease != nil {
			lease.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatal("second lease bypassed first", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled lease acquisition remained blocked")
	}
}

func TestProcessLeaseRecoveryBarrierAllowsOnlyExplicitStart(t *testing.T) {
	p := processLeaseFixture(t)
	p.SetRuntimeRecoveryRequired(true)
	if err := p.Start(); !errors.Is(err, orchestrator.ErrRuntimeRecoveryRequired) {
		t.Fatal("ordinary start bypassed recovery barrier", err)
	}
	if err := p.Reload(); !errors.Is(err, orchestrator.ErrRuntimeRecoveryRequired) {
		t.Fatal("ordinary reload bypassed recovery barrier", err)
	}
	if err := p.Stop(); err != nil {
		t.Fatal("explicit stop must remain possible", err)
	}
	lease, err := p.AcquireProcessLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err := lease.Start(); !errors.Is(err, errLeaseSpawnFixture) {
		t.Fatal("explicit recovery blocked by its own barrier", err)
	}
	p.SetRuntimeRecoveryRequired(false)
	lease.Close()
	if err := p.Start(); !errors.Is(err, errLeaseSpawnFixture) {
		t.Fatal("successful recovery did not clear process barrier", err)
	}
}

func TestOrchestratorLeaseAlsoExcludesDirectProcessCalls(t *testing.T) {
	p := processLeaseFixture(t)
	dir := t.TempDir()
	o := orchestrator.NewWithAppliedPath(dir, p, filepath.Join(t.TempDir(), "applied.json"))
	defer o.Close()
	if err := o.Register(orchestrator.SlotMeta{Slot: orchestrator.SlotRouter, Filename: "20-router.json"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "20-router.json"), []byte(`{"outbounds":[{"type":"direct","tag":"direct"}],"route":{"rules":[{"network":"tcp","outbound":"direct"}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := o.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	lease, err := o.AcquireRuntimeLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	done := make(chan error, 1)
	go func() { done <- p.Start() }()
	select {
	case err := <-done:
		t.Fatal("direct process start escaped orchestrator lease", err)
	case <-time.After(50 * time.Millisecond):
	}
	reload := make(chan error, 1)
	go func() { reload <- lease.ReloadNow() }()
	select {
	case err := <-reload:
		if !errors.Is(err, errLeaseSpawnFixture) {
			t.Fatal("leased orchestrator did not activate process", err)
		}
	case <-time.After(time.Second):
		t.Fatal("orchestrator activation deadlocked on process lease")
	}
	o.SetRuntimeRecoveryRequired(true)
	lease.Close()
	select {
	case err := <-done:
		if !errors.Is(err, orchestrator.ErrRuntimeRecoveryRequired) {
			t.Fatal("queued process start escaped recovery barrier", err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued direct process call did not resume")
	}
}

func TestOrchestratorProcessLeaseQueuedCancellationReleasesBothGates(t *testing.T) {
	p := processLeaseFixture(t)
	first, err := p.AcquireProcessLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	o := orchestrator.NewWithAppliedPath(t.TempDir(), p, filepath.Join(t.TempDir(), "applied.json"))
	defer o.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		lease, err := o.AcquireRuntimeLease(ctx)
		if lease != nil {
			lease.Close()
		}
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatal("orchestrator bypassed existing process lease", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("queued orchestrator cancellation retained a gate")
	}
	first.Close()
	next, err := o.AcquireRuntimeLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
	if err := p.Start(); !errors.Is(err, errLeaseSpawnFixture) {
		t.Fatal("cancelled acquisition retained process gate", err)
	}
}

func TestProcessLeaseDoesNotBlockRuntimeReads(t *testing.T) {
	p := processLeaseFixture(t)
	lease, err := p.AcquireProcessLease(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	done := make(chan struct{})
	go func() { p.IsRunning(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("process lease blocked runtime status reads")
	}
}
