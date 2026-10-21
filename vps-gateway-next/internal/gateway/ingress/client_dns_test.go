package ingress

import (
	"net/netip"
	"strings"
	"testing"
)

func TestRegistryExportProvidesDefaultDNSForPhone(t *testing.T) {
	for _, awg := range []bool{false, true} {
		name := "wg"
		if awg {
			name = "awg"
		}
		t.Run(name, func(t *testing.T) {
			allocator, _ := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
			registry := NewRegistry(allocator, func(string) (string, string, error) {
				return "synthetic-client-private", "synthetic-client-public", nil
			})
			if _, err := registry.Create("phone", "Phone"); err != nil {
				t.Fatal(err)
			}
			if awg {
				if err := registry.UseAWGParameters(testedAWGParameters()); err != nil {
					t.Fatal(err)
				}
			}
			profile, err := registry.RenderClientConfig("phone", "synthetic-server-public", "198.51.100.10:51820")
			if err != nil {
				t.Fatal(err)
			}
			iface, peer, ok := strings.Cut(profile, "\n\n[Peer]\n")
			if !ok || !strings.Contains("\n"+iface+"\n", "\nDNS = 1.1.1.1\n") || strings.Contains(peer, "DNS =") || strings.Count(profile, "DNS =") != 1 {
				t.Fatal("device export must include exactly one explicit default DNS in Interface")
			}
			if !strings.Contains(iface, "PrivateKey = synthetic-client-private") || !strings.Contains(peer, "Endpoint = 198.51.100.10:51820") {
				t.Fatal("DNS correction changed identity or endpoint")
			}
			if awg && (!strings.Contains(iface, "Jc = ") || !strings.Contains(iface, "S4 = ") || !strings.Contains(iface, "H4 = ")) {
				t.Fatal("DNS correction lost AWG parameters")
			}
		})
	}
}

func TestClientDNSValidationAndStartupOverride(t *testing.T) {
	for _, raw := range []string{"::1", "2001:db8::1", "0.0.0.0", "127.0.0.1", "224.0.0.1", "255.255.255.255", "bad", "1.1.1.1\nPrivateKey = injected"} {
		if _, err := ParseClientDNS(raw); err == nil {
			t.Errorf("accepted invalid DNS %q", raw)
		}
	}
	dns, err := ParseClientDNS(" 9.9.9.9 ")
	if err != nil {
		t.Fatal(err)
	}
	allocator, _ := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	registry := NewRegistry(allocator, func(string) (string, string, error) { return "synthetic-private", "synthetic-public", nil })
	if _, err := registry.Create("phone", "Phone"); err != nil {
		t.Fatal(err)
	}
	if err := registry.UseClientDNS(dns); err != nil {
		t.Fatal(err)
	}
	if err := registry.UseClientDNS(dns); err != nil {
		t.Fatal(err)
	}
	if err := registry.UseClientDNS(netip.MustParseAddr("1.0.0.1")); err == nil {
		t.Fatal("changed live DNS")
	}
	if err := registry.UseClientDNS(netip.Addr{}); err == nil {
		t.Fatal("accepted invalid address")
	}
	profile, err := registry.RenderClientConfig("phone", "synthetic-server", "198.51.100.10:51820")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(profile, "\nDNS = 9.9.9.9\n") || strings.Contains(profile, "DNS = 1.1.1.1") {
		t.Fatal("configured DNS lost or duplicated")
	}
	var absent *Registry
	if err := absent.UseClientDNS(dns); err == nil {
		t.Fatal("nil registry accepted DNS")
	}
}
