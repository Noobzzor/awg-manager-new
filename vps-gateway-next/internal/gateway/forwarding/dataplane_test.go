package forwarding

import (
	"net/netip"
	"strings"
	"testing"
)

func TestRenderRulesetFailsClosedOutsidePolicyTUN(t *testing.T) {
	script, err := RenderRuleset(netip.MustParsePrefix("10.66.0.0/24"), "eth0")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"chain input",
		"iifname \"awg0\" drop",
		"iifname \"awg0\" ip saddr != 10.66.0.0/24 drop",
		`iifname "awg0" ip saddr 10.66.0.0/24 oifname "awgm0" accept`,
		`iifname "awg0" ip saddr 10.66.0.0/24 ip daddr 10.66.0.0/24 drop`,
		`iifname "awgm0" ip daddr 10.66.0.0/24 oifname "awg0" ct state established,related accept`,
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("ruleset missing %q: %s", want, script)
		}
	}
	if strings.Contains(strings.ToLower(script), "masquerade") {
		t.Fatalf("client traffic must not be NATed directly to WAN: %s", script)
	}
	if strings.Contains(script, `iifname "awgm0" oifname "eth0" accept`) {
		t.Fatalf("policy TUN packets must not be forwarded directly to WAN: %s", script)
	}
	spoofDrop := strings.Index(script, "ip saddr != 10.66.0.0/24 drop")
	if spoofDrop < 0 {
		t.Fatalf("anti-spoofing rule missing: %s", script)
	}
}
