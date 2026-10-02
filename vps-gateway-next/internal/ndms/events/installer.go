package events

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

//go:embed hook-script.sh
var hookScriptContent string

// HookDirs are the NDMS hook directories we install our script into.
var HookDirs = []string{
	"iflayerchanged",
	"ifcreated",
	"ifdestroyed",
	"ifipchanged",
}

// Installer deploys the shared hook forwarder script into every NDMS
// hook directory we consume. Idempotent — re-running is safe.
type Installer struct {
	Root string
	Log  Logger
}

// NewInstaller returns an installer targeting the production NDMS root.
func NewInstaller(log Logger) *Installer {
	if log == nil {
		log = NopLogger()
	}
	return &Installer{Root: "/opt/etc/ndm", Log: log}
}

// Install deploys (or re-deploys) the hook script into every directory
// in HookDirs. Returns the first error encountered after logging it; all
// directories are attempted even if one fails.
func (i *Installer) Install() error {
	var firstErr error
	for _, hook := range HookDirs {
		dir := filepath.Join(i.Root, hook+".d")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			i.Log.Warnf("install hook %s: mkdir: %v", hook, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		path := filepath.Join(dir, "50-awg-manager.sh")

		existing, readErr := os.ReadFile(path)
		if readErr == nil && string(existing) == hookScriptContent {
			if err := os.Chmod(path, 0o755); err != nil {
				i.Log.Warnf("install hook %s: chmod: %v", hook, err)
			}
			continue
		}

		if err := storage.AtomicWrite(path, []byte(hookScriptContent)); err != nil {
			i.Log.Warnf("install hook %s: write: %v", hook, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := os.Chmod(path, 0o755); err != nil {
			i.Log.Warnf("install hook %s: chmod: %v", hook, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}
