package dnsroute

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

func TestCompileSingboxGolden(t *testing.T) {
	lists := []DomainList{
		{
			ID:             "Video List",
			Backend:        BackendSingbox,
			Enabled:        true,
			Domains:        []string{" Example.COM. ", ".Sub.Example.org"},
			Subnets:        []string{"192.0.2.1/24", "2001:db8::1/32"},
			Excludes:       []string{" DIRECT.Example.com. "},
			ExcludeSubnets: []string{"192.0.2.128/25"},
			Routes: []RouteTarget{{
				TunnelID:  "opaque-awg-id",
				Interface: "opaque-interface",
				Fallback:  "auto",
			}},
		},
		{
			ID:      "disabled",
			Backend: BackendSingbox,
			Enabled: false,
			Domains: []string{"must-not-appear.example"},
		},
		{
			ID:      "legacy",
			Backend: "ndms",
			Enabled: true,
			Domains: []string{"must-not-appear.example"},
		},
	}

	resolveCalls := 0
	got, err := CompileSingbox(lists, SingboxCompileOptions{
		DNSUpstream: "1.1.1.1",
		ResolveTarget: func(target RouteTarget) (string, error) {
			resolveCalls++
			if target.TunnelID != "opaque-awg-id" || target.Interface != "opaque-interface" {
				t.Fatalf("compiler changed opaque target: %+v", target)
			}
			return "awg-primary", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resolveCalls != 1 {
		t.Fatalf("ResolveTarget calls = %d, want 1", resolveCalls)
	}
	want := []byte(`{
  "dns": {
    "servers": [
      {
        "tag": "dnsroute-video-list-dns",
        "type": "udp",
        "server": "1.1.1.1",
        "detour": "dnsroute-video-list-outbound"
      }
    ],
    "rules": [
      {
        "type": "logical",
        "mode": "and",
        "rules": [
          {
            "domain_suffix": [
              "example.com",
              "sub.example.org"
            ]
          },
          {
            "invert": true,
            "domain_suffix": [
              "direct.example.com"
            ]
          }
        ],
        "server": "dnsroute-video-list-dns"
      }
    ]
  },
  "outbounds": [
    {
      "type": "urltest",
      "tag": "dnsroute-video-list-outbound",
      "outbounds": [
        "awg-primary",
        "direct"
      ],
      "url": "https://www.gstatic.com/generate_204",
      "interval": "30s",
      "tolerance": 30000
    }
  ],
  "route": {
    "rules": [
      {
        "type": "logical",
        "mode": "and",
        "rules": [
          {
            "domain_suffix": [
              "example.com",
              "sub.example.org"
            ],
            "ip_cidr": [
              "192.0.2.0/24",
              "2001:db8::/32"
            ]
          },
          {
            "invert": true,
            "domain_suffix": [
              "direct.example.com"
            ],
            "ip_cidr": [
              "192.0.2.128/25"
            ]
          }
        ],
        "action": "route",
        "outbound": "dnsroute-video-list-outbound"
      }
    ]
  }
}
`)
	if !bytes.Equal(got, want) {
		t.Fatalf("fragment mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	again, err := CompileSingbox(lists, SingboxCompileOptions{
		DNSUpstream:   "1.1.1.1",
		ResolveTarget: func(RouteTarget) (string, error) { return "awg-primary", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, again) {
		t.Fatal("compiler output is not stable")
	}
}

func TestCompileSingboxDirectAndReject(t *testing.T) {
	lists := []DomainList{
		{ID: "direct", Backend: BackendSingbox, Enabled: true, Domains: []string{"direct.example"}, Routes: []RouteTarget{{TunnelID: "d"}}},
		{ID: "reject", Backend: BackendSingbox, Enabled: true, Domains: []string{"reject.example"}, Routes: []RouteTarget{{TunnelID: "r"}}},
	}
	got, err := CompileSingbox(lists, SingboxCompileOptions{
		DNSUpstream: "9.9.9.9",
		ResolveTarget: func(target RouteTarget) (string, error) {
			if target.TunnelID == "d" {
				return "direct", nil
			}
			return "block", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, part := range []string{`"action": "reject"`, `"outbound": "direct"`, `"tag": "dnsroute-direct-direct-dns"`, `"server": "dnsroute-direct-direct-dns"`} {
		if !strings.Contains(s, part) {
			t.Errorf("missing %s in:\n%s", part, s)
		}
	}
	if strings.Contains(s, `"detour": "direct"`) {
		t.Errorf("DIRECT DNS server must omit detour to the implicit outbound:\n%s", s)
	}
	if directAt, rejectAt := strings.Index(s, `"domain_suffix": [
          "direct.example"`), strings.Index(s, `"domain_suffix": [
          "reject.example"`); directAt < 0 || rejectAt < 0 || directAt >= rejectAt {
		t.Fatalf("enabled list order not preserved: direct=%d reject=%d\n%s", directAt, rejectAt, s)
	}
	if strings.Contains(s, `"detour": "reject"`) {
		t.Fatalf("reject target must not create a DNS server:\n%s", s)
	}
}

func TestCompileSingboxSingleTargetUsesResolvedTag(t *testing.T) {
	got, err := CompileSingbox([]DomainList{{
		ID: "single", Backend: BackendSingbox, Enabled: true,
		Domains: []string{"single.example"}, Routes: []RouteTarget{{TunnelID: "opaque"}},
	}}, SingboxCompileOptions{
		DNSUpstream:   "1.1.1.1",
		ResolveTarget: func(RouteTarget) (string, error) { return "awg-only", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if strings.Contains(s, `"type": "urltest"`) {
		t.Fatalf("one target must not create a group:\n%s", s)
	}
	if !strings.Contains(s, `"outbound": "awg-only"`) || !strings.Contains(s, `"detour": "awg-only"`) {
		t.Fatalf("resolved tag not used for route and DNS:\n%s", s)
	}
}

func TestCompileSingboxOrderedTargetsAndTerminalFallback(t *testing.T) {
	tests := []struct {
		name         string
		fallback     string
		wantTerminal string
		noTerminal   string
	}{
		{name: "auto adds direct terminal", fallback: "auto", wantTerminal: "direct"},
		{name: "reject remains kill switch", fallback: "reject", noTerminal: "direct"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompileSingbox([]DomainList{{
				ID: "ordered", Backend: BackendSingbox, Enabled: true,
				Domains: []string{"ordered.example"},
				Routes:  []RouteTarget{{TunnelID: "first"}, {TunnelID: "second", Fallback: tt.fallback}},
			}}, SingboxCompileOptions{
				DNSUpstream:   "1.1.1.1",
				ResolveTarget: func(target RouteTarget) (string, error) { return "awg-" + target.TunnelID, nil },
			})
			if err != nil {
				t.Fatal(err)
			}
			s := string(got)
			ordered := `"outbounds": [
        "awg-first",
        "awg-second"`
			if !strings.Contains(s, ordered) {
				t.Fatalf("target order not preserved:\n%s", s)
			}
			if tt.wantTerminal != "" && !strings.Contains(s, `"awg-second",
        "`+tt.wantTerminal+`"`) {
				t.Fatalf("missing terminal %q:\n%s", tt.wantTerminal, s)
			}
			if tt.noTerminal != "" && strings.Contains(s, `"awg-second",
        "`+tt.noTerminal+`"`) {
				t.Fatalf("unexpected terminal %q:\n%s", tt.noTerminal, s)
			}
			if tt.fallback == "auto" && !strings.Contains(s, `"detour": "dnsroute-ordered-outbound"`) {
				t.Fatalf("DNS server must detour to the managed group with direct fallback:\n%s", s)
			}
		})
	}
}

func TestCompileSingboxErrors(t *testing.T) {
	t.Run("missing resolver", func(t *testing.T) {
		_, err := CompileSingbox([]DomainList{{ID: "x", Backend: BackendSingbox, Enabled: true, Domains: []string{"x.example"}, Routes: []RouteTarget{{TunnelID: "x"}}}}, SingboxCompileOptions{DNSUpstream: "1.1.1.1"})
		if err == nil || !strings.Contains(err.Error(), "resolver") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("missing target", func(t *testing.T) {
		_, err := CompileSingbox([]DomainList{{ID: "x", Backend: BackendSingbox, Enabled: true, Domains: []string{"x.example"}, Routes: []RouteTarget{{TunnelID: "missing"}}}}, SingboxCompileOptions{
			DNSUpstream:   "1.1.1.1",
			ResolveTarget: func(RouteTarget) (string, error) { return "", errors.New("not found") },
		})
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("managed tag collision", func(t *testing.T) {
		_, err := CompileSingbox([]DomainList{
			{ID: "same id", Backend: BackendSingbox, Enabled: true, Domains: []string{"a.example"}, Routes: []RouteTarget{{TunnelID: "a"}}},
			{ID: "same-id", Backend: BackendSingbox, Enabled: true, Domains: []string{"b.example"}, Routes: []RouteTarget{{TunnelID: "b"}}},
		}, SingboxCompileOptions{DNSUpstream: "1.1.1.1", ResolveTarget: func(target RouteTarget) (string, error) { return "awg-" + target.TunnelID, nil }})
		if err == nil || !strings.Contains(err.Error(), "collision") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("resolved target collides with managed tag", func(t *testing.T) {
		_, err := CompileSingbox([]DomainList{{ID: "x", Backend: BackendSingbox, Enabled: true, Domains: []string{"x.example"}, Routes: []RouteTarget{{TunnelID: "x"}, {TunnelID: "y"}}}}, SingboxCompileOptions{
			DNSUpstream:   "1.1.1.1",
			ResolveTarget: func(RouteTarget) (string, error) { return "dnsroute-x-outbound", nil },
		})
		if err == nil || !strings.Contains(err.Error(), "collision") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("invalid cidr", func(t *testing.T) {
		_, err := CompileSingbox([]DomainList{{ID: "x", Backend: BackendSingbox, Enabled: true, Subnets: []string{"not-a-cidr"}, Routes: []RouteTarget{{TunnelID: "x"}}}}, SingboxCompileOptions{
			DNSUpstream: "1.1.1.1", ResolveTarget: func(RouteTarget) (string, error) { return "awg-x", nil },
		})
		if err == nil || !strings.Contains(err.Error(), "CIDR") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestCompileSingboxDisabledProducesEmptyFragmentWithoutDependencies(t *testing.T) {
	got, err := CompileSingbox([]DomainList{{ID: "off", Backend: BackendSingbox, Domains: []string{"off.example"}}}, SingboxCompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "{}\n" {
		t.Fatalf("got %q", got)
	}
}

func TestCompileSingboxExclusionsAreScopedToTheirOwnInclude(t *testing.T) {
	got, err := CompileSingbox([]DomainList{
		{ID: "scoped", Backend: BackendSingbox, Enabled: true, Domains: []string{"example.com"}, Subnets: []string{"192.0.2.0/24"}, Excludes: []string{"private.example.com"}, ExcludeSubnets: []string{"192.0.2.7"}, Routes: []RouteTarget{{TunnelID: "vpn"}}},
		{ID: "later", Backend: BackendSingbox, Enabled: true, Domains: []string{"private.example.com"}, Subnets: []string{"192.0.2.7"}, Routes: []RouteTarget{{TunnelID: "vpn"}}},
	}, SingboxCompileOptions{DNSUpstream: "1.1.1.1", ResolveTarget: func(RouteTarget) (string, error) { return "vpn", nil }})
	if err != nil {
		t.Fatal(err)
	}
	var fragment struct {
		DNS struct {
			Rules []router.DNSRule `json:"rules"`
		} `json:"dns"`
		Route struct {
			Rules []router.Rule `json:"rules"`
		} `json:"route"`
	}
	if err := json.Unmarshal(got, &fragment); err != nil {
		t.Fatal(err)
	}
	if len(fragment.DNS.Rules) != 2 || fragment.DNS.Rules[0].Type != "logical" || fragment.DNS.Rules[0].Mode != "and" || len(fragment.DNS.Rules[0].Rules) != 2 || !fragment.DNS.Rules[0].Rules[1].Invert {
		t.Fatalf("DNS exclusion is not scoped: %+v", fragment.DNS.Rules)
	}
	if len(fragment.Route.Rules) != 2 || fragment.Route.Rules[0].Type != "logical" || fragment.Route.Rules[0].Mode != "and" || len(fragment.Route.Rules[0].Rules) != 2 || !fragment.Route.Rules[0].Rules[1].Invert {
		t.Fatalf("route exclusion is not scoped: %+v", fragment.Route.Rules)
	}
	if strings.Contains(string(got), `"outbound": "direct"`) || strings.Contains(string(got), `"detour": "direct"`) {
		t.Fatalf("scoped exclusions created a global direct leak:\n%s", got)
	}
	if cidrs := fragment.Route.Rules[0].Rules[1].IPCIDR; len(cidrs) != 1 || cidrs[0] != "192.0.2.7/32" {
		t.Fatalf("excluded IP = %v, want host CIDR", cidrs)
	}
}

func TestCompileSingboxBareIPsBecomeHostCIDRs(t *testing.T) {
	got, err := CompileSingbox([]DomainList{{ID: "ips", Backend: BackendSingbox, Enabled: true, Subnets: []string{"192.0.2.7", "2001:db8::7"}, Routes: []RouteTarget{{TunnelID: "vpn"}}}}, SingboxCompileOptions{ResolveTarget: func(RouteTarget) (string, error) { return "vpn", nil }})
	if err != nil {
		t.Fatal(err)
	}
	for _, cidr := range []string{"192.0.2.7/32", "2001:db8::7/128"} {
		if !strings.Contains(string(got), cidr) {
			t.Fatalf("missing host CIDR %q:\n%s", cidr, got)
		}
	}
}

func TestCompileSingboxCompatibilityErrors(t *testing.T) {
	for _, tt := range []struct {
		name   string
		list   DomainList
		marker string
	}{
		{name: "geosite", list: DomainList{Domains: []string{"geosite:youtube"}}, marker: "geosite:"},
		{name: "geoip", list: DomainList{Subnets: []string{"geoip:private"}}, marker: "geoip:"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.list.ID, tt.list.Backend, tt.list.Enabled, tt.list.Routes = tt.name, BackendSingbox, true, []RouteTarget{{TunnelID: "vpn"}}
			_, err := CompileSingbox([]DomainList{tt.list}, SingboxCompileOptions{ResolveTarget: func(RouteTarget) (string, error) { return "vpn", nil }})
			if err == nil || !strings.Contains(err.Error(), tt.marker) || !strings.Contains(err.Error(), "not supported") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestCompileSingboxTerminalTargetRejectsTrailingTargets(t *testing.T) {
	for _, terminal := range []string{"direct", "reject", "block"} {
		t.Run(terminal, func(t *testing.T) {
			_, err := CompileSingbox([]DomainList{{ID: "terminal", Backend: BackendSingbox, Enabled: true, Domains: []string{"x.example"}, Routes: []RouteTarget{{TunnelID: terminal}, {TunnelID: "later"}}}}, SingboxCompileOptions{DNSUpstream: "1.1.1.1", ResolveTarget: func(target RouteTarget) (string, error) { return target.TunnelID, nil }})
			if err == nil || !strings.Contains(err.Error(), "trailing targets") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestCompileSingboxReservedTagCollisions(t *testing.T) {
	base := []DomainList{{ID: "x", Backend: BackendSingbox, Enabled: true, Domains: []string{"x.example"}, Routes: []RouteTarget{{TunnelID: "vpn"}, {TunnelID: "backup"}}}}
	for _, tt := range []struct {
		name string
		opts SingboxCompileOptions
	}{
		{name: "managed outbound", opts: SingboxCompileOptions{ReservedOutboundTags: map[string]struct{}{"dnsroute-x-outbound": {}}}},
		{name: "managed DNS", opts: SingboxCompileOptions{ReservedDNSTags: map[string]struct{}{"dnsroute-x-dns": {}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.DNSUpstream = "1.1.1.1"
			tt.opts.ResolveTarget = func(target RouteTarget) (string, error) { return target.TunnelID, nil }
			_, err := CompileSingbox(base, tt.opts)
			if err == nil || !strings.Contains(err.Error(), "collision") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestCompileSingboxTrimsAndValidatesUDPUpstream(t *testing.T) {
	list := []DomainList{{ID: "x", Backend: BackendSingbox, Enabled: true, Domains: []string{"x.example"}, Routes: []RouteTarget{{TunnelID: "vpn"}}}}
	got, err := CompileSingbox(list, SingboxCompileOptions{DNSUpstream: " 1.1.1.1 ", ResolveTarget: func(RouteTarget) (string, error) { return "vpn", nil }})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), " 1.1.1.1 ") || !strings.Contains(string(got), `"server": "1.1.1.1"`) {
		t.Fatalf("upstream was not trimmed:\n%s", got)
	}
	for _, upstream := range []string{"https://dns.example/dns-query", "1.1.1.1:53", "bad host"} {
		_, err := CompileSingbox(list, SingboxCompileOptions{DNSUpstream: upstream, ResolveTarget: func(RouteTarget) (string, error) { return "vpn", nil }})
		if err == nil || !strings.Contains(err.Error(), "invalid UDP DNS upstream") {
			t.Fatalf("upstream %q: err = %v", upstream, err)
		}
	}
}
