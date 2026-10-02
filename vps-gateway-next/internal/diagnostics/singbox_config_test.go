package diagnostics

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestSanitizeSingboxConfigRedactsSensitiveFields(t *testing.T) {
	raw := `{
		"outbounds": [
			{
				"type": "vless",
				"tag": "test-out",
				"server": "example-vless.example.test",
				"server_port": 443,
				"uuid": "11111111-1111-4111-8111-111111111111",
				"tls": {
					"server_name": "example-sni.example.test",
					"reality": {
						"public_key": "TEST_REALITY_PUBLIC_KEY_111111111111111111111111111111111111111",
						"short_id": "deadbeef"
					}
				}
			}
		]
	}`

	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	sanitizeSingboxConfig(cfg)

	outbounds, ok := cfg["outbounds"].([]any)
	if !ok || len(outbounds) == 0 {
		t.Fatal("outbounds missing or empty")
	}
	ob := outbounds[0].(map[string]any)

	if ob["type"] != "vless" {
		t.Errorf("type: got %v, want vless", ob["type"])
	}
	if ob["tag"] != "test-out" {
		t.Errorf("tag: got %v, want test-out", ob["tag"])
	}

	rawSeen := map[string]bool{
		"example-vless.example.test":                                      false,
		"11111111-1111-4111-8111-111111111111":                            false,
		"TEST_REALITY_PUBLIC_KEY_111111111111111111111111111111111111111": false,
		"deadbeef":                 false,
		"example-sni.example.test": false,
	}

	for k := range rawSeen {
		rawSeen[k] = true
	}

	for k, v := range ob {
		if isSensitiveKey(k) {
			if sv, ok := v.(string); ok {
				if rawSeen[sv] {
					t.Errorf("raw value %q still present for key %q", sv, k)
				}
			}
		}
	}

	if sv, ok := getDeepString(ob, "tls", "server_name"); !ok || (ok && sv == "example-sni.example.test") {
		t.Errorf("server_name not redacted: got %v", sv)
	} else if ok {
		if sv[:4] != "SNI-" {
			t.Errorf("server_name alias prefix: got %q, want SNI-*", sv)
		}
	}

	if sv, ok := getDeepString(ob, "tls", "reality", "public_key"); !ok || (ok && sv == "TEST_REALITY_PUBLIC_KEY_111111111111111111111111111111111111111") {
		t.Errorf("public_key not redacted: got %v", sv)
	} else if ok {
		if sv[:15] != "REALITY-PUBKEY-" {
			t.Errorf("public_key alias prefix: got %q, want REALITY-PUBKEY-*", sv)
		}
	}

	if sv, ok := getDeepString(ob, "tls", "reality", "short_id"); !ok || (ok && sv == "deadbeef") {
		t.Errorf("short_id not redacted: got %v", sv)
	} else if ok {
		if sv[:9] != "SHORT-ID-" {
			t.Errorf("short_id alias prefix: got %q, want SHORT-ID-*", sv)
		}
	}

	// server_port must be 0 (float64 from json.Unmarshal)
	sp := ob["server_port"]
	if sp != 0 {
		t.Errorf("server_port: got %v, want 0", sp)
	}

	// Check top-level aliases
	if sv, ok := ob["server"].(string); !ok {
		t.Error("server key missing")
	} else if sv[:7] != "SERVER-" {
		t.Errorf("server alias: got %q, want SERVER-*", sv)
	}

	if sv, ok := ob["uuid"].(string); !ok {
		t.Error("uuid key missing")
	} else if sv[:5] != "UUID-" {
		t.Errorf("uuid alias: got %q, want UUID-*", sv)
	}
}

func isSensitiveKey(k string) bool {
	switch k {
	case "uuid", "server", "server_name", "server_port":
		return true
	default:
		return false
	}
}

func getDeepString(obj map[string]any, keys ...string) (string, bool) {
	cur := any(obj)
	for _, key := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = m[key]
		if !ok {
			return "", false
		}
	}
	s, ok := cur.(string)
	return s, ok
}

