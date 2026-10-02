package forwarding

import (
	"net/netip"
	"strings"
	"testing"
)

func TestRenderRulesetHasOwnershipAndKeepsClientOffDirectWANPath(t *testing.T) {
	script, err := RenderRuleset(netip.MustParsePrefix("10.66.0.0/24"), "eth0")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"table inet awgm_gateway",
		"comment \"AWGM_GATEWAY_OWNER\"",
		"iifname \"awg0\" drop",
		"iifname \"awg0\" ip saddr 10.66.0.0/24 oifname \"awgm0\" accept",
		"iifname \"awg0\" ip saddr 10.66.0.0/24 ip daddr 10.66.0.0/24 drop",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("ruleset missing %q: %s", want, script)
		}
	}
	if strings.Contains(strings.ToLower(script), "masquerade") {
		t.Fatalf("client traffic must not bypass policy TUN through WAN NAT: %s", script)
	}
}

func TestRenderRulesetAllowsOnlyDNATedTCPFromClientPoolToLocalIngress(t *testing.T) {
	script, err := RenderRuleset(netip.MustParsePrefix("10.66.0.0/24"), "eth0")
	if err != nil {
		t.Fatal(err)
	}
	allowRule := `iifname "awg0" ip saddr 10.66.0.0/24 meta l4proto tcp ct status dnat fib daddr type local accept`
	dropRule := `iifname "awg0" drop`
	allowAt := strings.Index(script, allowRule)
	dropAt := strings.Index(script, dropRule)
	if allowAt < 0 || dropAt < 0 || allowAt >= dropAt {
		t.Fatalf("only local, pool-sourced, DNATed TCP may pass before the AWG input drop: %s", script)
	}
	if strings.Contains(script, `iifname "awg0" accept`) {
		t.Fatalf("AWG input must not have an unconditional accept: %s", script)
	}
}

func TestRenderRulesetRejectsInvalidPoolAndInterface(t *testing.T) {
	if _, err := RenderRuleset(netip.MustParsePrefix("10.66.0.0/31"), "eth0"); err == nil {
		t.Fatal("expected invalid pool")
	}
	if _, err := RenderRuleset(netip.MustParsePrefix("10.66.0.0/24"), "bad iface; rm"); err == nil {
		t.Fatal("expected invalid interface")
	}
}

func TestRenderRulesetAllowsCustomGatewayInterfaces(t *testing.T) {
	script, err := RenderRuleset(netip.MustParsePrefix("10.66.0.0/24"), "eth0", "wg-gateway", "sb-gateway")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`iifname "wg-gateway" drop`, `oifname "sb-gateway" accept`, `iifname "wg-gateway" oifname "sb-gateway" ct state established,related accept`} {
		if !strings.Contains(script, want) {
			t.Fatalf("ruleset missing custom interface rule %q: %s", want, script)
		}
	}
}

func TestRenderRulesetKeepsClientGuardsAheadOfEstablishedTraffic(t *testing.T) {
	script, err := RenderRuleset(netip.MustParsePrefix("10.66.0.0/24"), "eth0")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(script, "%!") {
		t.Fatalf("ruleset contains fmt formatting diagnostics: %s", script)
	}
	for _, forbidden := range []string{
		"	  ct state established,related accept",
		`iifname "awg0" ct state established,related accept`,
		"ip daddr != 10.66.0.0/24 drop",
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("unsafe or inverted guard %q in ruleset: %s", forbidden, script)
		}
	}
	spoof := strings.Index(script, "ip saddr != 10.66.0.0/24 drop")
	isolation := strings.Index(script, "ip daddr 10.66.0.0/24 drop")
	policyEgress := strings.Index(script, `oifname "awgm0" accept`)
	if spoof < 0 || isolation < 0 || policyEgress < 0 || spoof > policyEgress || isolation > policyEgress {
		t.Fatalf("client source/destination guards must precede policy egress: %s", script)
	}
	for _, want := range []string{
		`iifname "awgm0" ip daddr 10.66.0.0/24 oifname "awg0" ct state established,related accept`,
		`iifname "awg0" oifname "awgm0" ct state established,related accept`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("scoped return rule %q missing: %s", want, script)
		}
	}
}

func TestRenderRulesetRejectsDirectClientWANPath(t *testing.T) {
	const pool = "10.66.0.0/24"
	script, err := RenderRuleset(netip.MustParsePrefix(pool), "eth0")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		`iifname "awg0" ip saddr 10.66.0.0/24 accept`,
		`ip saddr 10.66.0.0/24 oifname "eth0" masquerade`,
	} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("direct client WAN path %q must not be rendered: %s", forbidden, script)
		}
	}
}

func TestRenderRulesetRejectsPolicyInterfaceMatchingWAN(t *testing.T) {
	if _, err := RenderRuleset(netip.MustParsePrefix("10.66.0.0/24"), "eth0", "awg0", "eth0"); err == nil {
		t.Fatal("policy interface matching WAN must be rejected to prevent bypassing policy TUN")
	}
}
