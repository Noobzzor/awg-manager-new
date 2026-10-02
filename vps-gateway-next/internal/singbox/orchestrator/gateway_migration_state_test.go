package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayMigration_PreservesStateAndConflicts(t *testing.T) {
	legacy := `{"route":{"final":"legacy"}}`
	current := `{"route":{"final":"current"}}`
	for _, tc := range []struct {
		name          string
		oldLocations  []string
		newLocations  []string
		wantLocations []string
		wantEnabled   bool
		wantArchived  int
	}{
		{"active", []string{""}, nil, []string{""}, true, 0},
		{"disabled", []string{"disabled"}, nil, []string{"disabled"}, false, 0},
		{"pending", []string{"pending"}, nil, []string{"pending"}, false, 0},
		{"active-with-draft", []string{"", "pending"}, nil, []string{"", "pending"}, true, 0},
		{"current-active", []string{""}, []string{""}, []string{""}, true, 1},
		{"current-disabled-wins", []string{""}, []string{"disabled"}, []string{"disabled"}, false, 1},
		{"current-active-preserves-old-disabled", []string{"disabled"}, []string{""}, []string{""}, true, 1},
		{"current-draft-preserves-old-draft", []string{"pending"}, []string{"pending"}, []string{"pending"}, false, 1},
		{"current-draft-keeps-legacy-active", []string{"", "pending"}, []string{"pending"}, []string{"", "pending"}, true, 1},
		{"legacy-active-wins-preserves-disabled", []string{"", "disabled"}, nil, []string{""}, true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, dir := gatewayMigrationOrch(t)
			for _, location := range tc.oldLocations {
				migrationWrite(t, dir, location, oldGatewayFilename, legacy)
			}
			for _, location := range tc.newLocations {
				migrationWrite(t, dir, location, newGatewayFilename, current)
			}
			for attempt := 0; attempt < 2; attempt++ {
				if err := o.Bootstrap(); err != nil {
					t.Fatal(err)
				}
				_, enabled, err := o.SnapshotSlot(SlotGatewayPolicy)
				if err != nil || enabled != tc.wantEnabled {
					t.Fatalf("enabled=%v want=%v err=%v", enabled, tc.wantEnabled, err)
				}
				for _, location := range tc.oldLocations {
					if _, err := os.Stat(filepath.Join(dir, location, oldGatewayFilename)); !os.IsNotExist(err) {
						t.Fatalf("legacy file remains at %q: %v", location, err)
					}
				}
				for _, location := range tc.wantLocations {
					data, err := os.ReadFile(filepath.Join(dir, location, newGatewayFilename))
					if err != nil {
						t.Fatal(err)
					}
					want := legacy
					for _, existing := range tc.newLocations {
						if existing == location {
							want = current
						}
					}
					if string(data) != want {
						t.Fatalf("content lost at %q: %s", location, data)
					}
				}
				archived := 0
				err = filepath.Walk(filepath.Join(dir, ".legacy-gateway-policy"), func(path string, info os.FileInfo, walkErr error) error {
					if os.IsNotExist(walkErr) {
						return nil
					}
					if walkErr != nil {
						return walkErr
					}
					if info.IsDir() {
						return nil
					}
					data, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					if string(data) != legacy {
						t.Fatal("archive did not preserve legacy content")
					}
					archived++
					return nil
				})
				if err != nil || archived != tc.wantArchived {
					t.Fatalf("archived=%d want=%d err=%v", archived, tc.wantArchived, err)
				}
			}
		})
	}
}
