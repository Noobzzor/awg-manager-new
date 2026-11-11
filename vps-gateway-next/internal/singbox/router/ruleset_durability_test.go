package router

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func durabilityFixture(t *testing.T) (ruleSetMaterializer, RuleSet, *int) {
	t.Helper()
	calls := new(int)
	withFakeRuleSetCompiler(t, func(_ string, args []string) (string, string, error) {
		*calls++
		writeCompiledOutput(t, args, "owned compiled fixture")
		return "", "", nil
	})
	m := ruleSetMaterializer{configDir: t.TempDir(), binary: "fixture-compiler", immutableInline: true}
	rs := RuleSet{Tag: "durable", Type: "inline", Rules: []map[string]any{{"domain_suffix": []string{"before.example"}}}}
	return m, rs, calls
}

func TestPortableArtifactDurabilityRejectsSyncFailure(t *testing.T) {
	for _, stage := range []string{"source", "compiled", "directory", "rules-parent", "config-root"} {
		t.Run(stage, func(t *testing.T) {
			m, rs, _ := durabilityFixture(t)
			old, err := m.materializeRuleSet(rs)
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(old.Path)
			if err != nil {
				t.Fatal(err)
			}
			reached := false
			m.syncPath = func(path string) error {
				fail := stage == "source" && strings.HasSuffix(path, ".json.tmp") || stage == "compiled" && strings.HasSuffix(path, ".srs.tmp") || stage == "directory" && path == filepath.Join(m.configDir, "rule-sets", "inline") || stage == "rules-parent" && path == filepath.Join(m.configDir, "rule-sets") || stage == "config-root" && path == m.configDir
				if fail {
					reached = true
					return errors.New("injected artifact sync failure")
				}
				return nil
			}
			rs.Rules = []map[string]any{{"domain_suffix": []string{"after.example"}}}
			got, err := m.materializeRuleSet(rs)
			if err == nil || !reached || got.Path != "" {
				t.Fatalf("accepted unsynced artifact: err=%v reached=%t accepted=%t", err, reached, got.Path != "")
			}
			after, err := os.ReadFile(old.Path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("sync failure changed the previous immutable artifact")
			}
		})
	}
}

func TestPortableArtifactDurabilityChecksCacheHit(t *testing.T) {
	m, rs, calls := durabilityFixture(t)
	if _, err := m.materializeRuleSet(rs); err != nil {
		t.Fatal(err)
	}
	reached := false
	m.syncPath = func(string) error { reached = true; return errors.New("injected cache sync failure") }
	got, err := m.materializeRuleSet(rs)
	if err == nil || !reached || got.Path != "" || *calls != 1 {
		t.Fatal("cached artifact bypassed durability failure or was recompiled")
	}
}

func TestPortableArtifactDurabilityPreservesLegacyMode(t *testing.T) {
	m, rs, _ := durabilityFixture(t)
	m.immutableInline = false
	called := false
	m.syncPath = func(string) error { called = true; return errors.New("must not affect legacy mode") }
	if _, err := m.materializeRuleSet(rs); err != nil || called {
		t.Fatal("legacy materialization acquired portable durability behavior")
	}
}

func TestPortableArtifactDurabilityRetryAfterPublicationSyncFailure(t *testing.T) {
	m, rs, calls := durabilityFixture(t)
	m.syncPath = func(path string) error {
		if path == filepath.Join(m.configDir, "rule-sets", "inline") {
			return errors.New("injected publication sync failure")
		}
		return nil
	}
	if got, err := m.materializeRuleSet(rs); err == nil || got.Path != "" {
		t.Fatal("failed publication was accepted")
	}
	m.syncPath = nil
	got, err := m.materializeRuleSet(rs)
	if err != nil || got.Path == "" || *calls != 1 {
		t.Fatal("retry did not safely reuse and sync published immutable files")
	}
}
