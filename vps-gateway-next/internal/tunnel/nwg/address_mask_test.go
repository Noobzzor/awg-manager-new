package nwg

import "testing"

// issue #531: пользовательский CIDR (172.25.1.2/24) молча превращался в /32 —
// extractIPv4 срезал суффикс до того, как splitAddressMask видел маску.
func TestAddressMask_UserCIDRPreserved(t *testing.T) {
	cases := []struct {
		in       string
		wantAddr string
		wantMask string
	}{
		{"172.25.1.2/24", "172.25.1.2", "255.255.255.0"},                // кейс из issue
		{"10.0.0.2/32", "10.0.0.2", "255.255.255.255"},                  // явный /32
		{"10.0.0.2", "10.0.0.2", "255.255.255.255"},                     // без маски → дефолт /32
		{"172.16.0.2/16, 2606::1/128", "172.16.0.2", "255.255.0.0"},     // v6 в списке пропускается
		{"2606::1/128, 192.168.7.1/24", "192.168.7.1", "255.255.255.0"}, // v6 первым
	}
	for _, c := range cases {
		addr, mask := splitAddressMask(extractIPv4(c.in))
		if addr != c.wantAddr || mask != c.wantMask {
			t.Errorf("%q: got (%s, %s), want (%s, %s)", c.in, addr, mask, c.wantAddr, c.wantMask)
		}
	}
}

func TestExtractIPv4_KeepsCIDRSuffix(t *testing.T) {
	if got := extractIPv4("172.25.1.2/24"); got != "172.25.1.2/24" {
		t.Fatalf("suffix must be preserved, got %q", got)
	}
}
