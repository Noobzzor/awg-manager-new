package policy

import (
	"testing"

	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
)

type fakeAWG3Catalog struct {
	tags []awg3endpoint.TagInfo
}

func (f fakeAWG3Catalog) ListTags() []awg3endpoint.TagInfo { return f.tags }

func TestCompileOptionsFromAWG3SelectsDeterministicEndpoint(t *testing.T) {
	options, err := CompileOptionsFromAWG3(fakeAWG3Catalog{tags: []awg3endpoint.TagInfo{
		{Tag: "zeta", Kind: "awg3"},
		{Tag: "alpha", Kind: "awg3"},
		{Tag: "ignored", Kind: "other"},
	}}, "")
	if err != nil {
		t.Fatal(err)
	}
	if got := options.DefaultOutbounds[ActionVPN]; got != "alpha" {
		t.Fatalf("VPN outbound = %q, want alpha", got)
	}
	if _, ok := options.Outbounds["alpha"]; !ok {
		t.Fatal("selected AWG3 endpoint is not an outbound")
	}
	if _, ok := options.Outbounds["zeta"]; !ok || options.OutboundActions["zeta"] != ActionVPN {
		t.Fatal("non-default AWG3 endpoint is unavailable to policy rules")
	}
	if _, ok := options.Outbounds["ignored"]; ok {
		t.Fatal("non-AWG3 tag leaked into outbounds")
	}
}

func TestPolicyRuleCanSelectNonDefaultAWG3Endpoint(t *testing.T) {
	options, err := CompileOptionsFromAWG3(fakeAWG3Catalog{tags: []awg3endpoint.TagInfo{
		{Tag: "alpha", Kind: "awg3"},
		{Tag: "zeta", Kind: "awg3"},
	}}, "")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := Compile(Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
		Rules: []Rule{{
			ID:             "zeta-rule",
			Action:         ActionVPN,
			Outbound:       "zeta",
			DomainSuffixes: []string{"example.com"},
			Enabled:        true,
		}},
	}, options)
	if err != nil {
		t.Fatalf("compile non-default VPN rule: %v", err)
	}
	if len(compiled.Rules) != 1 || compiled.Rules[0].Outbound != "zeta" {
		t.Fatalf("compiled rule = %#v, want zeta outbound", compiled.Rules)
	}
}

func TestCompileOptionsFromAWG3HonoursSelection(t *testing.T) {
	options, err := CompileOptionsFromAWG3(fakeAWG3Catalog{tags: []awg3endpoint.TagInfo{
		{Tag: "alpha", Kind: "awg3"},
		{Tag: "beta", Kind: "awg3"},
	}}, "beta")
	if err != nil {
		t.Fatal(err)
	}
	if got := options.DefaultOutbounds[ActionVPN]; got != "beta" {
		t.Fatalf("VPN outbound = %q, want beta", got)
	}
}

func TestCompileOptionsFromAWG3RejectsMissingSelection(t *testing.T) {
	_, err := CompileOptionsFromAWG3(fakeAWG3Catalog{tags: []awg3endpoint.TagInfo{{Tag: "alpha", Kind: "awg3"}}}, "missing")
	if err == nil {
		t.Fatal("expected missing selection error")
	}
}

func TestCompileOptionsFromAWG3RejectsReservedDirectTag(t *testing.T) {
	_, err := CompileOptionsFromAWG3(fakeAWG3Catalog{tags: []awg3endpoint.TagInfo{{Tag: "direct", Kind: "awg3"}}}, "")
	if err == nil {
		t.Fatal("expected reserved direct tag to be rejected")
	}
}

func TestCompileOptionsFromAWG3DoesNotInventVPNWithoutEndpoint(t *testing.T) {
	options, err := CompileOptionsFromAWG3(fakeAWG3Catalog{}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := options.DefaultOutbounds[ActionVPN]; ok {
		t.Fatal("VPN fallback was invented without an AWG3 endpoint")
	}
}
