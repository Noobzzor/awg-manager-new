package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/gateway/policy"
	"github.com/hoaxisr/awg-manager/internal/gateway/routerpolicy"
	"github.com/hoaxisr/awg-manager/internal/singbox"
)

// This opt-in check uses the compiled main, original HTTP handlers, real AWG
// ingress and the actual TUN engine. It also tests cold recovery from on-disk
// journal phases and optionally sends TCP through the synthetic AWG peer.
func TestNativeOriginalApplyThroughApplication(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_ORIGINAL_APPLY_GOAL") != "1" {
		t.Skip("opt-in original application Apply goal")
	}
	binary := os.Getenv("AWGM_ORIGINAL_APP")
	if binary == "" {
		t.Fatal("AWGM_ORIGINAL_APP required")
	}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	var packet *originalPacketFixture
	wan := "lo"
	if os.Getenv("AWGM_NATIVE_ORIGINAL_PACKET") == "1" {
		packet = startOriginalPacketFixture(t, ctx)
		wan = "fixture0"
	}
	env := append(os.Environ(), "AWG_MODE=singbox", "AWG_ROUTER_POLICY_MODE=original", "AWG_DATA_DIR="+root,
		"AWG_HTTP_ADDR=127.0.0.1:2222", "SINGBOX_BIN=/usr/local/bin/sing-box", "SINGBOX_CONFIG_DIR="+filepath.Join(root, "sing-box/config.d"), "AWG_PROXY_ADDR=127.0.0.1:1080",
		"AWG_GATEWAY_ENABLE=true", "AWG_GATEWAY_BACKEND=userspace-awg", "AWG_GATEWAY_RUNTIME_BINARY=/input/amneziawg-go", "AWG_GATEWAY_IP_BINARY=/usr/sbin/ip", "AWG_GATEWAY_TOOL=/artifacts/awg",
		"AWG_GATEWAY_INTERFACE=m2oa0", "AWG_GATEWAY_PORT=51820", "AWG_GATEWAY_CLIENT_POOL=10.77.0.0/30", "AWG_GATEWAY_SERVER_ADDRESS=10.77.0.1/30", "AWG_GATEWAY_WAN_INTERFACE="+wan, "AWG_GATEWAY_PUBLIC_ENDPOINT=127.0.0.1:51820",
		`AWG_GATEWAY_PARAMETERS={"jc":3,"jmin":40,"jmax":80,"s1":16,"s2":16,"s3":16,"s4":16,"h1":1001,"h2":1002,"h3":1003,"h4":1004}`)
	client := &http.Client{Timeout: 2 * time.Second}
	base := "http://127.0.0.1:2222"
	var stop func(bool)
	defer func() {
		if stop != nil {
			stop(false)
		}
	}()
	start := func() int {
		t.Helper()
		app := exec.CommandContext(ctx, binary)
		var diagnostics bytes.Buffer
		app.Stdout, app.Stderr = &diagnostics, &diagnostics
		app.Env = env
		if err := app.Start(); err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() { done <- app.Wait() }()
		stop = func(requireClean bool) {
			stop = nil
			_ = app.Process.Signal(syscall.SIGTERM)
			select {
			case err := <-done:
				if requireClean && err != nil {
					t.Fatal("original application did not exit cleanly", err)
				}
			case <-time.After(12 * time.Second):
				_ = app.Process.Kill()
				<-done
				if requireClean {
					t.Fatal("original application shutdown timed out")
				}
			}
		}
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			select {
			case err := <-done:
				stop = nil
				output := diagnostics.String()
				if len(output) > 1800 {
					output = output[len(output)-1800:]
				}
				t.Log("disposable fixture startup diagnostics:", output)
				t.Fatal("application exited before original HTTP goal", err)
			default:
			}
			resp, err := client.Get(base + "/api/health")
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return app.Process.Pid
				}
			}
			time.Sleep(50 * time.Millisecond)
		}
		t.Fatal("application HTTP liveness unavailable")
		return 0
	}
	pid := start()
	if packet != nil {
		packet.provision()
	}
	owner, err := routerpolicy.ReadPolicyOwner(root)
	if err != nil || owner != routerpolicy.PolicyOwnerOriginal {
		t.Errorf("exact main has not selected original ownership: owner=%q err=%v", owner, err)
	}
	call := func(method, path, body string) json.RawMessage {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", base)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal("original HTTP request failed", path, err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct {
			Error bool            `json:"error"`
			Code  string          `json:"code"`
			Data  json.RawMessage `json:"data"`
			Check string          `json:"sbCheck"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil || resp.StatusCode != 200 || envelope.Error {
			detail := regexp.MustCompile(`[A-Za-z0-9+/]{43}=`).ReplaceAllString(envelope.Check, "[redacted]")
			if len(detail) > 900 {
				detail = detail[:900]
			}
			t.Fatalf("original HTTP request failed: %s status=%d code=%s envelope=%v check=%s", path, resp.StatusCode, envelope.Code, err, detail)
		}
		return envelope.Data
	}
	var complete struct{ Presets []json.RawMessage }
	if err := json.Unmarshal(call("GET", "/api/presets", ""), &complete); err != nil || len(complete.Presets) != 87 {
		t.Fatal("original complete catalog differs", len(complete.Presets), err)
	}
	var routerPresets []json.RawMessage
	if err := json.Unmarshal(call("GET", "/api/singbox/router/presets/list", ""), &routerPresets); err != nil || len(routerPresets) != 77 {
		t.Fatal("original router-applicable catalog differs", len(routerPresets), err)
	}
	var caps struct{ SingboxRouter bool }
	if err := json.Unmarshal(call("GET", "/api/capabilities", ""), &caps); err != nil || caps.SingboxRouter {
		t.Fatal("prototype advertised the full editor capability", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/singbox/router/rulesets/update", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", base)
	unsupported, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	unsupported.Body.Close()
	if unsupported.StatusCode != http.StatusNotImplemented {
		t.Fatal("unsupported rule-set update reached the original handler", unsupported.StatusCode)
	}
	seen := map[string]bool{}
	var initialDirect routerpolicy.ApplyState
	for _, final := range []string{"direct", "block", "direct"} {
		call("POST", "/api/singbox/router/route/final", `{"final":"`+final+`"}`)
		call("POST", "/api/singbox/router/staging/apply", `{}`)
		var status struct {
			Final string `json:"final"`
		}
		if err := json.Unmarshal(call("GET", "/api/singbox/router/status", ""), &status); err != nil || status.Final != final {
			t.Fatal("accepted original status differs", final, status.Final, err)
		}
		var staging struct {
			HasDraft bool `json:"hasDraft"`
		}
		if err := json.Unmarshal(call("GET", "/api/singbox/router/staging", ""), &staging); err != nil || staging.HasDraft {
			t.Fatal("accepted draft not finalized", err)
		}
		marker := originalGoalLiveBinding(t, ctx, client, root, pid, final)
		if seen[marker] {
			t.Fatal("Apply reused an earlier generation witness")
		}
		seen[marker] = true
		if packet != nil {
			packet.probe(final)
		}
		if final == "direct" && initialDirect.Editor == nil {
			initialDirect = originalGoalSnapshot(t, root)
		}
		if final == "block" {
			blocked := originalGoalSnapshot(t, root)
			stop(true)
			client.CloseIdleConnections()
			pid = start()
			restarted := originalGoalLiveBinding(t, ctx, client, root, pid, final)
			if seen[restarted] {
				t.Fatal("restart reused a previous engine generation")
			}
			seen[restarted] = true
			if packet != nil {
				packet.provision()
				packet.probe(final)
			}
			t.Log("clean restart restored the live BLOCK policy")
			for _, recovery := range []struct {
				phase, final string
				expected     routerpolicy.ApplyState
			}{
				{"prepared", "direct", initialDirect},
				{"accepted", "block", blocked},
			} {
				stop(true)
				client.CloseIdleConnections()
				journal := filepath.Join(routerpolicy.OriginalStateDir(root), "apply-journal.json")
				raw, err := originalGoalJournal(recovery.phase, initialDirect, blocked)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(journal, raw, 0600); err != nil {
					t.Fatal(err)
				}
				pid = start()
				recovered := originalGoalSnapshot(t, root)
				if !bytes.Equal(recovered.Editor, recovery.expected.Editor) || !bytes.Equal(recovered.Runtime, recovery.expected.Runtime) {
					t.Fatal("cold journal recovery restored wrong authored or runtime state", recovery.phase)
				}
				witness := originalGoalLiveBinding(t, ctx, client, root, pid, recovery.final)
				if seen[witness] {
					t.Fatal("journal recovery reused a prior engine generation")
				}
				seen[witness] = true
				if _, err := os.Lstat(journal); !os.IsNotExist(err) {
					t.Fatal("confirmed journal recovery did not clear the journal", recovery.phase)
				}
				if packet != nil {
					packet.provision()
					packet.probe(recovery.final)
				}
				t.Log("cold journal recovery confirmed", recovery.phase, recovery.final)
			}
		}
	}
	if os.Getenv("AWGM_NATIVE_ORIGINAL_ORDER") == "1" {
		if packet == nil {
			t.Fatal("rule order check requires the packet fixture")
		}
		call("POST", "/api/singbox/router/rules/add", `{"ip_cidr":["203.0.113.2/32"],"port":[51980],"action":"route","outbound":"direct"}`)
		call("POST", "/api/singbox/router/rules/add", `{"ip_cidr":["203.0.113.2/32"],"port":[51980],"action":"reject"}`)
		assertOrder := func(first, second string) {
			t.Helper()
			var rules []struct{ Action string }
			err := json.Unmarshal(call("GET", "/api/singbox/router/rules/list", ""), &rules)
			if err != nil || len(rules) != 2 || rules[0].Action != first || rules[1].Action != second {
				t.Fatal("original rule priority differs", rules, err)
			}
		}
		assertOrder("route", "reject")
		call("POST", "/api/singbox/router/staging/apply", `{}`)
		originalGoalLiveBinding(t, ctx, client, root, pid, "direct")
		packet.probe("direct")
		call("POST", "/api/singbox/router/rules/move", `{"from":1,"to":0}`)
		assertOrder("reject", "route")
		packet.probe("direct")
		call("POST", "/api/singbox/router/staging/apply", `{}`)
		originalGoalLiveBinding(t, ctx, client, root, pid, "direct")
		packet.probe("block")
		t.Log("original staged order only changed AWG delivery after Apply")
		stop(true)
		client.CloseIdleConnections()
		pid = start()
		assertOrder("reject", "route")
		originalGoalLiveBinding(t, ctx, client, root, pid, "direct")
		packet.provision()
		packet.probe("block")
		t.Log("original applied rule order survived restart")
	}
	if os.Getenv("AWGM_NATIVE_ORIGINAL_ASSETS") == "1" {
		if packet == nil {
			t.Fatal("rule-set check requires the packet fixture")
		}
		call("POST", "/api/singbox/router/rulesets/add", `{"tag":"local-asset-proof","type":"inline","rules":[{"ip_cidr":["203.0.113.2/32"]}]}`)
		call("POST", "/api/singbox/router/rules/add", `{"rule_set":["local-asset-proof"],"action":"reject"}`)
		packet.probe("direct")
		call("POST", "/api/singbox/router/staging/apply", `{}`)
		originalGoalLiveBinding(t, ctx, client, root, pid, "direct")
		packet.probe("block")
		stop(true)
		client.CloseIdleConnections()
		pid = start()
		originalGoalLiveBinding(t, ctx, client, root, pid, "direct")
		packet.provision()
		packet.probe("block")
		t.Log("original compiled rule-set blocked AWG TCP after Apply and restart")
	}
	if os.Getenv("AWGM_NATIVE_ORIGINAL_PRESET") == "1" {
		if packet == nil {
			t.Fatal("preset requires the packet fixture")
		}
		originalGoalGitHubPreset(t, ctx, root, call, packet)
		originalGoalLiveBinding(t, ctx, client, root, pid, "direct")
		stop(true)
		client.CloseIdleConnections()
		pid = start()
		originalGoalLiveBinding(t, ctx, client, root, pid, "direct")
		var rules []struct{ Action string }
		err := json.Unmarshal(call("GET", "/api/singbox/router/rules/list", ""), &rules)
		if err != nil || len(rules) != 2 || rules[0].Action != "reject" || rules[1].Action != "route" {
			t.Fatal("preset priority lost after restart", rules, err)
		}
		packet.provision()
		packet.probePayload("block", []byte("GET / HTTP/1.0\nHOST:github.com\n\n"))
		t.Log("original GitHub preset selected, reordered, applied and restored")
	}
	stop(true)
	t.Log("original application HTTP Apply DIRECT/BLOCK/DIRECT and owned immutable generation/controller binding passed")
}

func originalGoalGitHubPreset(t *testing.T, ctx context.Context, root string, call func(string, string, string) json.RawMessage, packet *originalPacketFixture) {
	t.Helper()
	const digest = "438f12d297174b168b3b6acd4572c360c083c24641736bdaefb4ed2cb3d0295f"
	raw, err := os.ReadFile("/artifacts/original-github-reference.srs")
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != digest {
		t.Fatal("original GitHub artifact differs", err)
	}
	store := filepath.Join(root, "sing-box", "config.d", "rule-sets", "presets")
	if err := os.MkdirAll(store, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store, "sha256-"+digest+".srs")
	if err := os.WriteFile(path, raw, 0400); err != nil {
		t.Fatal(err)
	}
	match := exec.CommandContext(ctx, "/usr/local/bin/sing-box", "rule-set", "match", "-f", "binary", path, "github.com")
	result, _ := match.CombinedOutput()
	if !strings.Contains(string(result), "match rules.") {
		t.Fatal("pinned GitHub set did not match github.com")
	}
	call("POST", "/api/singbox/router/presets/apply", `{"id":"github","outbound":""}`)
	call("POST", "/api/singbox/router/rules/add", `{"rule_set":["geosite-github"],"action":"route","outbound":"direct"}`)
	call("POST", "/api/singbox/router/rules/move", `{"from":1,"to":0}`)
	call("POST", "/api/singbox/router/staging/apply", `{}`)
	packet.probePayload("direct", []byte("GET / HTTP/1.0\nHost:github.com\n\n"))
	call("POST", "/api/singbox/router/rules/move", `{"from":1,"to":0}`)
	call("POST", "/api/singbox/router/staging/apply", `{}`)
	packet.probePayload("block", []byte("GET / HTTP/1.0\nhost:github.com\n\n"))
	packet.probePayload("direct", []byte("GET / HTTP/1.0\nHost:example.io\n\n"))
}

func originalGoalJournal(phase string, before, after routerpolicy.ApplyState) ([]byte, error) {
	state, err := json.Marshal(struct {
		Version int                     `json:"version"`
		Phase   string                  `json:"phase"`
		Before  routerpolicy.ApplyState `json:"before"`
		After   routerpolicy.ApplyState `json:"after"`
	}{Version: 1, Phase: phase, Before: before, After: after})
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		State  json.RawMessage `json:"state"`
		SHA256 string          `json:"sha256"`
	}{State: state, SHA256: fmt.Sprintf("%x", sha256.Sum256(state))})
}

func originalGoalSnapshot(t *testing.T, root string) routerpolicy.ApplyState {
	t.Helper()
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	config := filepath.Join(root, "sing-box", "config.d")
	type slot struct {
		Data    []byte `json:"data"`
		Enabled bool   `json:"enabled"`
	}
	runtime, err := json.Marshal(struct {
		Version int  `json:"version"`
		Ingress slot `json:"ingress"`
		Routing slot `json:"routing"`
	}{
		Version: 1,
		Ingress: slot{Data: read(filepath.Join(config, "18-z-gateway-policy.json")), Enabled: true},
		Routing: slot{Data: read(filepath.Join(config, "20-router.json")), Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	return routerpolicy.ApplyState{
		Editor:  read(filepath.Join(routerpolicy.OriginalStateDir(root), "editor", "applied.json")),
		Runtime: runtime, Enabled: true,
	}
}

func originalGoalLiveBinding(t *testing.T, ctx context.Context, client *http.Client, root string, parent int, want string) string {
	t.Helper()
	// Independently find the owner of the actual loopback controller socket.
	listener := func() string {
		raw, err := os.ReadFile("/proc/net/tcp")
		if err != nil {
			t.Fatal(err)
		}
		address := fmt.Sprintf("0100007F:%04X", singbox.DefaultClashPort)
		for _, line := range strings.Split(string(raw), "\n") {
			fields := strings.Fields(line)
			if len(fields) > 9 && fields[1] == address && fields[3] == "0A" {
				return fields[9]
			}
		}
		t.Fatal("expected engine controller listener absent")
		return ""
	}
	inode := listener()
	entries, err := os.ReadDir("/proc")
	if err != nil {
		t.Fatal(err)
	}
	pid := 0
	for _, entry := range entries {
		candidate, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		fds, err := os.ReadDir(filepath.Join("/proc", entry.Name(), "fd"))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			target, _ := os.Readlink(filepath.Join("/proc", entry.Name(), "fd", fd.Name()))
			if target == "socket:["+inode+"]" {
				if pid != 0 && pid != candidate {
					t.Fatal("controller has multiple process owners")
				}
				pid = candidate
			}
		}
	}
	if pid <= 1 {
		t.Fatal("controller socket owner absent")
	}
	actual, err := os.Stat(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.Stat("/usr/local/bin/sing-box")
	if err != nil || !os.SameFile(actual, expected) {
		t.Fatal("controller is not owned by the actual engine", err)
	}
	birth := func() string {
		raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			t.Fatal(err)
		}
		end := strings.LastIndex(string(raw), ") ")
		if end < 0 {
			t.Fatal("invalid process identity")
		}
		fields := strings.Fields(string(raw[end+2:]))
		if len(fields) < 20 || fields[0] == "Z" || fields[1] != strconv.Itoa(parent) {
			t.Fatal("engine is not a live child of this application")
		}
		return fields[19]
	}
	beforeBirth := birth()
	argv, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSuffix(string(argv), "\x00"), "\x00")
	if len(args) != 4 || args[1] != "run" {
		t.Fatal("unexpected managed engine arguments")
	}
	config := args[3]
	if args[2] == "-C" {
		entries, err := os.ReadDir(config)
		if err != nil {
			t.Fatal(err)
		}
		files := []string{}
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
				files = append(files, filepath.Join(config, entry.Name()))
			}
		}
		if len(files) != 1 {
			t.Fatal("engine does not read one prepared full configuration")
		}
		config = files[0]
	} else if args[2] != "-c" {
		t.Fatal("engine config source is not explicit")
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	realConfig, err := filepath.EvalSymlinks(config)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(realRoot, realConfig)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		t.Fatal("prepared config escapes owned root")
	}
	info, err := os.Lstat(config)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		t.Fatal("prepared config is not private and regular")
	}
	raw, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	var full struct {
		Inbounds []struct {
			Type, Tag string
			Include   json.RawMessage `json:"include_interface"`
		} `json:"inbounds"`
		Outbounds []struct{ Type, Tag string } `json:"outbounds"`
		Route     struct {
			Final string
			Rules []struct {
				Inbound          json.RawMessage
				Action, Outbound string
			}
		} `json:"route"`
	}
	if err := json.Unmarshal(raw, &full); err != nil {
		t.Fatal(err)
	}
	if full.Route.Final == "block" {
		t.Fatal("gateway block hijacked the global final")
	}
	ingress := false
	for _, in := range full.Inbounds {
		if in.Tag == policy.GatewayInboundTag {
			var include []string
			if json.Unmarshal(in.Include, &include) != nil {
				var single string
				if json.Unmarshal(in.Include, &single) == nil {
					include = []string{single}
				}
			}
			ingress = in.Type == "tun" && len(include) == 1 && include[0] == "m2oa0"
		}
	}
	if !ingress {
		t.Fatal("prepared config replaced actual gateway TUN ingress")
	}
	fallback := false
	for i := len(full.Route.Rules) - 1; i >= 0; i-- {
		rule := full.Route.Rules[i]
		var tags []string
		if json.Unmarshal(rule.Inbound, &tags) != nil {
			var tag string
			if json.Unmarshal(rule.Inbound, &tag) == nil {
				tags = []string{tag}
			}
		}
		scoped := false
		for _, tag := range tags {
			if tag == policy.GatewayInboundTag {
				scoped = true
			}
		}
		if scoped {
			fallback = (want == "block" && rule.Action == "reject") || (want == "direct" && rule.Outbound == "direct")
			break
		}
	}
	if !fallback {
		t.Fatal("live prepared gateway fallback differs from accepted original draft")
	}
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/proxies", singbox.DefaultClashPort), nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil || resp.StatusCode != 200 {
		t.Fatal("live generation query failed", err)
	}
	var observed struct {
		Proxies map[string]struct{ Name, Type string } `json:"proxies"`
	}
	if err := json.Unmarshal(body, &observed); err != nil {
		t.Fatal(err)
	}
	marker := ""
	valid := regexp.MustCompile(`^awgm-generation-[0-9a-f]{64}$`)
	for tag, item := range observed.Proxies {
		if strings.HasPrefix(tag, "awgm-generation-") {
			if marker != "" || !valid.MatchString(tag) || item.Name != tag || !strings.EqualFold(item.Type, "direct") {
				t.Fatal("ambiguous live generation witness")
			}
			marker = tag
		}
	}
	if marker == "" {
		t.Fatal("live generation witness absent")
	}
	// Independently compare the entire prepared document with a fresh native
	// merge of the caller-selected canonical source, excluding only its witness.
	expectedFile := filepath.Join(t.TempDir(), "native-expected.json")
	merge := exec.CommandContext(ctx, "/usr/local/bin/sing-box", "merge", expectedFile, "-C", filepath.Join(root, "sing-box/config.d"))
	if err := merge.Run(); err != nil {
		t.Fatal("independent native source merge failed", err)
	}
	expectedRaw, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatal(err)
	}
	decode := func(raw []byte) map[string]any {
		t.Helper()
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		var value map[string]any
		if err := d.Decode(&value); err != nil || value == nil {
			t.Fatal("invalid full prepared/source document", err)
		}
		return value
	}
	prepared := decode(raw)
	outbounds, ok := prepared["outbounds"].([]any)
	if !ok {
		t.Fatal("prepared outbounds absent")
	}
	withoutWitness := make([]any, 0, len(outbounds))
	removed := 0
	for _, value := range outbounds {
		item, _ := value.(map[string]any)
		if item["tag"] == marker {
			removed++
			continue
		}
		withoutWitness = append(withoutWitness, value)
	}
	prepared["outbounds"] = withoutWitness
	expectedDoc := decode(expectedRaw)
	originalGoalCompareRuleSetCopies(t, filepath.Dir(config), prepared, expectedDoc)
	actualCanonical, err := json.Marshal(prepared)
	if err != nil {
		t.Fatal(err)
	}
	expectedCanonical, err := json.Marshal(expectedDoc)
	if err != nil || removed != 1 || !bytes.Equal(actualCanonical, expectedCanonical) {
		t.Fatal("full prepared config differs from independent native source merge", err)
	}
	inConfig := false
	for _, out := range full.Outbounds {
		if out.Tag == marker && out.Type == "direct" {
			inConfig = true
		}
	}
	after, err := os.ReadFile(config)
	if err != nil || !bytes.Equal(raw, after) || sha256.Sum256(after) != digest || !inConfig || beforeBirth != birth() || inode != listener() {
		t.Fatal("runtime/config/controller identity changed across confirmation")
	}
	return marker
}

func originalGoalCompareRuleSetCopies(t *testing.T, generation string, prepared, source map[string]any) {
	t.Helper()
	sets := func(doc map[string]any) []any {
		route, _ := doc["route"].(map[string]any)
		rules, _ := route["rule_set"].([]any)
		return rules
	}
	got, want := sets(prepared), sets(source)
	if len(got) != len(want) {
		t.Fatal("prepared rule-set count changed")
	}
	for index, value := range got {
		copySet, okCopy := value.(map[string]any)
		sourceSet, okSource := want[index].(map[string]any)
		if !okCopy || !okSource {
			t.Fatal("invalid rule-set declaration")
		}
		p, _ := copySet["path"].(string)
		s, _ := sourceSet["path"].(string)
		info, err := os.Lstat(p)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0400 {
			t.Fatal("prepared rule-set is not a read-only regular file", err)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		wantPath := filepath.Join(generation, "assets", fmt.Sprintf("sha256-%x.srs", sha256.Sum256(raw)))
		sourceRaw, err := os.ReadFile(s)
		if err != nil || p != wantPath || !bytes.Equal(raw, sourceRaw) {
			t.Fatal("prepared rule-set differs from owned source copy", err)
		}
		copySet["path"] = s
	}
}
