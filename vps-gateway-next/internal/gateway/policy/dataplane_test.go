package policy

import (
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/configmerge"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

func TestRenderSlotCapturesGatewayIngressAndAppliesExplicitDefault(t *testing.T) {
	profile := Profile{
		ID:            "dataplane",
		Name:          "Dataplane",
		DefaultAction: ActionDirect,
		Rules: []Rule{{
			ID:          "phone-vpn",
			Action:      ActionVPN,
			SourceCIDRs: []string{"10.66.0.2/32"},
			Protocols:   []string{"tcp"},
			Outbound:    "vpn",
			Enabled:     true,
		}},
	}
	compiled, err := Compile(profile, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}, "vpn": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect, "vpn": ActionVPN},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err != nil {
		t.Fatalf("compile policy: %v", err)
	}
	data, err := RenderSlot(compiled)
	if err != nil {
		t.Fatalf("render policy slot: %v", err)
	}

	var slot struct {
		Inbounds []struct {
			Type             string   `json:"type"`
			Tag              string   `json:"tag"`
			InterfaceName    string   `json:"interface_name"`
			Address          []string `json:"address"`
			AutoRoute        bool     `json:"auto_route"`
			AutoRedirect     bool     `json:"auto_redirect"`
			IncludeInterface []string `json:"include_interface"`
			DNSMode          string   `json:"dns_mode"`
		} `json:"inbounds"`
		Route struct {
			Final            string `json:"final"`
			DefaultInterface string `json:"default_interface"`
			Rules            []struct {
				Inbound      []string `json:"inbound"`
				SourceIPCIDR []string `json:"source_ip_cidr"`
				Protocol     string   `json:"protocol"`
				Port         []int    `json:"port"`
				Network      []string `json:"network"`
				Action       string   `json:"action"`
				Outbound     string   `json:"outbound"`
			} `json:"rules"`
		} `json:"route"`
	}
	if err := json.Unmarshal(data, &slot); err != nil {
		t.Fatalf("decode policy slot: %v", err)
	}

	if slot.Route.Final != "" {
		t.Fatalf("gateway slot must not set a global route.final, got %q", slot.Route.Final)
	}
	if len(slot.Inbounds) != 1 {
		t.Fatalf("gateway slot inbounds = %d, want one TUN inbound: %s", len(slot.Inbounds), data)
	}
	tun := slot.Inbounds[0]
	if tun.Type != "tun" || tun.Tag != "awg-gateway-in" || tun.InterfaceName != "awgm0" {
		t.Fatalf("gateway TUN identity = %#v", tun)
	}
	if !tun.AutoRoute || !tun.AutoRedirect || tun.DNSMode != "disabled" {
		t.Fatalf("gateway TUN routing/DNS settings = %#v", tun)
	}
	if len(tun.Address) != 1 || !strings.HasSuffix(tun.Address[0], "/30") {
		t.Fatalf("gateway TUN must be IPv4-only, address=%v", tun.Address)
	}
	if len(tun.IncludeInterface) != 1 || tun.IncludeInterface[0] != "awg0" {
		t.Fatalf("gateway TUN interface scope = %v, want [awg0]", tun.IncludeInterface)
	}
	if slot.Route.DefaultInterface != "" {
		t.Fatalf("gateway policy must not set a global WAN interface, got %q", slot.Route.DefaultInterface)
	}
	if len(slot.Route.Rules) != 5 {
		t.Fatalf("gateway route rules = %d, want sniff, DNS hijacks, policy rule, and explicit default: %s", len(slot.Route.Rules), data)
	}
	for i, rule := range slot.Route.Rules {
		if len(rule.Inbound) != 1 || rule.Inbound[0] != "awg-gateway-in" {
			t.Fatalf("route rule %d inbound scope = %v", i, rule.Inbound)
		}
	}
	if got := slot.Route.Rules[0]; got.Action != "sniff" || got.Outbound != "" {
		t.Fatalf("Gateway must sniff before matching protocol/domain: %#v", got)
	}
	if got := slot.Route.Rules[1]; got.Action != "hijack-dns" || got.Protocol != "dns" || got.Outbound != "" || len(got.Port) != 0 {
		t.Fatalf("Gateway protocol-DNS hijack = %#v", got)
	}
	if got := slot.Route.Rules[2]; got.Action != "hijack-dns" || got.Protocol != "" || len(got.Port) != 1 || got.Port[0] != 53 || got.Outbound != "" {
		t.Fatalf("Gateway port-53 DNS hijack = %#v", got)
	}
	if got := slot.Route.Rules[3]; got.Action != "route" || got.Outbound != "vpn" || len(got.SourceIPCIDR) != 1 || got.SourceIPCIDR[0] != "10.66.0.2/32" {
		t.Fatalf("explicit client rule = %#v", got)
	}
	if got := slot.Route.Rules[4]; got.Action != "route" || got.Outbound != "direct" || len(got.SourceIPCIDR) != 0 {
		t.Fatalf("explicit inbound default rule must cover unmatched source traffic: %#v", got)
	}
}

