package policy

import (
	"encoding/json"
	"testing"
)

func TestRenderSlotSniffsGatewayBeforeDNSAndDomainPolicy(t *testing.T) {
	compiled, err := Compile(Profile{ID: "sniff", Name: "Sniff", DefaultAction: ActionDirect,
		Rules: []Rule{{ID: "domain-block", Enabled: true, Action: ActionBlock, DomainSuffixes: []string{"blocked.gateway.test"}}}},
		CompileOptions{Outbounds: map[string]struct{}{"direct": {}}, OutboundActions: map[string]Action{"direct": ActionDirect}, DefaultOutbounds: map[Action]string{ActionDirect: "direct"}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := RenderSlot(compiled)
	if err != nil {
		t.Fatal(err)
	}
	var slot slotConfig
	if err := json.Unmarshal(data, &slot); err != nil {
		t.Fatal(err)
	}
	rules := slot.Route.Rules
	if len(rules) != 5 || rules[0].Action != "sniff" {
		t.Fatalf("Gateway must sniff before DNS classification/domain match/fallback: %s", data)
	}
	for _, rule := range rules {
		if len(rule.Inbound) != 1 || rule.Inbound[0] != GatewayInboundTag {
			t.Fatalf("Gateway rule leaks to another inbound: %#v", rule)
		}
	}
	if rules[0].Outbound != "" || rules[0].Protocol != "" || len(rules[0].Port) != 0 || len(rules[0].DomainSuffix) != 0 {
		t.Fatalf("sniff action must be non-terminal and unconditional within Gateway: %#v", rules[0])
	}
	if rules[1].Action != "hijack-dns" || rules[1].Protocol != "dns" || rules[2].Action != "hijack-dns" || len(rules[2].Port) != 1 || rules[2].Port[0] != 53 {
		t.Fatalf("DNS hijacks must follow sniff and precede policy: %#v", rules)
	}
	if rules[3].Action != "reject" || len(rules[3].DomainSuffix) != 1 || rules[3].DomainSuffix[0] != "blocked.gateway.test" || rules[4].Action != "route" || rules[4].Outbound != "direct" {
		t.Fatalf("domain policy must precede terminal fallback: %#v", rules)
	}
}
