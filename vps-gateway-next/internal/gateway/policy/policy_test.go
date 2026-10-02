package policy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProfileValidateAcceptsFourActions(t *testing.T) {
	profile := Profile{
		ID:            "phone-default",
		Name:          "Phone default",
		DefaultAction: ActionVPN,
		Rules: []Rule{
			{ID: "warp-sites", Action: ActionWARP, DomainSuffixes: []string{"example.com"}, Outbound: "warp"},
			{ID: "local", Action: ActionDirect, CIDRs: []string{"192.0.2.0/24"}, Priority: 20, Outbound: "direct"},
			{ID: "blocked", Action: ActionBlock, Ports: []int{25}, Protocols: []string{"tcp"}, Priority: 30},
		},
	}

	if err := profile.Validate(); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
}

func TestProfileValidateRejectsNetworkValuesNotSupportedByPinnedSingBox(t *testing.T) {
	for _, protocol := range []string{"icmpv4", "icmpv6", "TCP"} {
		t.Run(protocol, func(t *testing.T) {
			profile := Profile{
				ID:            "p",
				Name:          "P",
				DefaultAction: ActionDirect,
				Rules: []Rule{{
					ID: "r", Action: ActionDirect, Outbound: "direct", Protocols: []string{protocol},
				}},
			}
			if err := profile.Validate(); err == nil {
				t.Fatalf("unsupported network value %q was accepted", protocol)
			}
		})
	}
}

func TestProfileValidateRejectsUnknownAction(t *testing.T) {
	profile := Profile{ID: "p", Name: "P", DefaultAction: Action("proxy")}

	err := profile.Validate()
	if err == nil || !strings.Contains(err.Error(), "default action") {
		t.Fatalf("expected default action error, got %v", err)
	}
}

func TestProfileValidateRejectsBlockOutbound(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{{
			ID:       "blocked",
			Action:   ActionBlock,
			Outbound: "direct",
		}},
	}

	err := profile.Validate()
	if err == nil || !strings.Contains(err.Error(), "block action") {
		t.Fatalf("expected block outbound error, got %v", err)
	}
}

func TestProfileValidateRejectsInvalidPortAndCIDR(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{{
			ID:       "bad",
			Action:   ActionVPN,
			CIDRs:    []string{"not-a-cidr"},
			Ports:    []int{70000},
			Outbound: "vpn-1",
		}},
	}

	err := profile.Validate()
	if err == nil || !strings.Contains(err.Error(), "CIDR") {
		t.Fatalf("expected CIDR error first, got %v", err)
	}
}

func TestProfileValidateRejectsDuplicateRuleID(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{
			{ID: "same", Action: ActionDirect, Outbound: "direct"},
			{ID: "same", Action: ActionWARP, Outbound: "warp"},
		},
	}

	err := profile.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate rule id") {
		t.Fatalf("expected duplicate rule error, got %v", err)
	}
}

func TestProfileValidateAcceptsClientAndGroupSelectors(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{{
			ID:        "phone-warp",
			Action:    ActionWARP,
			ClientIDs: []string{"phone"},
			GroupIDs:  []string{"family"},
			Outbound:  "warp",
		}},
	}

	if err := profile.Validate(); err != nil {
		t.Fatalf("client/group selectors rejected: %v", err)
	}
}

func TestProfileOrderedRulesSortsByPriorityStably(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{
			{ID: "late", Action: ActionVPN, Priority: 20, Outbound: "vpn"},
			{ID: "first", Action: ActionWARP, Priority: 10, Outbound: "warp"},
			{ID: "same-priority", Action: ActionBlock, Priority: 10},
		},
	}

	ordered, err := profile.OrderedRules()
	if err != nil {
		t.Fatalf("order rejected: %v", err)
	}
	want := []string{"first", "same-priority", "late"}
	for i, rule := range ordered {
		if rule.ID != want[i] {
			t.Fatalf("ordered[%d] = %q, want %q", i, rule.ID, want[i])
		}
	}

	ordered[0].ID = "mutated-copy"
	if profile.Rules[1].ID != "first" {
		t.Fatal("OrderedRules returned aliases to profile rules")
	}
}

func TestProfileValidateRejectsNegativePriority(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules:         []Rule{{ID: "bad", Action: ActionDirect, Priority: -1, Outbound: "direct"}},
	}

	if err := profile.Validate(); err == nil || !strings.Contains(err.Error(), "priority") {
		t.Fatalf("expected priority error, got %v", err)
	}
}