func TestRenderSlotUsesExplicitBlockForUnmatchedGatewayTraffic(t *testing.T) {
	compiled, err := Compile(Profile{ID: "block-default", Name: "Block default", DefaultAction: ActionBlock}, CompileOptions{})
	if err != nil {
		t.Fatalf("compile BLOCK default: %v", err)
	}
	data, err := RenderSlot(compiled)
	if err != nil {
		t.Fatalf("render BLOCK default: %v", err)
	}
	var slot struct {
		Route struct {
			Rules []struct {
				Inbound      []string `json:"inbound"`
				SourceIPCIDR []string `json:"source_ip_cidr"`
				Protocol     string   `json:"protocol"`
				Port         []int    `json:"port"`
				Action       string   `json:"action"`
				Outbound     string   `json:"outbound"`
			} `json:"rules"`
		} `json:"route"`
	}
	if err := json.Unmarshal(data, &slot); err != nil {
		t.Fatalf("decode BLOCK slot: %v", err)
	}
	if len(slot.Route.Rules) != 4 {
		t.Fatalf("BLOCK slot rules = %#v, want sniff, two DNS hijacks and the BLOCK fallback", slot.Route.Rules)
	}
	if got := slot.Route.Rules[0]; got.Action != "sniff" || len(got.Inbound) != 1 || got.Inbound[0] != GatewayInboundTag {
		t.Fatalf("BLOCK slot must sniff Gateway traffic first: %#v", got)
	}
	for i, want := range []struct {
		protocol string
		port     int
	}{{protocol: "dns"}, {port: 53}} {
		got := slot.Route.Rules[i+1]
		if got.Action != "hijack-dns" || len(got.Inbound) != 1 || got.Inbound[0] != GatewayInboundTag || got.Protocol != want.protocol {
			t.Fatalf("BLOCK slot DNS hijack %d = %#v", i, got)
		}
		if want.port == 0 && len(got.Port) != 0 || want.port != 0 && (len(got.Port) != 1 || got.Port[0] != want.port) {
			t.Fatalf("BLOCK slot DNS hijack %d port = %v", i, got.Port)
		}
	}
	if got := slot.Route.Rules[3]; got.Action != "reject" || got.Outbound != "" || len(got.SourceIPCIDR) != 0 || len(got.Inbound) != 1 || got.Inbound[0] != GatewayInboundTag {
		t.Fatalf("unmatched gateway traffic must use explicit BLOCK default: %#v", slot.Route.Rules)
	}
}

func TestRenderSlotUsesConfiguredClientPoolForDefaultRule(t *testing.T) {
	compiled, err := Compile(Profile{ID: "custom-pool", Name: "Custom pool", DefaultAction: ActionDirect}, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
		ClientPool:       netip.MustParsePrefix("10.77.0.0/24"),
	})
	if err != nil {
		t.Fatalf("compile policy: %v", err)
	}
	data, err := RenderSlot(compiled)
	if err != nil {
		t.Fatalf("render policy slot: %v", err)
	}
	var slot struct {
		Route struct {
			Rules []struct {
				SourceIPCIDR []string `json:"source_ip_cidr"`
			} `json:"rules"`
		} `json:"route"`
	}
	if err := json.Unmarshal(data, &slot); err != nil {
		t.Fatalf("decode policy slot: %v", err)
	}
	if len(slot.Route.Rules) != 4 || len(slot.Route.Rules[3].SourceIPCIDR) != 0 {
		t.Fatalf("inbound default must not be narrowed by source CIDR: %#v", slot.Route.Rules)
	}
}

