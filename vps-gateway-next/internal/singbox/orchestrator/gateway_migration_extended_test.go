package orchestrator

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/configmerge"
)

// Snapshot without following links or opening special files. These fixtures
// live only in t.TempDir; no running engine or production state is involved.
func extendedMigrationTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += ":" + string(data)
		} else if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += ":" + target
		}
		result[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func extendedMigrationBytes(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("content mismatch at %s: got=%q want=%q err=%v", path, got, want, err)
	}
}

func TestGatewayMigrationExtended_PartialFailureThenRetry(t *testing.T) {
	for _, tc := range []struct {
		name, first, conflict string
		enabled               bool
	}{
		{"active-rename-before-disabled-archive-failure", "", "disabled", true},
		{"disabled-rename-before-pending-archive-failure", "disabled", "pending", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, dir := gatewayMigrationOrch(t)
			const first = `{"route":{"final":"first-sentinel"}}`
			const conflict = `{"route":{"final":"conflict-sentinel"}}`
			const current = `{"route":{"final":"current-draft"}}`
			migrationWrite(t, dir, tc.first, oldGatewayFilename, first)
			migrationWrite(t, dir, tc.conflict, oldGatewayFilename, conflict)
			if tc.conflict == "pending" {
				migrationWrite(t, dir, "pending", newGatewayFilename, current)
			} else {
				migrationWrite(t, dir, "pending", oldGatewayFilename, current)
			}
			archiveRoot := filepath.Join(dir, ".legacy-gateway-policy")
			migrationWrite(t, dir, "", ".legacy-gateway-policy", "archive-blocker")
			err := o.Bootstrap()
			if err == nil || !strings.Contains(err.Error(), "gateway filename migration archive") {
				t.Fatalf("expected archive failure after first rename: %v", err)
			}
			extendedMigrationBytes(t, filepath.Join(dir, tc.first, newGatewayFilename), first)
			if _, err := os.Lstat(filepath.Join(dir, tc.first, oldGatewayFilename)); !os.IsNotExist(err) {
				t.Fatalf("first rename not completed: %v", err)
			}
			extendedMigrationBytes(t, filepath.Join(dir, tc.conflict, oldGatewayFilename), conflict)
			extendedMigrationBytes(t, archiveRoot, "archive-blocker")
			pendingName := newGatewayFilename
			if tc.conflict == "disabled" {
				pendingName = oldGatewayFilename
			}
			extendedMigrationBytes(t, filepath.Join(dir, "pending", pendingName), current)
			if err := os.Remove(archiveRoot); err != nil {
				t.Fatal(err)
			}
			if err := o.Bootstrap(); err != nil {
				t.Fatalf("retry failed: %v", err)
			}
			got, enabled, err := o.SnapshotSlot(SlotGatewayPolicy)
			if err != nil || enabled != tc.enabled || string(got) != first {
				t.Fatalf("retry slot: enabled=%v data=%q err=%v", enabled, got, err)
			}
			extendedMigrationBytes(t, filepath.Join(dir, "pending", newGatewayFilename), current)
			archives, err := filepath.Glob(filepath.Join(archiveRoot, "migration-*", "*-"+oldGatewayFilename))
			if err != nil || len(archives) != 1 {
				t.Fatalf("retry must archive conflict exactly once: %v err=%v", archives, err)
			}
			extendedMigrationBytes(t, archives[0], conflict)
			for _, location := range []string{"", "disabled", "pending"} {
				if _, err := os.Lstat(filepath.Join(dir, location, oldGatewayFilename)); !os.IsNotExist(err) {
					t.Fatalf("legacy candidate survives retry at %q: %v", location, err)
				}
			}
			before := extendedMigrationTree(t, dir)
			if err := o.Bootstrap(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, extendedMigrationTree(t, dir)) {
				t.Fatal("successful retry not idempotent")
			}
		})
	}
}

func TestGatewayMigrationExtended_NonregularCandidates(t *testing.T) {
	for _, location := range []string{"", "disabled", "pending"} {
		for _, name := range []string{oldGatewayFilename, newGatewayFilename} {
			for _, kind := range []string{"directory", "symlink", "dangling-symlink", "fifo", "socket"} {
				t.Run(filepath.Join("active"+location, name, kind), func(t *testing.T) {
					o, dir := gatewayMigrationOrch(t)
					// Unix socket paths have a small kernel limit; use a short,
					// disposable Go fixture root rather than the subtest name.
					if kind == "socket" {
						short, err := os.MkdirTemp("", "gateway-candidate-")
						if err != nil {
							t.Fatal(err)
						}
						t.Cleanup(func() { _ = os.RemoveAll(short) })
						dir = short
						o.configDir = dir
					}
					if err := o.Bootstrap(); err != nil {
						t.Fatal(err)
					}
					if err := os.MkdirAll(filepath.Join(dir, location), 0755); err != nil {
						t.Fatal(err)
					}
					// A valid earlier candidate proves the whole candidate scan
					// rejects the later hazard before any migration writes.
					if location != "" || name == newGatewayFilename {
						migrationWrite(t, dir, "", oldGatewayFilename, `{"route":{"final":"untouched"}}`)
					}
					path := filepath.Join(dir, location, name)
					external := t.TempDir()
					migrationWrite(t, external, "", "sentinel.json", "external-sentinel")
					var err error
					switch kind {
					case "directory":
						err = os.Mkdir(path, 0700)
					case "symlink":
						err = os.Symlink(filepath.Join(external, "sentinel.json"), path)
					case "dangling-symlink":
						err = os.Symlink(filepath.Join(external, "absent.json"), path)
					case "fifo":
						err = syscall.Mkfifo(path, 0600)
					case "socket":
						var listener net.Listener
						listener, err = net.Listen("unix", path)
						if err == nil {
							t.Cleanup(func() { _ = listener.Close() })
						}
					}
					if err != nil {
						t.Fatal(err)
					}
					before := extendedMigrationTree(t, dir)
					outside := extendedMigrationTree(t, external)
					for attempt := 0; attempt < 2; attempt++ {
						err := o.Bootstrap()
						if err == nil || !strings.Contains(err.Error(), "refuses non-regular file") || !strings.Contains(err.Error(), path) {
							t.Fatalf("expected candidate refusal for %s: %v", path, err)
						}
						if !reflect.DeepEqual(before, extendedMigrationTree(t, dir)) || !reflect.DeepEqual(outside, extendedMigrationTree(t, external)) {
							t.Fatal("candidate rejection mutated fixture or external target")
						}
					}
				})
			}
		}
	}
}

