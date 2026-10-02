// internal/singbox/deviceproxy_config_test.go
package singbox

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildDeviceProxyConfigEnabled(t *testing.T) {
	spec := DeviceProxySpec{
		Enabled:     true,
		ListenAddr:  "192.168.1.1",
		Port:        1080,
		SelectedTag: "awg-vpn0",
		AWGTags:     []string{"awg-vpn0"},
	}
	data, err := BuildDeviceProxyConfig(spec)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, "tag") {
		t.Fatalf("output looks empty: %s", s)
	}
	// Must contain selector outbound + inbound + route rule.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	inbounds, _ := raw["inbounds"].([]any)
	if len(inbounds) == 0 {
		t.Errorf("missing/empty inbounds: %s", s)
	}
	outbounds, _ := raw["outbounds"].([]any)
	if len(outbounds) == 0 {
		t.Errorf("missing/empty outbounds: %s", s)
	}
	route, ok := raw["route"].(map[string]any)
	if !ok {
		t.Fatalf("missing route: %s", s)
	}
	rules, _ := route["rules"].([]any)
	if len(rules) == 0 {
		t.Errorf("missing/empty route.rules: %s", s)
	}

	// Inbound tag must be the device-proxy one.
	ib, _ := inbounds[0].(map[string]any)
	if tag, _ := ib["tag"].(string); tag != deviceProxyInboundTag {
		t.Errorf("first inbound tag = %q, want %q", tag, deviceProxyInboundTag)
	}

	// Selector outbound must reference the selected member.
	var selector map[string]any
	for _, v := range outbounds {
		ob, _ := v.(map[string]any)
		if tag, _ := ob["tag"].(string); tag == deviceProxySelectorTag {
			selector = ob
		}
	}
	if selector == nil {
		t.Fatalf("selector outbound %q missing in: %s", deviceProxySelectorTag, s)
	}
	if def, _ := selector["default"].(string); def != "awg-vpn0" {
		t.Errorf("selector default = %q, want awg-vpn0", def)
	}

	// Slot file must NOT carry base-config keys — those live in
	// 00-base.json and would collide if duplicated here.
	for _, k := range []string{"log", "dns", "experimental"} {
		if _, ok := raw[k]; ok {
			t.Errorf("slot output must not contain %q key: %s", k, s)
		}
	}
}

func TestBuildDeviceProxyConfigDisabled(t *testing.T) {
	spec := DeviceProxySpec{Enabled: false}
	data, err := BuildDeviceProxyConfig(spec)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// When disabled, output is a well-formed but contentless config.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// No inbound or selector outbound from device-proxy.
	if inbounds, ok := raw["inbounds"].([]any); ok && len(inbounds) > 0 {
		t.Errorf("expected no inbounds: %v", inbounds)
	}
	// Outbounds may include the NewConfig() defaults but must not include
	// the device-proxy selector tag.
	outbounds, _ := raw["outbounds"].([]any)
	for _, v := range outbounds {
		ob, _ := v.(map[string]any)
		if tag, _ := ob["tag"].(string); tag == deviceProxySelectorTag {
			t.Errorf("unexpected device-proxy selector outbound: %v", ob)
		}
	}
}
