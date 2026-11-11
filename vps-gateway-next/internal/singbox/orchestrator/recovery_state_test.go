package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func recoveryStateTree(t *testing.T, root string) []byte {
	t.Helper()
	data := map[string]string{}
	err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		info, err := e.Info()
		if err != nil {
			return err
		}
		data[rel] = info.Mode().String()
		if !e.IsDir() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			data[rel] += "\n" + string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func recoveryStateFixture(t *testing.T, dir string) *Orchestrator {
	t.Helper()
	o := NewWithAppliedPath(dir, nil, filepath.Join(t.TempDir(), "applied.json"))
	t.Cleanup(o.Close)
	for _, m := range KnownSlots() {
		if err := o.Register(m); err != nil {
			t.Fatal(err)
		}
	}
	return o
}
func recoveryStateWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryStateReadsActualFilesWithoutCleanup(t *testing.T) {
	dir := t.TempDir()
	o := recoveryStateFixture(t, dir)
	recoveryStateWrite(t, filepath.Join(dir, "00-base.json"), `{}`)
	recoveryStateWrite(t, filepath.Join(dir, "19-dns-routes.json"), `{}`)
	recoveryStateWrite(t, filepath.Join(dir, "20-router.json"), `{"active":"protected"}`)
	recoveryStateWrite(t, filepath.Join(dir, "disabled/20-router.json"), `{"rollback":"protected"}`)
	recoveryStateWrite(t, filepath.Join(dir, "disabled/40-subscriptions.json"), `{}`)
	recoveryStateWrite(t, filepath.Join(dir, ".batch-check-protected/proof"), "keep")
	before := recoveryStateTree(t, dir)
	if err := o.LoadRecoveryState(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, recoveryStateTree(t, dir)) {
		t.Fatal("startup discovery normalized or deleted recovery resources")
	}
	if !o.enabled[SlotBase] || !o.enabled[SlotDNSRoutes] || !o.enabled[SlotRouter] || o.enabled[SlotSubscriptions] || o.enabled[SlotAwg3] {
		t.Fatal("recovery discovery does not reflect actual active files")
	}
	if !o.RuntimeRecoveryRequired() {
		t.Fatal("read-only discovery did not retain the recovery barrier")
	}
}

func TestRecoveryStateMissingTreeIsNotCreated(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "absent")
	o := recoveryStateFixture(t, dir)
	if err := o.LoadRecoveryState(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("discovery created runtime directories")
	}
	if !o.RuntimeRecoveryRequired() {
		t.Fatal("missing state cleared recovery barrier")
	}
}

func TestRecoveryStateDoesNotPromoteDisabledAlwaysOn(t *testing.T) {
	dir := t.TempDir()
	o := recoveryStateFixture(t, dir)
	recoveryStateWrite(t, filepath.Join(dir, "disabled/00-base.json"), `{}`)
	before := recoveryStateTree(t, dir)
	if err := o.LoadRecoveryState(context.Background()); err != nil {
		t.Fatal(err)
	}
	if o.enabled[SlotBase] || !bytes.Equal(before, recoveryStateTree(t, dir)) {
		t.Fatal("discovery promoted an inactive AlwaysOn slot before recovery")
	}
}

func TestRecoveryStateRejectsUnsafeInputAtomically(t *testing.T) {
	for _, kind := range []string{"active-link", "disabled-link", "unknown-json"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			o := recoveryStateFixture(t, dir)
			o.enabled[SlotRouter] = true
			if kind == "unknown-json" {
				recoveryStateWrite(t, filepath.Join(dir, "77-foreign.json"), `{}`)
			} else {
				target := filepath.Join(t.TempDir(), "proof")
				recoveryStateWrite(t, target, `{}`)
				path := filepath.Join(dir, "19-dns-routes.json")
				if kind == "disabled-link" {
					path = filepath.Join(dir, "disabled/19-dns-routes.json")
					if err := os.Mkdir(filepath.Dir(path), 0700); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			}
			if err := o.LoadRecoveryState(context.Background()); err == nil {
				t.Fatal("unsafe recovery input accepted")
			}
			if !o.enabled[SlotRouter] || !o.RuntimeRecoveryRequired() {
				t.Fatal("failed discovery changed enabled map or lost recovery barrier")
			}
		})
	}
}

func TestRecoveryStateCancellationKeepsBarrier(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "absent")
	o := recoveryStateFixture(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(o.LoadRecoveryState(ctx), context.Canceled) {
		t.Fatal("cancelled discovery accepted")
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("cancelled discovery wrote state")
	}
	if !o.RuntimeRecoveryRequired() {
		t.Fatal("cancelled discovery lost barrier")
	}
}

func TestRecoveryStateLegacyBootstrapStillPromotes(t *testing.T) {
	dir := t.TempDir()
	o := recoveryStateFixture(t, dir)
	recoveryStateWrite(t, filepath.Join(dir, "disabled/00-base.json"), `{}`)
	if err := o.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "00-base.json")); err != nil {
		t.Fatal("legacy Bootstrap behavior changed", err)
	}
}
