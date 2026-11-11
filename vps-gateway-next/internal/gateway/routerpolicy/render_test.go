package routerpolicy

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/gateway/policy"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

func TestOriginalRulesPrecedeScopedGatewayFallback(t *testing.T) {
	cfg := router.NewEmptyConfig()
	cfg.Route.Rules = []router.Rule{
		{Type: "logical", Mode: "or", Invert: true, Rules: []router.Rule{{DomainSuffix: []string{"fixture.example"}}, {IPCIDR: []string{"192.0.2.0/24"}}}, Action: "reject"},
		{Inbound: []string{"other-in"}, SourceIPCIDR: []string{"10.66.0.2/32"}, Port: []int{443}, Outbound: "vpn-fixture"},
	}
	before, _ := json.Marshal(cfg)
	raw, err := Render("awg0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Inbounds []map[string]any `json:"inbounds"`
		Route    router.Route     `json:"route"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Route.Rules) != 6 {
		t.Fatalf("rules=%d; want sniff+DNS+DNS+two original rules+fallback", len(got.Route.Rules))
	}
	if got.Route.Final != "" {
		t.Fatal("gateway projection must not install a global route.final")
	}
	if got.Inbounds[0]["dns_mode"] != "disabled" || !reflect.DeepEqual(got.Inbounds[0]["include_interface"], []any{"awg0"}) {
		t.Fatal("original gateway TUN constraints were lost")
	}
	for i, original := range cfg.Route.Rules {
		rule := got.Route.Rules[i+3]
		if rule.Type != "logical" || rule.Mode != "and" || rule.Invert || len(rule.Rules) != 2 {
			t.Fatal("gateway scope must enclose original matcher, including its inversion")
		}
		if !reflect.DeepEqual(rule.Rules[0].Inbound, []string{policy.GatewayInboundTag}) {
			t.Fatal("rule can capture unrelated ingress")
		}
		match := original
		match.Action = ""
		match.Outbound = ""
		match.UDPTimeout = ""
		if !reflect.DeepEqual(rule.Rules[1], match) || rule.Action != original.Action || rule.Outbound != original.Outbound {
			t.Fatal("original logical/matcher/action semantics changed")
		}
	}
	last := got.Route.Rules[5]
	if !reflect.DeepEqual(last.Inbound, []string{policy.GatewayInboundTag}) || last.Outbound != "direct" {
		t.Fatal("scoped fallback missing or misplaced")
	}
	after, _ := json.Marshal(cfg)
	if string(before) != string(after) {
		t.Fatal("render mutated editor config")
	}
}

func TestOriginalProjectionRetainsCatalogResourcesAndDNS(t *testing.T) {
	cfg := router.NewEmptyConfig()
	cfg.Route.Final = "block"
	cfg.Route.RuleSet = []router.RuleSet{{Tag: "fixture-set", Type: "remote", Format: "binary", URL: "https://example.invalid/fixture.srs"}}
	cfg.Outbounds = []router.Outbound{{Type: "selector", Tag: "fixture-selector", Outbounds: []string{"direct"}}}
	cfg.DNS.Servers = []router.DNSServer{{Type: "udp", Tag: "fixture-dns", Server: "192.0.2.53"}}
	cfg.DNS.Final = "fixture-dns"
	raw, err := Render("awg0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	var got router.RouterConfig
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	wantDNS, _ := json.Marshal(cfg.DNS)
	gotDNS, _ := json.Marshal(got.DNS)
	if string(wantDNS) != string(gotDNS) {
		t.Fatalf("original DNS changed: %s != %s", gotDNS, wantDNS)
	}
	if !reflect.DeepEqual(got.Outbounds, cfg.Outbounds) || !reflect.DeepEqual(got.Route.RuleSet, cfg.Route.RuleSet) {
		t.Fatal("original outbounds/rule-set definitions lost")
	}
	last := got.Route.Rules[len(got.Route.Rules)-1]
	if last.Action != "reject" || last.Outbound != "" {
		t.Fatal("BLOCK default became a route")
	}
}

func TestOriginalProjectionRefusesForeignRouterInbounds(t *testing.T) {
	cfg := router.NewEmptyConfig()
	cfg.Inbounds = []router.Inbound{{Type: "tun", Tag: "legacy-router"}}
	if _, err := Render("awg0", cfg); err == nil {
		t.Fatal("legacy platform inbound must not replace gateway ingress")
	}
}
