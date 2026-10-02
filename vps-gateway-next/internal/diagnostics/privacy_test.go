package diagnostics

import "testing"

func TestMaskHumanLabel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"ISP", "***"},
		{"WAN 1", "*****"},
		{"beeline home 0898616286", "bee*****************286"},
		{"Домашний интернет Иван", "Дом****************ван"},
	}

	for _, tc := range cases {
		if got := maskHumanLabel(tc.in); got != tc.want {
			t.Fatalf("maskHumanLabel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSanitizeReportPrivacy_MasksAllInterfaceLabels(t *testing.T) {
	report := &Report{
		WAN: WANInfo{
			Interfaces: map[string]WANIfaceInfo{
				"apcli0": {Up: false, Label: "Wi-Fi клиент 2.4 ГГц"},
				"eth3":   {Up: true, Label: "beeline home 0898616286"},
				"t2s0":   {Up: true, Label: "vless-tcp-reality - Cloned-vopxs8ja"},
				"t2s1":   {Up: true, Label: "iq0"},
			},
		},
	}

	sanitizeReportPrivacy(report)

	got := report.WAN.Interfaces

	if got["apcli0"].Label != "Wi-**************ГГц" {
		t.Fatalf("apcli0 label = %q", got["apcli0"].Label)
	}
	if got["eth3"].Label != "bee*****************286" {
		t.Fatalf("eth3 label = %q", got["eth3"].Label)
	}
	if got["t2s0"].Label != "vle*****************************8ja" {
		t.Fatalf("t2s0 label = %q", got["t2s0"].Label)
	}
	if got["t2s1"].Label != "***" {
		t.Fatalf("t2s1 label = %q", got["t2s1"].Label)
	}

	if !report.Privacy.Sanitized {
		t.Fatalf("privacy.sanitized = false")
	}
	if len(report.Privacy.Rules) == 0 || report.Privacy.Rules[0] != "wan-interfaces-labels" {
		t.Fatalf("privacy.rules = %#v", report.Privacy.Rules)
	}
}
