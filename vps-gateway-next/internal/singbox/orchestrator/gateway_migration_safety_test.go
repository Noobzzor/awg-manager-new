package orchestrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGatewayMigration_RejectsSymlinkStateDirectories(t *testing.T) {
	for _, location := range []string{"pending", "disabled"} {
		t.Run(location, func(t *testing.T) {
			o, dir := gatewayMigrationOrch(t)
			external := t.TempDir()
			const legacy = `{"route":{"final":"external-sentinel"}}`
			migrationWrite(t, external, "", oldGatewayFilename, legacy)
			migrationWrite(t, external, "", "sentinel.tmp.keep", legacy)
			migrationWrite(t, dir, "", oldGatewayFilename, legacy)
			if err := os.Symlink(external, filepath.Join(dir, location)); err != nil {
				t.Fatal(err)
			}
			if err := o.Bootstrap(); err == nil {
				t.Fatal("Bootstrap accepted a symlinked state directory")
			}
			if data, err := os.ReadFile(filepath.Join(external, "sentinel.tmp.keep")); err != nil || string(data) != legacy {
				t.Fatalf("startup sweep modified external content: %v", err)
			}
			for _, root := range []string{dir, external} {
				data, err := os.ReadFile(filepath.Join(root, oldGatewayFilename))
				if err != nil || string(data) != legacy {
					t.Fatalf("legacy content changed: err=%v", err)
				}
				if _, err := os.Lstat(filepath.Join(root, newGatewayFilename)); !os.IsNotExist(err) {
					t.Fatalf("new filename created before refusing symlink: %v", err)
				}
			}
		})
	}
}
