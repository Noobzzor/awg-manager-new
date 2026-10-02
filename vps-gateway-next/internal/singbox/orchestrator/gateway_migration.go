package orchestrator

import (
	"fmt"
	"os"
	"path/filepath"
)

const legacyGatewayPolicyFilename = "19-gateway-policy.json"
const gatewayPolicyFilename = "18-z-gateway-policy.json"

// migrateGatewayPolicyLocked keeps legacy files inside the managed slot contract.
// Existing current state wins; conflicting legacy content is retained outside
// the non-recursive live merge. Caller must hold o.mu before the enabled scan.
func (o *Orchestrator) migrateGatewayPolicyLocked() error {
	meta, registered := o.slots[SlotGatewayPolicy]
	if !registered || meta.Filename != gatewayPolicyFilename {
		return nil
	}
	locations := []string{"", disabledSubdir, "pending"}
	oldPresent := make([]bool, len(locations))
	newPresent := make([]bool, len(locations))
	for i, location := range locations {
		for _, candidate := range []struct {
			name    string
			present *bool
		}{
			{legacyGatewayPolicyFilename, &oldPresent[i]},
			{meta.Filename, &newPresent[i]},
		} {
			path := filepath.Join(o.configDir, location, candidate.name)
			info, err := os.Lstat(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return fmt.Errorf("gateway filename migration inspect %s: %w", path, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("gateway filename migration refuses non-regular file %s", path)
			}
			*candidate.present = true
		}
	}

	archiveDir := ""
	archive := func(path, location string) error {
		if archiveDir == "" {
			root := filepath.Join(o.configDir, ".legacy-gateway-policy")
			info, err := os.Lstat(root)
			if os.IsNotExist(err) {
				if err := os.Mkdir(root, 0700); err != nil {
					return err
				}
			} else if err != nil {
				return err
			} else if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("gateway migration archive is not a regular directory")
			}
			archiveDir, err = os.MkdirTemp(root, "migration-")
			if err != nil {
				return err
			}
		}
		if location == "" {
			location = "active"
		}
		return os.Rename(path, filepath.Join(archiveDir, location+"-"+legacyGatewayPolicyFilename))
	}

	for i, location := range locations {
		if !oldPresent[i] {
			continue
		}
		oldPath := filepath.Join(o.configDir, location, legacyGatewayPolicyFilename)
		// Active/disabled are one state channel; pending is an independent
		// draft channel. A new draft must not disable a legacy active policy.
		conflict := newPresent[i]
		if i < 2 {
			conflict = newPresent[0] || newPresent[1]
		}
		if conflict {
			if err := archive(oldPath, location); err != nil {
				return fmt.Errorf("gateway filename migration archive: %w", err)
			}
			continue
		}
		if err := os.Rename(oldPath, filepath.Join(o.configDir, location, meta.Filename)); err != nil {
			return fmt.Errorf("gateway filename migration rename: %w", err)
		}
		newPresent[i] = true
	}
	return nil
}