func TestGatewayMigrationExtended_ConflictArchiveExcludedFromMergeDir(t *testing.T) {
	for _, currentLocation := range []string{"", "disabled"} {
		t.Run("current-"+currentLocation, func(t *testing.T) {
			o, dir := gatewayMigrationOrch(t)
			legacy := `{"inbounds":[{"type":"mixed","tag":"gateway"}],"route":{"final":"legacy-final","rules":[{"domain":["archived.invalid"],"action":"reject"}]}}`
			current := `{"inbounds":[{"type":"mixed","tag":"gateway"}],"route":{"final":"current-final"}}`
			migrationWrite(t, dir, "", oldGatewayFilename, legacy)
			migrationWrite(t, dir, currentLocation, newGatewayFilename, current)
			if err := o.Bootstrap(); err != nil {
				t.Fatal(err)
			}
			archives, err := filepath.Glob(filepath.Join(dir, ".legacy-gateway-policy", "migration-*", "active-"+oldGatewayFilename))
			if err != nil || len(archives) != 1 {
				t.Fatalf("archive missing: %v err=%v", archives, err)
			}
			extendedMigrationBytes(t, archives[0], legacy)
			merged, err := configmerge.MergeDir(dir)
			if err != nil {
				t.Fatalf("actual MergeDir reads duplicate archived inbound: %v", err)
			}
			var cfg struct {
				Inbounds []json.RawMessage          `json:"inbounds"`
				Route    map[string]json.RawMessage `json:"route"`
			}
			if err := json.Unmarshal([]byte(merged), &cfg); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(merged, "legacy-final") || strings.Contains(merged, "archived.invalid") {
				t.Fatalf("archive leaked into actual MergeDir: %s", merged)
			}
			if currentLocation == "" {
				if len(cfg.Inbounds) != 1 || string(cfg.Route["final"]) != `"current-final"` {
					t.Fatalf("current active content not preserved in merge: %s", merged)
				}
			} else if len(cfg.Inbounds) != 0 || cfg.Route["final"] != nil {
				t.Fatalf("disabled current or archived legacy activated: %s", merged)
			}
		})
	}
}

func TestGatewayMigrationExtended_CompatibilityGuards(t *testing.T) {
	for _, registered := range []bool{false, true} {
		for _, special := range []bool{false, true} {
			name := "absent"
			if registered {
				name = "custom"
			}
			if special {
				name += "-nonregular-pending"
			}
			t.Run(name, func(t *testing.T) {
				o, dir := newTestOrch(t)
				o.HoldReloads()
				const customName = "42-custom-gateway.json"
				const custom = `{"route":{"final":"custom-disabled"}}`
				if registered {
					if err := o.Register(SlotMeta{Slot: SlotGatewayPolicy, Filename: customName}); err != nil {
						t.Fatal(err)
					}
				}
				if err := o.Bootstrap(); err != nil {
					t.Fatal(err)
				}
				for _, location := range []string{"", "disabled", "pending"} {
					for _, candidate := range []string{oldGatewayFilename, newGatewayFilename} {
						migrationWrite(t, dir, location, candidate, `{"route":{"final":"compat-sentinel"}}`)
					}
				}
				if registered {
					migrationWrite(t, dir, "disabled", customName, custom)
				}
				if special {
					path := filepath.Join(dir, "pending", newGatewayFilename)
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				}
				before := extendedMigrationTree(t, dir)
				for attempt := 0; attempt < 2; attempt++ {
					if err := o.Bootstrap(); err != nil {
						t.Fatalf("migration must not run for absent/custom contract: %v", err)
					}
					if !reflect.DeepEqual(before, extendedMigrationTree(t, dir)) {
						t.Fatal("compatibility guard modified unowned candidates")
					}
					if registered {
						got, enabled, err := o.SnapshotSlot(SlotGatewayPolicy)
						if err != nil || enabled || string(got) != custom {
							t.Fatalf("custom disabled contract changed: enabled=%v data=%q err=%v", enabled, got, err)
						}
					}
				}
			})
		}
	}
}