func TestSanitizeSingboxConfigPreservesRepeatedAliases(t *testing.T) {
	raw := `{
		"outbounds": [
			{
				"type": "vless",
				"tag": "out-1",
				"server": "example-vless.example.test",
				"server_port": 443,
				"uuid": "11111111-1111-4111-8111-111111111111"
			},
			{
				"type": "vless",
				"tag": "out-2",
				"server": "example-vless.example.test",
				"server_port": 443,
				"uuid": "11111111-1111-4111-8111-111111111111"
			}
		]
	}`

	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	sanitizeSingboxConfig(cfg)

	outbounds := cfg["outbounds"].([]any)
	ob1 := outbounds[0].(map[string]any)
	ob2 := outbounds[1].(map[string]any)

	if ob1["server"] != ob2["server"] {
		t.Errorf("repeated server got different aliases: %q vs %q", ob1["server"], ob2["server"])
	}
	if ob1["uuid"] != ob2["uuid"] {
		t.Errorf("repeated uuid got different aliases: %q vs %q", ob1["uuid"], ob2["uuid"])
	}

	if ob1["server_port"] != 0 || ob2["server_port"] != 0 {
		t.Errorf("server_port not 0: %v, %v", ob1["server_port"], ob2["server_port"])
	}
}

func TestSanitizeSingboxConfigPreservesNonSensitiveStructure(t *testing.T) {
	raw := `{
		"outbounds": [
			{
				"type": "direct",
				"tag": "direct-out",
				"bind_interface": "eth1"
			}
		],
		"route": {
			"rules": [
				{ "outbound": "direct-out" }
			]
		},
		"dns": {
			"servers": [
				{ "tag": "cloudflare-dns", "address": "1.1.1.1" }
			]
		}
	}`

	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	sanitizeSingboxConfig(cfg)

	outbounds := cfg["outbounds"].([]any)
	ob := outbounds[0].(map[string]any)
	if ob["type"] != "direct" {
		t.Errorf("type changed: %v", ob["type"])
	}
	if ob["tag"] != "direct-out" {
		t.Errorf("tag changed: %v", ob["tag"])
	}
	if ob["bind_interface"] != "eth1" {
		t.Errorf("bind_interface changed: %v", ob["bind_interface"])
	}

	route, ok := cfg["route"].(map[string]any)
	if !ok {
		t.Fatal("route section missing")
	}
	rules, ok := route["rules"].([]any)
	if !ok || len(rules) == 0 {
		t.Fatal("route rules missing")
	}

	dns, ok := cfg["dns"].(map[string]any)
	if !ok {
		t.Fatal("dns section missing")
	}
	servers, ok := dns["servers"].([]any)
	if !ok || len(servers) == 0 {
		t.Fatal("dns servers missing")
	}

	dnsServer := servers[0].(map[string]any)
	if dnsServer["tag"] != "cloudflare-dns" {
		t.Fatalf("dns tag changed: %v", dnsServer["tag"])
	}

	addr, _ := dnsServer["address"].(string)
	if addr == "1.1.1.1" {
		t.Fatalf("dns server address was not sanitized")
	}
	if !strings.HasPrefix(addr, "DNS-SERVER-") {
		t.Fatalf("dns server address alias = %q, want DNS-SERVER-*", addr)
	}
}

func TestCollectSingboxConfigSuccess(t *testing.T) {
	raw := `{"outbounds": [{"type": "direct", "tag": "direct"}]}`

	deps := Deps{
		SingboxConfigPreview: func() (string, error) {
			return raw, nil
		},
	}

	runner := NewRunner(deps)
	info := runner.collectSingboxConfig()

	if info == nil || !info.Available {
		t.Fatal("expected available config")
	}
	if info.Error != "" {
		t.Errorf("unexpected error: %s", info.Error)
	}
	if info.Config == nil {
		t.Fatal("config is nil")
	}
}

func TestCollectSingboxConfigError(t *testing.T) {
	deps := Deps{
		SingboxConfigPreview: func() (string, error) {
			return "", fmt.Errorf("preview failed")
		},
	}

	runner := NewRunner(deps)
	info := runner.collectSingboxConfig()

	if info == nil {
		t.Fatal("expected non-nil info on error")
	}
	if info.Available != false {
		t.Errorf("Available: got %v, want false", info.Available)
	}
	if info.Error == "" {
		t.Error("expected non-empty error")
	}
	if info.Config != nil {
		t.Error("expected nil Config on error")
	}
}

func TestCollectSingboxConfigDisabledDep(t *testing.T) {
	runner := NewRunner(Deps{})
	info := runner.collectSingboxConfig()

	if info != nil {
		t.Errorf("expected nil when dep is disabled, got %+v", info)
	}
}
