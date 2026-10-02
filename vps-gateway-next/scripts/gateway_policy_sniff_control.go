//go:build ignore

// A namespace-local rule-engine control, NOT an AWG/TUN datapath test.
// The generated Gateway route is unchanged; only its inbound adapter is SOCKS.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hoaxisr/awg-manager/internal/gateway/policy"
)

const body = "gateway-sniff-control\n"

func main() {
	output := flag.String("output", "/tmp/sniff-control", "evidence directory")
	engine := flag.String("sing-box", "/usr/local/bin/sing-box", "engine binary")
	without := flag.Bool("without-sniff", false, "reproduce pre-fix order by removing only sniff")
	flag.Parse()
	if err := run(*output, *engine, *without); err != nil {
		fmt.Fprintln(os.Stderr, "CONTROL_FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("RULE_ENGINE_CONTROL_PASS (not AWG/TUN DIRECT proof)")
}

func run(output, engine string, without bool) error {
	if err := os.MkdirAll(output, 0700); err != nil { return err }
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil { return err }
	defer listener.Close()
	var mu sync.Mutex
	hits := map[string]int{}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock(); hits[r.Host]++; mu.Unlock()
		_, _ = io.WriteString(w, body)
	})}
	defer server.Close()
	go func() { _ = server.Serve(listener) }()
	compiled, err := policy.Compile(policy.Profile{ID: "sniff-control", Name: "Sniff control", DefaultAction: policy.ActionDirect,
		Rules: []policy.Rule{{ID: "domain-block", Enabled: true, Action: policy.ActionBlock, DomainSuffixes: []string{"blocked.gateway.test"}}}},
		policy.CompileOptions{Outbounds: map[string]struct{}{"direct": {}}, OutboundActions: map[string]policy.Action{"direct": policy.ActionDirect}, DefaultOutbounds: map[policy.Action]string{policy.ActionDirect: "direct"}})
	if err != nil { return err }
	data, err := policy.RenderSlot(compiled)
	if err != nil { return err }
	var slot struct { Route struct { Rules []policy.CompiledRule `json:"rules"` } `json:"route"` }
	if err := json.Unmarshal(data, &slot); err != nil { return err }
	if without {
		kept := make([]policy.CompiledRule, 0, len(slot.Route.Rules))
		for _, rule := range slot.Route.Rules { if rule.Action != "sniff" { kept = append(kept, rule) } }
		slot.Route.Rules = kept
	}
	config := map[string]any{
		"log": map[string]any{"level": "debug", "timestamp": true},
		"inbounds": []any{map[string]any{"type": "socks", "tag": policy.GatewayInboundTag, "listen": "127.0.0.1", "listen_port": 10801}},
		"outbounds": []any{map[string]any{"type": "direct", "tag": "direct"}},
		"route": slot.Route,
	}
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil { return err }
	configPath := filepath.Join(output, "config.json")
	if err := os.WriteFile(configPath, configData, 0600); err != nil { return err }
	check := exec.Command(engine, "check", "-c", configPath)
	checkOutput, checkErr := check.CombinedOutput()
	if err := os.WriteFile(filepath.Join(output, "check.log"), checkOutput, 0600); err != nil { return err }
	if checkErr != nil { return fmt.Errorf("engine check: %w: %s", checkErr, checkOutput) }
	log, err := os.Create(filepath.Join(output, "engine.log"))
	if err != nil { return err }
	defer log.Close()
	cmd := exec.Command(engine, "run", "-c", configPath)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil { return err }
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	ready := false
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		c, dialErr := net.DialTimeout("tcp4", "127.0.0.1:10801", 100*time.Millisecond)
		if dialErr == nil { _ = c.Close(); ready = true; break }
		time.Sleep(20 * time.Millisecond)
	}
	if !ready { return fmt.Errorf("SOCKS listener did not become ready") }
	fmt.Printf("WITHOUT_SNIFF=%t TARGET=%s EXPECTED_SHA256=%x\n", without, listener.Addr(), sha256.Sum256([]byte(body)))
	var failures []string
	for _, host := range []string{"allowed.gateway.test", "blocked.gateway.test", "allowed.gateway.test"} {
		got, connected, requestErr := request(listener.Addr().String(), host)
		fmt.Printf("HOST=%s SOCKS_CONNECTED=%t HTTP_SUCCESS=%t BODY_SHA256=%x ERROR=%v\n", host, connected, requestErr == nil, sha256.Sum256([]byte(got)), requestErr)
		if !connected { failures = append(failures, "SOCKS connect failed, not evidence of domain BLOCK") }
		if host == "blocked.gateway.test" {
			if requestErr == nil { failures = append(failures, "domain BLOCK escaped through DIRECT fallback") }
		} else if requestErr != nil || got != body { failures = append(failures, "allowed DIRECT response/hash failed") }
	}
	mu.Lock()
	hitData, err := json.Marshal(hits)
	blockedHits, allowedHits := hits["blocked.gateway.test"], hits["allowed.gateway.test"]
	mu.Unlock()
	if err != nil { return err }
	fmt.Printf("SINK_HITS=%s\n", hitData)
	if err := os.WriteFile(filepath.Join(output, "sink-hits.json"), hitData, 0600); err != nil { return err }
	if blockedHits != 0 || allowedHits != 2 { failures = append(failures, "sink counters do not show two allowed and zero blocked requests") }
	if len(failures) != 0 { return fmt.Errorf("%v", failures) }
	return nil
}

// Connect to a literal IPv4 sink through SOCKS, never resolve a hostname.
// The domain exists only in HTTP Host, so a match must come from sniffing.
func request(target, host string) (string, bool, error) {
	conn, err := net.DialTimeout("tcp4", "127.0.0.1:10801", time.Second)
	if err != nil { return "", false, err }
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(4 * time.Second))
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil { return "", false, err }
	var greeting [2]byte
	if _, err := io.ReadFull(conn, greeting[:]); err != nil { return "", false, err }
	if greeting != [2]byte{5, 0} { return "", false, fmt.Errorf("unexpected SOCKS greeting %v", greeting) }
	ipText, portText, err := net.SplitHostPort(target)
	if err != nil { return "", false, err }
	port, err := strconv.Atoi(portText)
	if err != nil { return "", false, err }
	ip := net.ParseIP(ipText).To4()
	if ip == nil { return "", false, fmt.Errorf("target is not literal IPv4") }
	connect := append([]byte{5, 1, 0, 1}, ip...)
	connect = append(connect, byte(port>>8), byte(port))
	if _, err := conn.Write(connect); err != nil { return "", false, err }
	var reply [4]byte
	if _, err := io.ReadFull(conn, reply[:]); err != nil { return "", false, err }
	if reply[0] != 5 || reply[1] != 0 { return "", false, fmt.Errorf("SOCKS CONNECT rejected %v", reply) }
	remaining := 0
	switch reply[3] {
	case 1: remaining = 6
	case 4: remaining = 18
	case 3:
		var length [1]byte
		if _, err := io.ReadFull(conn, length[:]); err != nil { return "", false, err }
		remaining = int(length[0]) + 2
	default: return "", false, fmt.Errorf("invalid SOCKS address type")
	}
	if _, err := io.CopyN(io.Discard, conn, int64(remaining)); err != nil { return "", false, err }
	if _, err := fmt.Fprintf(conn, "GET /probe HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host); err != nil { return "", true, err }
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "GET"})
	if err != nil { return "", true, err }
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 1024))
	if err != nil { return "", true, err }
	if response.StatusCode != http.StatusOK { return string(data), true, fmt.Errorf("HTTP status %d", response.StatusCode) }
	return string(data), true, nil
}
