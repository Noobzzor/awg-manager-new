package orchestrator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestGatewayMigrationInvalidationErrorDoesNotMutateActiveFiles(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, legacyGatewayPolicyFilename)
	body := []byte(`{"route":{"rules":[{"port":18082,"action":"reject"}]}}`)
	if err := os.WriteFile(legacy, body, 0600); err != nil {
		t.Fatal(err)
	}
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	fp := &fakeProc{running: true}
	o := NewWithAppliedPath(dir, fp, filepath.Join(blocker, "applied.json"))
	t.Cleanup(o.Close)
	if err := o.Register(SlotMeta{Slot: SlotGatewayPolicy, Filename: gatewayPolicyFilename}); err != nil {
		t.Fatal(err)
	}
	err := o.Bootstrap()
	if err == nil || !strings.Contains(err.Error(), "invalidate applied state") || !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("want propagated invalidation ENOTDIR, got %v", err)
	}
	got, err := os.ReadFile(legacy)
	if err != nil || string(got) != string(body) {
		t.Fatalf("active legacy changed after invalidation failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, gatewayPolicyFilename)); !os.IsNotExist(err) {
		t.Fatalf("current policy unexpectedly created: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".legacy-gateway-policy")); !os.IsNotExist(err) {
		t.Fatalf("archive unexpectedly created: %v", err)
	}
	if len(fp.calls()) != 0 {
		t.Fatalf("migration must not touch process: %v", fp.calls())
	}
}

func TestGatewayMigrationPartialRetryKeepsInvalidationAndTunShape(t *testing.T) {
	for _, tun := range []bool{false, true} {
		name := "no-tun"
		if tun {
			name = "tun"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			applied := filepath.Join(t.TempDir(), "applied.json")
			write := func(name, body string) {
				t.Helper()
				p := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			router := `{"route":{"rules":[]}}`
			if tun {
				router = tunInboundConfig
			}
			write("20-router.json", router)
			write(legacyGatewayPolicyFilename, `{"route":{"rules":[{"port":18082,"action":"reject"}]}}`)
			write("pending/"+legacyGatewayPolicyFilename, `{"route":{"rules":[]}}`)
			write("pending/"+gatewayPolicyFilename, `{"route":{"rules":[]}}`)
			// Active rename succeeds; only the later pending conflict needs
			// this archive directory. A regular blocker fails deterministically.
			write(".legacy-gateway-policy", "archive blocker")
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
			st, ok := loadAppliedState(applied)
			if !ok || st.Hash == "" || st.HasTun != tun {
				t.Fatalf("bad production-applied baseline: %+v", st)
			}
			fresh := func() *Orchestrator {
				o := NewWithAppliedPath(dir, fp, applied)
				t.Cleanup(o.Close)
				for _, m := range []SlotMeta{{Slot: SlotRouter, Filename: "20-router.json"}, {Slot: SlotGatewayPolicy, Filename: gatewayPolicyFilename}} {
					if err := o.Register(m); err != nil {
						t.Fatal(err)
					}
				}
				return o
			}
			first := fresh()
			if err := first.Bootstrap(); err == nil || !strings.Contains(err.Error(), "archive") {
				t.Fatalf("expected later archive failure, got %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, legacyGatewayPolicyFilename)); !os.IsNotExist(err) {
				t.Fatalf("active rename did not precede failure: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, gatewayPolicyFilename)); err != nil {
				t.Fatalf("renamed active missing: %v", err)
			}
			st, ok = loadAppliedState(applied)
			if !ok || st.Hash != "" || st.HasTun != tun {
				t.Fatalf("partial failure lost persisted invalidation/shape: %+v", st)
			}
			first.Close()
			if err := os.Remove(filepath.Join(dir, ".legacy-gateway-policy")); err != nil {
				t.Fatal(err)
			}
			retry := fresh()
			if retry.CurrentHasTun() != tun {
				t.Fatal("fresh retry did not retain previous process shape")
			}
			if err := retry.Bootstrap(); err != nil {
				t.Fatal(err)
			}
			n := fp.reloadsN()
			starts, stops := fp.startsN(), fp.stopsN()
			if err := retry.Reload(); err != nil {
				t.Fatal(err)
			}
			if fp.reloadsN() != n+1 || fp.startsN() != starts || fp.stopsN() != stops {
				t.Fatalf("retry did not apply with preserved shape: %v", fp.calls())
			}
			if err := retry.Reload(); err != nil {
				t.Fatal(err)
			}
			if fp.reloadsN() != n+1 {
				t.Fatal("unchanged successful retry should adopt")
			}
		})
	}
}

func TestAppliedInvalidationSafeMissingCorruptAndAlreadyDirty(t *testing.T) {
	for _, body := range []string{"missing", "{broken", `{"Hash":"","HasTun":true}`} {
		t.Run(body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "applied.json")
			if body != "missing" {
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := invalidateAppliedState(path); err != nil {
				t.Fatal(err)
			}
			if body != "missing" {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != body {
					t.Fatalf("invalid history should stay untouched: %v", err)
				}
			}
		})
	}
}
