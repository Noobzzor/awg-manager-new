package policy

import "testing"

func TestAnalyzeFindsDuplicateMatcher(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{
			{ID: "first", Action: ActionVPN, DomainSuffixes: []string{"example.com"}, Outbound: "vpn", Enabled: true},
			{ID: "second", Action: ActionWARP, DomainSuffixes: []string{"example.com"}, Outbound: "warp", Priority: 10, Enabled: true},
		},
	}

	issues, err := Analyze(profile)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if len(issues) != 1 || issues[0].Kind != IssueDuplicateMatcher || issues[0].RuleID != "second" {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestAnalyzeFindsRuleShadowedByCatchAll(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{
			{ID: "all", Action: ActionVPN, Outbound: "vpn", Enabled: true},
			{ID: "specific", Action: ActionWARP, DomainSuffixes: []string{"example.com"}, Outbound: "warp", Priority: 10, Enabled: true},
		},
	}

	issues, err := Analyze(profile)
	if err != nil {
		t.Fatalf("analyze failed: %v", err)
	}
	if len(issues) != 1 || issues[0].Kind != IssueShadowed || issues[0].RuleID != "specific" {
		t.Fatalf("issues = %#v", issues)
	}
}

func TestAnalyzeAllowsDifferentMatchers(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{
			{ID: "web", Action: ActionVPN, Protocols: []string{"tcp"}, Outbound: "vpn", Enabled: true},
			{ID: "dns", Action: ActionWARP, Protocols: []string{"udp"}, Outbound: "warp", Priority: 10, Enabled: true},
		},
	}

	issues, err := Analyze(profile)
	if err != nil || len(issues) != 0 {
		t.Fatalf("issues=%#v err=%v", issues, err)
	}
}

func TestAnalyzeIgnoresDisabledCatchAllRules(t *testing.T) {
	profile := Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{
			{ID: "disabled-all", Action: ActionVPN, Outbound: "vpn", Enabled: false},
			{ID: "specific", Action: ActionWARP, DomainSuffixes: []string{"example.com"}, Outbound: "warp", Enabled: true},
		},
	}
	issues, err := Analyze(profile)
	if err != nil || len(issues) != 0 {
		t.Fatalf("issues=%#v err=%v", issues, err)
	}
}