func TestMergedGatewayDNSHijackPrecedesDirectFallbackWithoutRouterSlot(t *testing.T) {
	dir := t.TempDir()
	filenames := make(map[orchestrator.Slot]string)
	for _, meta := range orchestrator.KnownSlots() {
		filenames[meta.Slot] = meta.Filename
	}
	writeSlot := func(slot orchestrator.Slot, body []byte) {
		t.Helper()
		filename := filenames[slot]
		if filename == "" {
			t.Fatalf("slot %q is not registered", slot)
		}
		if err := os.WriteFile(filepath.Join(dir, filename), body, 0644); err != nil {
			t.Fatalf("write %s: %v", filename, err)
		}
	}

	writeSlot(orchestrator.SlotQoSRoutes, []byte(`{"route":{"rules":[{"action":"route-options","network":["udp"],"udp_timeout":"1m"}]}}`))
	profile := Profile{
		ID:            "merged-order",
		Name:          "Merged order",
		DefaultAction: ActionDirect,
		Rules: []Rule{
			{ID: "vpn-policy", Action: ActionVPN, Outbound: "vpn", DomainSuffixes: []string{"policy-vpn.example"}, Priority: 10, Enabled: true},
			{ID: "direct-policy", Action: ActionDirect, Outbound: "direct", Ports: []int{8443}, Priority: 20, Enabled: true},
		},
	}
	compiled, err := Compile(profile, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}, "vpn": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect, "vpn": ActionVPN},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err != nil {
		t.Fatalf("compile policy: %v", err)
	}
	gatewaySlot, err := RenderSlot(compiled)
	if err != nil {
		t.Fatalf("render gateway slot: %v", err)
	}
	writeSlot(orchestrator.SlotGatewayPolicy, gatewaySlot)
	writeSlot(orchestrator.SlotDNSRoutes, []byte(`{"route":{"rules":[{"action":"route","domain_suffix":["dns-route-first.example"],"outbound":"vpn"},{"action":"route","domain_suffix":["dns-route-last.example"],"outbound":"direct"}]}}`))

	if _, err := os.Stat(filepath.Join(dir, filenames[orchestrator.SlotRouter])); !os.IsNotExist(err) {
		t.Fatalf("router slot must be absent for this standalone Gateway-order test, stat err=%v", err)
	}
	mergedJSON, err := configmerge.MergeDir(dir)
	if err != nil {
		t.Fatalf("merge active slots: %v", err)
	}
	var merged struct {
		Route struct {
			Rules []struct {
				Inbound      []string `json:"inbound"`
				Protocol     string   `json:"protocol"`
				Port         []int    `json:"port"`
				DomainSuffix []string `json:"domain_suffix"`
				SourceIPCIDR []string `json:"source_ip_cidr"`
				Network      []string `json:"network"`
				UDPTimeout   string   `json:"udp_timeout"`
				Action       string   `json:"action"`
				Outbound     string   `json:"outbound"`
			} `json:"rules"`
		} `json:"route"`
	}
	if err := json.Unmarshal([]byte(mergedJSON), &merged); err != nil {
		t.Fatalf("decode merged config: %v", err)
	}
	rules := merged.Route.Rules
	if len(rules) != 9 {
		t.Fatalf("merged route rules = %d, want QoS + sniff + two DNS hijacks + two policy rules + fallback + two DNS-route rules: %s", len(rules), mergedJSON)
	}
	if got := rules[0]; got.Action != "route-options" || len(got.Network) != 1 || got.Network[0] != "udp" || got.UDPTimeout != "1m" {
		t.Fatalf("QoS special rule lost or reordered: %#v", got)
	}
	if got := rules[1]; got.Action != "sniff" || len(got.Inbound) != 1 || got.Inbound[0] != GatewayInboundTag || got.Outbound != "" {
		t.Fatalf("Gateway sniff must precede matchers without changing QoS priority: %#v", got)
	}
	if got := rules[2]; got.Action != "hijack-dns" || len(got.Inbound) != 1 || got.Inbound[0] != GatewayInboundTag || got.Protocol != "dns" {
		t.Fatalf("Gateway DNS-protocol hijack is not first after sniff: %#v", got)
	}
	if got := rules[3]; got.Action != "hijack-dns" || len(got.Inbound) != 1 || got.Inbound[0] != GatewayInboundTag || len(got.Port) != 1 || got.Port[0] != 53 {
		t.Fatalf("Gateway port-53 DNS hijack is not before policy rules: %#v", got)
	}
	if got := rules[4]; got.Action != "route" || got.Outbound != "vpn" || len(got.DomainSuffix) != 1 || got.DomainSuffix[0] != "policy-vpn.example" {
		t.Fatalf("first existing policy rule lost or reordered: %#v", got)
	}
	if got := rules[5]; got.Action != "route" || got.Outbound != "direct" || len(got.Port) != 1 || got.Port[0] != 8443 {
		t.Fatalf("second existing policy rule lost or reordered: %#v", got)
	}
	if got := rules[6]; got.Action != "route" || got.Outbound != "direct" || len(got.Inbound) != 1 || got.Inbound[0] != GatewayInboundTag || len(got.Port) != 0 || len(got.DomainSuffix) != 0 || len(got.SourceIPCIDR) != 0 {
		t.Fatalf("Gateway direct catch-all fallback lost or moved ahead of special rules: %#v", got)
	}
	if got := rules[7]; got.Action != "route" || got.Outbound != "vpn" || len(got.DomainSuffix) != 1 || got.DomainSuffix[0] != "dns-route-first.example" {
		t.Fatalf("first later DNS-route rule lost or reordered: %#v", got)
	}
	if got := rules[8]; got.Action != "route" || got.Outbound != "direct" || len(got.DomainSuffix) != 1 || got.DomainSuffix[0] != "dns-route-last.example" {
		t.Fatalf("last DNS-route rule lost or reordered: %#v", got)
	}
}
