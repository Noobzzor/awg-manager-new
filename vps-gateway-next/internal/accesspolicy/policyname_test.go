package accesspolicy

import "testing"

func TestIsStandardPolicyName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"Policy0", true},
		{"Policy1", true},
		{"Policy42", true},
		{"Policy63", true},
		// HR-NEO and other custom names — NDMS allows arbitrary identifiers
		// for non-Policy policies; they share /show/ip/policy storage but
		// must not be treated as built-in PolicyN.
		{"germany-vpn", false},
		{"HydraRoute", false},
		{"HrNeoUS", false},
		{"", false},
		{"Policy", false},    // prefix only, no number
		{"PolicyABC", false}, // suffix is not a number
		{"Policy-1", false},  // strconv.Atoi accepts "-1"; filter rejects negatives
		{"Policy+1", false},  // strconv.Atoi accepts "+1"; filter rejects signed prefix
		{"Policy 1", false},  // space inside
		{"policy0", false},   // case-sensitive prefix
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsStandardPolicyName(c.name); got != c.want {
				t.Errorf("IsStandardPolicyName(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}
