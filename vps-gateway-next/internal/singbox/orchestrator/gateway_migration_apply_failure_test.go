package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type migrationFailingReload struct {
	*fakeProc
	err error
}

func (p *migrationFailingReload) Reload() error {
	_ = p.fakeProc.Reload()
	return p.err
}

func TestGatewayInvalidationFailedApplyRemainsDirtyAcrossFreshRetry(t *testing.T) {
	dir := t.TempDir()
	applied := filepath.Join(t.TempDir(), "applied.json")
	for name, body := range map[string]string{
		"20-router.json":                    `{"route":{"rules":[]}}`,
		legacyGatewayPolicyFilename:         `{"route":{"rules":[{"port":18082,"action":"reject"}]}}`,
		"disabled/" + gatewayPolicyFilename: `{"route":{"rules":[]}}`,
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	fp := &migrationFailingReload{fakeProc: &fakeProc{running: true}}
	makeOrch := func(gateway bool) *Orchestrator {
		o := NewWithAppliedPath(dir, fp, applied)
		t.Cleanup(o.Close)
		if err := o.Register(SlotMeta{Slot: SlotRouter, Filename: "20-router.json"}); err != nil {
			t.Fatal(err)
		}
		if gateway {
			if err := o.Register(SlotMeta{Slot: SlotGatewayPolicy, Filename: gatewayPolicyFilename}); err != nil {
				t.Fatal(err)
			}
		}
		if err := o.Bootstrap(); err != nil {
			t.Fatal(err)
		}
		return o
	}
	before := makeOrch(false)
	if err := before.Reload(); err != nil {
		t.Fatal(err)
	}
	before.Close()
	after := makeOrch(true)
	want := errors.New("controlled apply failure")
	fp.err = want
	if err := after.Reload(); !errors.Is(err, want) {
		t.Fatalf("lost apply error: %v", err)
	}
	after.Close()
	st, ok := loadAppliedState(applied)
	if !ok || st.Hash != "" {
		t.Fatalf("failed apply restored trusted history: %+v", st)
	}
	fp.err = nil
	retry := makeOrch(true)
	n := fp.reloadsN()
	if err := retry.Reload(); err != nil {
		t.Fatal(err)
	}
	if fp.reloadsN() != n+1 {
		t.Fatal("fresh retry skipped required apply")
	}
	retry.Close()
	adopt := makeOrch(true)
	if err := adopt.Reload(); err != nil {
		t.Fatal(err)
	}
	if fp.reloadsN() != n+1 {
		t.Fatal("unchanged successful restart should adopt")
	}
}

func TestGatewayInactiveOnlyMigrationPreservesAppliedCredential(t *testing.T) {
	for _, location := range []string{"disabled", "pending"} {
		t.Run(location, func(t *testing.T) {
			dir := t.TempDir()
			applied := filepath.Join(t.TempDir(), "applied.json")
			if err := os.MkdirAll(filepath.Join(dir, location), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "20-router.json"), []byte(`{}`), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, location, legacyGatewayPolicyFilename), []byte(`{}`), 0600); err != nil {
				t.Fatal(err)
			}
			fp := &fakeProc{running: true}
			before := NewWithAppliedPath(dir, fp, applied)
			t.Cleanup(before.Close)
			if err := before.Register(SlotMeta{Slot: SlotRouter, Filename: "20-router.json"}); err != nil {
				t.Fatal(err)
			}
			if err := before.Bootstrap(); err != nil {
				t.Fatal(err)
			}
			if err := before.Reload(); err != nil {
				t.Fatal(err)
			}
			before.Close()
			original, err := os.ReadFile(applied)
			if err != nil {
				t.Fatal(err)
			}
			after := NewWithAppliedPath(dir, fp, applied)
			t.Cleanup(after.Close)
			for _, m := range []SlotMeta{{Slot: SlotRouter, Filename: "20-router.json"}, {Slot: SlotGatewayPolicy, Filename: gatewayPolicyFilename}} {
				if err := after.Register(m); err != nil {
					t.Fatal(err)
				}
			}
			if err := after.Bootstrap(); err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(applied)
			if err != nil || string(got) != string(original) {
				t.Fatalf("inactive-only migration invalidated apply: %v", err)
			}
			if err := after.Reload(); err != nil {
				t.Fatal(err)
			}
			if fp.reloadsN() != 1 {
				t.Fatalf("inactive-only migration should not reload: %v", fp.calls())
			}
		})
	}
}
