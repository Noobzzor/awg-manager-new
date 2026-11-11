package router

import (
	"bytes"
	"os"
	"testing"
)

// This control uses the actual compiler and matcher without network access.
// It does not prove the product's AWG packet or apply path.
func TestNativePortableInlineVersions(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_INLINE_PROOF") != "1" {
		t.Skip("requires explicit native inline control")
	}
	binary := os.Getenv("SINGBOX_BIN")
	if binary == "" {
		t.Fatal("SINGBOX_BIN is required")
	}
	m := ruleSetMaterializer{configDir: t.TempDir(), binary: binary, immutableInline: true}
	materialize := func(domain string) RuleSet {
		t.Helper()
		rs, err := m.materializeRuleSet(RuleSet{Tag: "personal", Type: "inline", Rules: []map[string]any{{"domain_suffix": []string{domain}}}})
		if err != nil {
			t.Fatal(err)
		}
		return rs
	}
	old := materialize("before.example")
	oldBytes, err := os.ReadFile(old.Path)
	if err != nil {
		t.Fatal(err)
	}
	next := materialize("after.example")
	if old.Path == next.Path {
		t.Fatal("draft replaced the applied artifact")
	}
	stillOld, err := os.ReadFile(old.Path)
	if err != nil || !bytes.Equal(oldBytes, stillOld) {
		t.Fatal("applied artifact changed")
	}
	if materialize("before.example").Path != old.Path {
		t.Fatal("same content did not reuse artifact")
	}
	for _, tc := range []struct {
		rs     RuleSet
		domain string
		want   bool
	}{
		{old, "sub.before.example", true}, {old, "after.example", false},
		{next, "sub.after.example", true}, {next, "before.example", false},
	} {
		matched, supported, err := matchRuleSet(tc.domain, tc.rs, binary, nil, nil)
		if err != nil || !supported || matched != tc.want {
			t.Fatalf("native match %s: matched=%v supported=%v err=%v", tc.domain, matched, supported, err)
		}
	}
	t.Log("native inline compile and match controls passed")
}
