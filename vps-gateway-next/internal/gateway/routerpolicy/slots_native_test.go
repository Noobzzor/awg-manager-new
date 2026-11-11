package routerpolicy

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/gateway/policy"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

type nativeOwnerValidator struct{ binary string }

func (v nativeOwnerValidator) Validate(ctx context.Context, dir string) error {
	return exec.CommandContext(ctx, v.binary, "check", "-C", dir).Run()
}

// Connect the actual orchestrator lifecycle to the isolated real engine.
type nativeOwnerProcess struct {
	backend *nativeApplyBackend
	ctx     context.Context
}

func (p nativeOwnerProcess) IsRunning() (bool, int) {
	if p.backend.command == nil || p.backend.command.Process == nil {
		return false, 0
	}
	process := p.backend.command.Process
	return process.Signal(syscall.Signal(0)) == nil, process.Pid
}
func (p nativeOwnerProcess) Start() error  { return p.backend.Activate(p.ctx) }
func (p nativeOwnerProcess) Reload() error { return p.backend.Activate(p.ctx) }
func (p nativeOwnerProcess) Stop() error   { p.backend.stop(); return nil }

// Real-engine order control. Only the generated TUN inbound is replaced with
// loopback mixed inbounds; this is not the product's AWG/TUN packet path.
func TestNativeSplitProjectionOrder(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_OWNER_PROOF") != "1" {
		t.Skip("requires explicit isolated native order control")
	}
	binary := os.Getenv("SINGBOX_BIN")
	if binary == "" {
		t.Fatal("SINGBOX_BIN required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "owned-order-fixture") }))
	defer fixture.Close()
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config.d")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name string, raw []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(configDir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("00-base.json", []byte(`{"log":{"level":"error"},"outbounds":[{"type":"direct","tag":"direct"}]}`))
	b := &nativeApplyBackend{dir: dir, binary: binary}
	defer b.stop()
	cfg := router.NewEmptyConfig()
	cfg.Route.Rules = []router.Rule{{Network: "tcp", Action: "reject"}}
	slots, err := RenderSlots("awg0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	write("18-z-gateway-policy.json", slots.Ingress)
	write("20-router.json", slots.Routing)
	dnsRoute := []byte(`{"route":{"rules":[{"inbound":["awg-gateway-in"],"action":"route","outbound":"direct"}]}}`)
	write("19-dns-routes.json", dnsRoute)
	if output, err := exec.CommandContext(ctx, binary, "check", "-C", configDir).CombinedOutput(); err != nil {
		t.Fatalf("generated synthetic TUN split schema rejected: %v: %s", err, output)
	}
	orch := orchestrator.NewWithAppliedPath(configDir, nativeOwnerProcess{backend: b, ctx: ctx}, filepath.Join(dir, "orchestrator-applied.json"))
	defer orch.Close()
	for _, meta := range orchestrator.KnownSlots() {
		if err := orch.Register(meta); err != nil {
			t.Fatal(err)
		}
	}
	if err := orch.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	orch.SetValidator(nativeOwnerValidator{binary})
	res, err := orch.CheckMergedBatch(ctx, map[orchestrator.Slot]orchestrator.SlotCandidate{
		orchestrator.SlotGatewayPolicy: {Data: slots.Ingress, Enabled: true}, orchestrator.SlotRouter: {Data: slots.Routing, Enabled: true},
	})
	if err != nil || !res.Ok() {
		t.Fatalf("joint projected native candidate rejected: %v %s", err, res.Error())
	}
	res, err = orch.CheckMergedBatch(ctx, map[orchestrator.Slot]orchestrator.SlotCandidate{
		orchestrator.SlotRouter: {Data: []byte(`{"outbounds":[{"type":"synthetic-invalid-type","tag":"fixture-bad"}]}`), Enabled: true},
	})
	if err != nil || res.Ok() {
		t.Fatalf("invalid native candidate accepted: %v %s", err, res.Error())
	}
	current, _, err := orch.SnapshotSlot(orchestrator.SlotRouter)
	if err != nil || string(current) != string(slots.Routing) {
		t.Fatal("native candidate check mutated live router slot", err)
	}
	t.Log("joint native validation accepted projection and refused invalid outbound without writes")
	var ingress map[string]json.RawMessage
	if err := json.Unmarshal(slots.Ingress, &ingress); err != nil {
		t.Fatal(err)
	}
	ingress["inbounds"], err = json.Marshal([]map[string]any{
		{"type": "mixed", "tag": policy.GatewayInboundTag, "listen": "127.0.0.1", "listen_port": 10880},
		{"type": "mixed", "tag": "unrelated-in", "listen": "127.0.0.1", "listen_port": 10881},
	})
	if err != nil {
		t.Fatal(err)
	}
	mixed, err := json.Marshal(ingress)
	if err != nil {
		t.Fatal(err)
	}
	write("18-z-gateway-policy.json", mixed)
	probe := func(port string, want int) {
		t.Helper()
		proxy, _ := url.Parse("http://127.0.0.1:" + port)
		transport := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: time.Second}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fixture.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 100))
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != want || (want == 200 && string(body) != "owned-order-fixture") {
			t.Fatalf("port%s: HTTP%d want%d", port, resp.StatusCode, want)
		}
	}
	phase := func(name string, want int, changes map[orchestrator.Slot]orchestrator.SlotCandidate) {
		t.Helper()
		res, err := orch.CheckMergedBatch(ctx, changes)
		if err != nil || !res.Ok() {
			t.Fatalf("phase candidate rejected: %v %s", err, res.Error())
		}
		lease, err := orch.AcquireRuntimeLease(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Close()
		for slot, candidate := range changes {
			if err := orch.RestoreSlot(slot, candidate.Data, candidate.Enabled); err != nil {
				t.Fatal(err)
			}
		}
		if err := lease.ReloadNow(); err != nil {
			t.Fatal(err)
		}
		probe("10880", want)
		probe("10881", 200)
		t.Log(name)
	}
	phase("19 DNS-route precedes20 editor BLOCK; unrelated ingress retained", 200, nil)
	phase("without19, editor BLOCK rejects gateway only", 502, map[orchestrator.Slot]orchestrator.SlotCandidate{orchestrator.SlotDNSRoutes: {Enabled: false}})
	cfg.Route.Rules = []router.Rule{{Network: "tcp", Action: "route", Outbound: "direct"}, {Network: "tcp", Action: "reject"}}
	slots, err = RenderSlots("awg0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	phase("first editor DIRECT wins over later BLOCK", 200, map[orchestrator.Slot]orchestrator.SlotCandidate{orchestrator.SlotRouter: {Data: slots.Routing, Enabled: true}})
	cfg.Route.Rules[0], cfg.Route.Rules[1] = cfg.Route.Rules[1], cfg.Route.Rules[0]
	slots, err = RenderSlots("awg0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	phase("reordered first editor BLOCK wins over later DIRECT", 502, map[orchestrator.Slot]orchestrator.SlotCandidate{orchestrator.SlotRouter: {Data: slots.Routing, Enabled: true}})
	phase("restored19 DNS-route wins again", 200, map[orchestrator.Slot]orchestrator.SlotCandidate{orchestrator.SlotDNSRoutes: {Data: dnsRoute, Enabled: true}})
	t.Log("native orchestrator runtime lease lifecycle controls passed")
	t.Log("native split precedence controls passed")
}
