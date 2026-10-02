package forwarding

import (
	"net/netip"
	"testing"
)

func TestRenderPolicyAddsDNSAndMSSBaseline(t *testing.T) {
	policy, err := NewPolicy(netip.MustParseAddr("1.1.1.1"), 1380)
	if err != nil {
		t.Fatal(err)
	}
	script, err := RenderPolicyRuleset(policy, netip.MustParsePrefix("10.66.0.0/24"), "eth0")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"ip daddr 1.1.1.1 udp dport 53 accept",
		"ip daddr 1.1.1.1 tcp dport 53 accept",
		"tcp flags syn tcp option maxseg size set 1340",
	} {
		if !contains(script, want) {
			t.Fatalf("policy ruleset missing %q: %s", want, script)
		}
	}
}

func TestNewPolicyRejectsUnsafeMTU(t *testing.T) {
	for _, mtu := range []int{0, 1279, 1501} {
		if _, err := NewPolicy(netip.MustParseAddr("1.1.1.1"), mtu); err == nil {
			t.Fatalf("expected MTU rejection for %d", mtu)
		}
	}
}

func contains(s, want string) bool { return len(want) <= len(s) && index(s, want) >= 0 }
func index(s, want string) int {
	for i := 0; i+len(want) <= len(s); i++ {
		if s[i:i+len(want)] == want {
			return i
		}
	}
	return -1
}
