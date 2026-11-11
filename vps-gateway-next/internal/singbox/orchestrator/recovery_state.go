package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// LoadRecoveryState is the read-only discovery boundary for original startup.
// The original factory must call it before recovery and publishing producers.
func (o *Orchestrator) LoadRecoveryState(ctx context.Context) error {
	// This is startup preparation, not permission to run. Failures and
	// cancellation must leave normal lifecycle operations blocked.
	o.SetRuntimeRecoveryRequired(true)
	if err := ctx.Err(); err != nil {
		return err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return fmt.Errorf("orchestrator is closed")
	}
	if err := ValidateStateDirectories(o.configDir); err != nil {
		return err
	}
	known := make(map[string]bool, len(o.slots))
	for _, meta := range o.slots {
		if filepath.Base(meta.Filename) != meta.Filename {
			return fmt.Errorf("invalid recovery slot filename")
		}
		known[meta.Filename] = true
	}
	entries, err := os.ReadDir(o.configDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("inspect recovery config directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" && !known[entry.Name()] {
			return fmt.Errorf("unregistered active JSON requires ownership recovery")
		}
	}
	enabled := make(map[Slot]bool, len(o.slots))
	for slot, meta := range o.slots {
		active, err := recoveryRegularFile(o.activePath(meta))
		if err != nil {
			return err
		}
		if _, err := recoveryRegularFile(o.disabledPath(meta)); err != nil {
			return err
		}
		// Match what -C actually reads. Preserve both copies and disabled
		// AlwaysOn resources until the journal owner decides their fate.
		enabled[slot] = active
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	o.enabled = enabled
	return nil
}

func recoveryRegularFile(path string) (bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect recovery slot: %w", err)
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("recovery slot must be a regular file")
	}
	return true, nil
}
