package policy

import "testing"

func TestAddWARPRegistersOnlyValidatedProvider(t *testing.T) {
	options := CompileOptions{Outbounds: map[string]struct{}{"direct": {}}, OutboundActions: map[string]Action{"direct": ActionDirect}, DefaultOutbounds: map[Action]string{ActionDirect: "direct"}}
	cfg := WARPConfig{Tag: "warp", Endpoint: "engage.cloudflareclient.com:2408", MTU: 1280}
	if err := AddWARP(&options, cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := options.Outbounds["warp"]; !ok || options.DefaultOutbounds[ActionWARP] != "warp" {
		t.Fatalf("WARP not registered: %+v", options)
	}
}

func TestAddWARPRejectsInvalidMTUWithoutMutation(t *testing.T) {
	options := CompileOptions{Outbounds: map[string]struct{}{"direct": {}}, OutboundActions: map[string]Action{"direct": ActionDirect}, DefaultOutbounds: map[Action]string{ActionDirect: "direct"}}
	if err := AddWARP(&options, WARPConfig{Tag: "warp", Endpoint: "x", MTU: 900}); err == nil {
		t.Fatal("expected invalid MTU error")
	}
	if _, ok := options.Outbounds["warp"]; ok {
		t.Fatal("invalid WARP was added")
	}
}

func TestAddWARPRejectsReservedOrConflictingTagWithoutMutation(t *testing.T) {
	for _, tag := range []string{"direct", "awg1"} {
		t.Run(tag, func(t *testing.T) {
			options := CompileOptions{
				Outbounds:        map[string]struct{}{"direct": {}, "awg1": {}},
				OutboundActions:  map[string]Action{"direct": ActionDirect, "awg1": ActionVPN},
				DefaultOutbounds: map[Action]string{ActionDirect: "direct", ActionVPN: "awg1"},
			}
			if err := AddWARP(&options, WARPConfig{Tag: tag, Endpoint: "engage.cloudflareclient.com:2408", MTU: 1280}); err == nil {
				t.Fatalf("expected tag %q to be rejected", tag)
			}
			if options.OutboundActions[tag] == ActionWARP || options.DefaultOutbounds[ActionWARP] != "" {
				t.Fatalf("rejected WARP tag mutated compile options: %+v", options)
			}
		})
	}
}
