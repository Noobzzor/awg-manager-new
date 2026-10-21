package ingress

import (
	"net/netip"
	"reflect"
	"strings"
	"testing"
)

func testedAWGParameters() AWGParameters {
	return AWGParameters{Jc: 3, Jmin: 40, Jmax: 80, S1: 16, S2: 16, S3: 16, S4: 16, H1: 1001, H2: 1002, H3: 1003, H4: 1004}
}

func TestAWGClientExportUsesSameParametersAsServerArguments(t *testing.T) {
	params := testedAWGParameters()
	args, err := params.SetArguments()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"jc", "3", "jmin", "40", "jmax", "80", "s1", "16", "s2", "16", "s3", "16", "s4", "16", "h1", "1001", "h2", "1002", "h3", "1003", "h4", "1004"}
	if !reflect.DeepEqual(args, want) {
		t.Fatal("AWG argument fields differ from the tested server profile")
	}
	client, err := NewClient("test-client", "Synthetic", netip.MustParseAddr("10.66.0.2"), "synthetic-test-private", "synthetic-test-public")
	if err != nil {
		t.Fatal(err)
	}
	conf, err := RenderAWGClientConfig(client, "synthetic-server-public", "198.51.100.10:51820", "10.66.0.2/32", "", params)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(conf, "\n\n[Peer]\n")
	if len(parts) != 2 {
		t.Fatal("invalid client configuration section boundary")
	}
	for _, field := range []string{"Jc = 3", "Jmin = 40", "Jmax = 80", "S1 = 16", "S2 = 16", "S3 = 16", "S4 = 16", "H1 = 1001", "H2 = 1002", "H3 = 1003", "H4 = 1004"} {
		if strings.Count(parts[0]+"\n", "\n"+field+"\n") != 1 || strings.Contains(parts[1], field) {
			t.Fatalf("AWG field missing, duplicated or in wrong section: %s", field)
		}
	}
	if strings.Contains(strings.ToLower(conf), "advancedsecurity") || strings.Contains(strings.ToLower(strings.Join(args, " ")), "advanced-security") {
		t.Fatal("unsupported userspace per-peer activation must not be emitted")
	}
}

func TestAWGParametersRefuseUnsafeOrWGCompatibleProfiles(t *testing.T) {
	cases := []struct {
		name string
		change func(*AWGParameters)
	}{
		{"empty", func(p *AWGParameters) { *p = AWGParameters{} }},
		{"no-junk", func(p *AWGParameters) { p.Jc = 0 }},
		{"unbounded-junk-count", func(p *AWGParameters) { p.Jc = 17 }},
		{"zero-min", func(p *AWGParameters) { p.Jmin = 0 }},
		{"equal-junk-bounds", func(p *AWGParameters) { p.Jmax = p.Jmin }},
		{"reversed-junk-bounds", func(p *AWGParameters) { p.Jmin = p.Jmax + 1 }},
		{"oversized-junk", func(p *AWGParameters) { p.Jmax = 1281 }},
		{"zero-s1", func(p *AWGParameters) { p.S1 = 0 }},
		{"zero-s2", func(p *AWGParameters) { p.S2 = 0 }},
		{"zero-s3", func(p *AWGParameters) { p.S3 = 0 }},
		{"zero-s4", func(p *AWGParameters) { p.S4 = 0 }},
		{"oversized-s1", func(p *AWGParameters) { p.S1 = 257 }},
		{"oversized-s2", func(p *AWGParameters) { p.S2 = 257 }},
		{"oversized-s3", func(p *AWGParameters) { p.S3 = 257 }},
		{"oversized-s4", func(p *AWGParameters) { p.S4 = 257 }},
		{"wg-h1", func(p *AWGParameters) { p.H1 = 1 }},
		{"wg-h2", func(p *AWGParameters) { p.H2 = 2 }},
		{"wg-h3", func(p *AWGParameters) { p.H3 = 3 }},
		{"wg-h4", func(p *AWGParameters) { p.H4 = 4 }},
		{"missing-header", func(p *AWGParameters) { p.H3 = 0 }},
		{"overlapping-headers", func(p *AWGParameters) { p.H2 = p.H1 }},
	}
	client, err := NewClient("test-client", "Synthetic", netip.MustParseAddr("10.66.0.2"), "synthetic-test-private", "synthetic-test-public")
	if err != nil { t.Fatal(err) }
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := testedAWGParameters()
			tc.change(&p)
			args, err := p.SetArguments()
			if err == nil || len(args) != 0 { t.Fatal("unsafe AWG profile produced server arguments") }
			conf, err := RenderAWGClientConfig(client, "synthetic-server-public", "198.51.100.10:51820", "10.66.0.2/32", "", p)
			if err == nil || conf != "" { t.Fatal("unsafe AWG profile produced a client config or silently fell back to WG") }
		})
	}
}

