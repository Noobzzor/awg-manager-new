package singbox

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

type nativeLifecycleValidator struct{ binary string }

func (v nativeLifecycleValidator) Validate(ctx context.Context, dir string) error {
	return exec.CommandContext(ctx, v.binary, "check", "-C", dir).Run()
}

// Actual Process and orchestrator against an isolated real engine. Loopback
// mixed ingress deliberately avoids AWG/TUN/NET_ADMIN and external traffic.
func TestNativeProcessLeaseWithOrchestrator(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_PROCESS_LEASE_PROOF") != "1" {
		t.Skip("requires isolated native process lifecycle control")
	}
	binary := os.Getenv("SINGBOX_BIN")
	if binary == "" {
		t.Fatal("SINGBOX_BIN required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "owned-process-fixture") }))
	defer fixture.Close()
	dir := t.TempDir()
	config := filepath.Join(dir, "config.d")
	if err := os.MkdirAll(config, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(config, "00-base.json"), []byte(`{"log":{"level":"error"},"outbounds":[{"type":"direct","tag":"direct"}],"experimental":{"clash_api":{"external_controller":"127.0.0.1:10999"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	direct := []byte(`{"inbounds":[{"type":"mixed","tag":"owned-process-in","listen":"127.0.0.1","listen_port":10880}],"route":{"rules":[{"inbound":["owned-process-in"],"network":"tcp","outbound":"direct"}]}}`)
	block := []byte(`{"inbounds":[{"type":"mixed","tag":"owned-process-in","listen":"127.0.0.1","listen_port":10880}],"route":{"rules":[{"inbound":["owned-process-in"],"network":"tcp","action":"reject"}]}}`)
	if err := os.WriteFile(filepath.Join(config, "20-router.json"), direct, 0600); err != nil {
		t.Fatal(err)
	}
	p := NewProcess(binary, config, filepath.Join(dir, "engine.pid"))
	p.logDir = dir
	p.ReloadNeedsRestart = func() bool { return true }
	t.Cleanup(func() { _ = p.Stop(); p.Close() })
	var writer heavyop.Gate
	writer.Lock()
	defer writer.Unlock()
	o := orchestrator.NewWithAppliedPath(config, p, filepath.Join(dir, "orch-applied.json"))
	defer o.Close()
	o.SetOperationCoordinator(&writer)
	for _, meta := range orchestrator.KnownSlots() {
		if err := o.Register(meta); err != nil {
			t.Fatal(err)
		}
	}
	if err := o.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	o.SetValidator(nativeLifecycleValidator{binary})
	probe := func(want int) error {
		proxy, _ := url.Parse("http://127.0.0.1:10880")
		transport := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: time.Second}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fixture.URL, nil)
		if err != nil {
			return err
		}
		response, err := client.Do(req)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		body, err := io.ReadAll(io.LimitReader(response.Body, 100))
		if err != nil {
			return err
		}
		if response.StatusCode != want || (want == 200 && string(body) != "owned-process-fixture") {
			return fmt.Errorf("HTTP%d wanted%d", response.StatusCode, want)
		}
		return nil
	}
	queued, cancelQueued := context.WithTimeout(ctx, 30*time.Millisecond)
	err := o.ReloadContext(queued)
	cancelQueued()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("ordinary native reload escaped writer admission", err)
	}
	if running, _ := p.IsRunning(); running {
		t.Fatal("cancelled ordinary reload started engine")
	}
	t.Log("NATIVE_WRITER ordinary reload admission cancelled without engine start")
	lease, err := o.AcquireRuntimeLease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if err := lease.ReloadNow(); err != nil {
		t.Fatal(err)
	}
	if err := probe(200); err != nil {
		t.Fatal(err)
	}
	t.Log("NATIVE_PROCESS actual Process starts DIRECT under orchestrator lease")
	done := make(chan error, 1)
	go func() { done <- p.Stop() }()
	select {
	case err := <-done:
		t.Fatal("public stop escaped active transaction", err)
	case <-time.After(50 * time.Millisecond):
	}
	if running, _ := p.IsRunning(); !running {
		t.Fatal("queued stop killed running engine")
	}
	for _, phase := range []struct {
		raw  []byte
		want int
	}{{block, 502}, {direct, 200}} {
		result, err := o.CheckMergedBatchNative(ctx, map[orchestrator.Slot]orchestrator.SlotCandidate{orchestrator.SlotRouter: {Data: phase.raw, Enabled: true}})
		if err != nil || !result.Ok() {
			t.Fatal("native candidate refused", err, result.Error())
		}
		if err := o.RestoreSlot(orchestrator.SlotRouter, phase.raw, true); err != nil {
			t.Fatal(err)
		}
		if err := lease.ReloadNow(); err != nil {
			t.Fatal(err)
		}
		if err := probe(phase.want); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("NATIVE_PROCESS real stop/start BLOCK then DIRECT completes while public stop waits")
	o.SetRuntimeRecoveryRequired(true)
	lease.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("queued stop did not resume")
	}
	if err := p.Start(); !errors.Is(err, orchestrator.ErrRuntimeRecoveryRequired) {
		t.Fatal("direct process restart bypassed recovery barrier", err)
	}
	recovery, err := o.AcquireRuntimeLease(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer recovery.Close()
	if err := recovery.ReloadNow(); err != nil {
		t.Fatal(err)
	}
	if err := probe(200); err != nil {
		t.Fatal(err)
	}
	o.SetRuntimeRecoveryRequired(false)
	recovery.Close()
	op := &Operator{proc: p, binary: binary, operationCoordinator: &writer, configPath: config, clash: NewClashClient("127.0.0.1:10999"), log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if spawned, err := op.startAndWait(ctx); err != nil || spawned {
		t.Fatal("real Operator readiness failed or duplicated running process", err)
	}
	t.Log("NATIVE_PROCESS actual Operator readiness accepts the healthy current generation without a duplicate spawn")
	queuedStop, cancelStop := context.WithTimeout(ctx, 30*time.Millisecond)
	err = op.Control(queuedStop, "stop")
	cancelStop()
	if !errors.Is(err, context.DeadlineExceeded) || op.IsManuallyStopped() {
		t.Fatal("queued native manual stop changed intent", err)
	}
	if running, _ := p.IsRunning(); !running {
		t.Fatal("queued native manual stop killed engine")
	}
	t.Log("NATIVE_WRITER manual stop admission cancelled; engine and intent preserved")
	t.Log("native actual Process lease exclusion and recovery controls passed")
}
