package singbox

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
	"testing"
	"time"
)

// Native library control only: not the product's owner/startup/AWG/TUN path.
func TestNativeRuntimeGenerationWitness(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_GENERATION_PROOF") != "1" {
		t.Skip("requires explicit isolated native generation control")
	}
	binary := os.Getenv("SINGBOX_BIN")
	if binary == "" {
		t.Fatal("SINGBOX_BIN required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	makeGeneration := func(block bool, level string) RuntimeGeneration {
		rules := []any{}
		if block {
			rules = append(rules, map[string]any{"action": "reject"})
		}
		doc := map[string]any{
			"log":          map[string]any{"level": level},
			"inbounds":     []any{map[string]any{"type": "mixed", "tag": "probe-in", "listen": "127.0.0.1", "listen_port": 10880}},
			"outbounds":    []any{map[string]any{"type": "direct", "tag": "direct"}},
			"route":        map[string]any{"rules": rules, "final": "direct"},
			"experimental": map[string]any{"clash_api": map[string]any{"external_controller": "127.0.0.1:10999"}},
		}
		raw, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		g, err := PrepareRuntimeGeneration(raw)
		if err != nil {
			t.Fatal(err)
		}
		return g
	}
	path := filepath.Join(t.TempDir(), "config.json")
	write := func(g RuntimeGeneration) {
		t.Helper()
		if err := os.WriteFile(path, g.Config, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var cmd *exec.Cmd
	stop := func() {
		if cmd != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			cmd = nil
		}
	}
	defer stop()
	start := func() {
		t.Helper()
		cmd = exec.CommandContext(ctx, binary, "run", "-c", path)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	clash := NewClashClient("127.0.0.1:10999")
	wait := func(g RuntimeGeneration) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			probeCtx, probeCancel := context.WithTimeout(ctx, 200*time.Millisecond)
			err := clash.VerifyRuntimeGeneration(probeCtx, g.Tag)
			probeCancel()
			if err == nil {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
		t.Fatal("native library did not observe the intended generation")
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "owned-generation-origin") }))
	defer origin.Close()
	proxy, _ := url.Parse("http://127.0.0.1:10880")
	transport := &http.Transport{Proxy: http.ProxyURL(proxy), DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: time.Second}
	probe := func(want int) {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal("transport error is not proof of BLOCK", err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 100))
		if err != nil || resp.StatusCode != want || (want == 200 && string(body) != "owned-generation-origin") {
			t.Fatalf("native fixture HTTP%d wanted%d", resp.StatusCode, want)
		}
	}
	a, b, c := makeGeneration(false, "error"), makeGeneration(true, "error"), makeGeneration(true, "warn")
	if a.Tag == b.Tag || b.Tag == c.Tag {
		t.Fatal("distinct runtime JSON shared a witness")
	}
	write(a)
	start()
	wait(a)
	probe(200)
	write(b)
	if clash.VerifyRuntimeGeneration(ctx, b.Tag) == nil {
		t.Fatal("disk-only candidate falsely confirmed")
	}
	if err := clash.VerifyRuntimeGeneration(ctx, a.Tag); err != nil {
		t.Fatal("actual old generation lost", err)
	}
	probe(200)
	t.Log("NATIVE_GENERATION disk-only B rejected while actual A serves DIRECT")
	stop()
	start()
	wait(b)
	if clash.VerifyRuntimeGeneration(ctx, a.Tag) == nil {
		t.Fatal("retired A falsely confirmed")
	}
	probe(502)
	t.Log("NATIVE_GENERATION actual B confirmed and fixture HTTP502 BLOCK observed")
	write(c)
	if clash.VerifyRuntimeGeneration(ctx, c.Tag) == nil {
		t.Fatal("unstarted C falsely confirmed")
	}
	if err := clash.VerifyRuntimeGeneration(ctx, b.Tag); err != nil {
		t.Fatal("actual B witness lost", err)
	}
	write(a)
	stop()
	start()
	wait(a)
	probe(200)
	t.Log("native generation library and stale rules controls passed")
}