func TestAWGExportRefusesPresharedKeyUntilProtectedFileContract(t *testing.T) {
	client, err := NewClient("test-client", "Synthetic", netip.MustParseAddr("10.66.0.2"), "synthetic-test-private", "synthetic-test-public")
	if err != nil { t.Fatal(err) }
	conf, err := RenderAWGClientConfig(client, "synthetic-server-public", "198.51.100.10:51820", "10.66.0.2/32", "synthetic-test-psk", testedAWGParameters())
	if err == nil || conf != "" { t.Fatal("AWG M2 export must reject PSK instead of advertising an unsupported server contract") }
	if strings.Contains(err.Error(), "synthetic-test-psk") { t.Fatal("AWG error exposes key material") }
	// Preserve the existing ordinary-WG export behavior exactly.
	wg, err := RenderClientConfig(client, "synthetic-server-public", "198.51.100.10:51820", "10.66.0.2/32", "synthetic-test-psk")
	if err != nil || !strings.Contains(wg, "PresharedKey = synthetic-test-psk\n") { t.Fatal("legacy WG PSK export regressed") }
}

func TestRegistryAWGExportPreservesExistingClientLifecycle(t *testing.T) {
	allocator, err := NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	if err != nil { t.Fatal(err) }
	r := NewRegistry(allocator, func(string) (string, string, error) { return "synthetic-test-private", "synthetic-test-public", nil })
	client, err := r.Create("client-a", "Synthetic")
	if err != nil { t.Fatal(err) }
	wg, err := r.RenderClientConfig(client.ID, "synthetic-server-public", "198.51.100.10:51820")
	if err != nil || strings.Contains(wg, "Jc =") { t.Fatal("ordinary WG export changed before selecting AWG") }
	p := testedAWGParameters()
	if err := r.UseAWGParameters(p); err != nil { t.Fatal(err) }
	if err := r.UseAWGParameters(p); err != nil { t.Fatal("same AWG profile must be idempotent") }
	p.H1 = 9001
	conf, err := r.RenderClientConfig(client.ID, "synthetic-server-public", "198.51.100.10:51820")
	if err != nil || !strings.Contains(conf, "H1 = 1001\n") || strings.Contains(conf, "H1 = 9001") { t.Fatal("registry must copy the validated immutable server profile") }
	if err := r.UseAWGParameters(p); err == nil { t.Fatal("live export profile replacement must be refused") }
	if err := r.Disable(client.ID); err != nil { t.Fatal(err) }
	conf, err = r.RenderClientConfig(client.ID, "synthetic-server-public", "198.51.100.10:51820")
	if err == nil || conf != "" { t.Fatal("disabled AWG client exported") }
	if err := r.Enable(client.ID); err != nil { t.Fatal(err) }
	conf, err = r.RenderClientConfig(client.ID, "synthetic-server-public", "198.51.100.10:51820")
	if err != nil || !strings.Contains(conf, "Jc = 3\n") { t.Fatal("AWG export not restored after enable") }
	if err := r.Revoke(client.ID); err != nil { t.Fatal(err) }
	conf, err = r.RenderClientConfig(client.ID, "synthetic-server-public", "198.51.100.10:51820")
	if err == nil || conf != "" { t.Fatal("revoked AWG client exported") }
}