func TestCompileResolvesSourcesAndActions(t *testing.T) {
	profile := Profile{
		ID:            "phone-default",
		Name:          "Phone default",
		DefaultAction: ActionVPN,
		Rules: []Rule{
			{ID: "warp", Action: ActionWARP, ClientIDs: []string{"phone"}, DomainSuffixes: []string{"example.com"}, Outbound: "warp", Priority: 10, Enabled: true},
			{ID: "block", Action: ActionBlock, GroupIDs: []string{"family"}, Ports: []int{25}, Priority: 20, Enabled: true},
		},
	}

	compiled, err := Compile(profile, CompileOptions{
		Outbounds:        map[string]struct{}{"vpn": {}, "warp": {}},
		OutboundActions:  map[string]Action{"vpn": ActionVPN, "warp": ActionWARP},
		DefaultOutbounds: map[Action]string{ActionVPN: "vpn"},
		Clients:          map[string][]string{"phone": {"10.0.0.2/32"}},
		Groups:           map[string][]string{"family": {"10.0.0.0/24"}},
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if compiled.Final != "vpn" {
		t.Fatalf("final outbound = %q, want vpn", compiled.Final)
	}
	if len(compiled.Rules) != 2 {
		t.Fatalf("compiled rules = %d, want 2", len(compiled.Rules))
	}
	if got := compiled.Rules[0].SourceIPCIDR; len(got) != 1 || got[0] != "10.0.0.2/32" {
		t.Fatalf("source CIDRs = %#v, want phone address", got)
	}
	if compiled.Rules[0].Action != "route" || compiled.Rules[0].Outbound != "warp" {
		t.Fatalf("warp rule = %#v", compiled.Rules[0])
	}
	if compiled.Rules[1].Action != "reject" || compiled.Rules[1].Outbound != "" {
		t.Fatalf("block rule = %#v", compiled.Rules[1])
	}

}

func TestCompileSkipsDisabledRulesWithoutResolvingTheirSources(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{{
			ID:        "disabled",
			Action:    ActionBlock,
			ClientIDs: []string{"removed-client"},
			Enabled:   false,
		}},
	}
	compiled, err := Compile(profile, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err != nil {
		t.Fatalf("disabled rule should not block compile: %v", err)
	}
	if len(compiled.Rules) != 0 {
		t.Fatalf("compiled disabled rule: %#v", compiled.Rules)
	}
}

func TestCompileRejectsOutboundThatDoesNotMatchAction(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{{
			ID:       "vpn-via-direct",
			Action:   ActionVPN,
			Outbound: "direct",
			Enabled:  true,
			Domains:  []string{"example.com"},
		}},
	}
	_, err := Compile(profile, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match action") {
		t.Fatalf("expected action/outbound mismatch error, got %v", err)
	}
}

func TestCompileRejectsMissingOutbound(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules:         []Rule{{ID: "vpn", Action: ActionVPN, Outbound: "missing", Enabled: true}},
	}

	_, err := Compile(profile, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err == nil || !strings.Contains(err.Error(), "missing outbound") {
		t.Fatalf("expected missing outbound error, got %v", err)
	}
}

func TestCompileRejectsUnresolvedSource(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules:         []Rule{{ID: "warp", Action: ActionWARP, ClientIDs: []string{"unknown"}, Outbound: "warp", Enabled: true}},
	}

	_, err := Compile(profile, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}, "warp": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect, "warp": ActionWARP},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown client") {
		t.Fatalf("expected unknown client error, got %v", err)
	}
}

func TestCompilePreservesRuleSets(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{{
			ID:       "geo-vpn",
			Action:   ActionVPN,
			RuleSets: []string{"ru-domains", "streaming"},
			Outbound: "vpn",
			Enabled:  true,
		}},
	}

	compiled, err := Compile(profile, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}, "vpn": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect, "vpn": ActionVPN},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	want := []string{"ru-domains", "streaming"}
	if len(compiled.Rules) != 1 || len(compiled.Rules[0].RuleSet) != 2 {
		t.Fatalf("compiled rulesets = %#v", compiled.Rules)
	}
	for i, got := range compiled.Rules[0].RuleSet {
		if got != want[i] {
			t.Fatalf("ruleset[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestCompileMapsTransportProtocolToNetwork(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{{
			ID:        "tcp-only",
			Action:    ActionBlock,
			Protocols: []string{"tcp"},
			Enabled:   true,
		}},
	}
	compiled, err := Compile(profile, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if len(compiled.Rules) != 1 {
		t.Fatalf("compiled rules = %d, want 1", len(compiled.Rules))
	}

	data, err := json.Marshal(compiled.Rules[0])
	if err != nil {
		t.Fatalf("marshal compiled rule: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal compiled rule: %v", err)
	}
	var network []string
	if err := json.Unmarshal(got["network"], &network); err != nil {
		t.Fatalf("decode network matcher from %s: %v", data, err)
	}
	if len(network) != 1 || network[0] != "tcp" {
		t.Fatalf("network = %#v, want [tcp] in %s", network, data)
	}
	if _, exists := got["protocol"]; exists {
		t.Fatalf("unexpected protocol matcher in %s", data)
	}
}

func TestRenderSlotProducesDeterministicSingboxJSON(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{{
			ID:             "warp",
			Action:         ActionWARP,
			DomainSuffixes: []string{"example.com"},
			Outbound:       "warp",
			Enabled:        true,
		}},
	}
	compiled, err := Compile(profile, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}, "warp": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect, "warp": ActionWARP},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	first, err := RenderSlot(compiled)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	second, err := RenderSlot(compiled)
	if err != nil {
		t.Fatalf("second render failed: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("render is not deterministic:\n%s\n---\n%s", first, second)
	}
	if strings.Contains(string(first), `"final"`) || !strings.Contains(string(first), `"domain_suffix"`) || !strings.Contains(string(first), `"inbounds"`) {
		t.Fatalf("rendered slot lacks expected route fields: %s", first)
	}
}
