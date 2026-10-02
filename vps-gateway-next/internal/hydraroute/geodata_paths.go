package hydraroute

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const geoSubdir = "geo"

// resolveGeoConfigPath turns a path from hrneo.conf into an absolute path.
// Relative entries are resolved under /opt/etc/HydraRoute.
func resolveGeoConfigPath(p string) string {
	clean := filepath.Clean(strings.TrimSpace(p))
	if clean == "" || clean == "." {
		return ""
	}
	if filepath.IsAbs(clean) {
		return clean
	}
	return filepath.Clean(filepath.Join(hrDir, clean))
}

// hasPathPrefix reports whether clean is equal to prefix or is a child of prefix.
func hasPathPrefix(clean, prefix string) bool {
	clean = filepath.Clean(clean)
	prefix = filepath.Clean(prefix)
	if clean == prefix {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(clean, prefix+sep)
}

// isManagedPath reports whether path is under awg-manager/geo or HydraRoute's
// directory — the only locations we allow list/update/delete operations on.
func (s *GeoDataStore) isManagedPath(path string) bool {
	if s == nil {
		return false
	}
	clean := filepath.Clean(path)
	if s.geoDir != "" && hasPathPrefix(clean, s.geoDir) {
		return true
	}
	return hasPathPrefix(clean, hrDir)
}

// isHRPath reports whether path lives under HydraRoute but not awg-manager/geo.
func (s *GeoDataStore) isHRPath(path string) bool {
	if s == nil {
		return false
	}
	clean := filepath.Clean(path)
	if s.geoDir != "" && hasPathPrefix(clean, s.geoDir) {
		return false
	}
	return hasPathPrefix(clean, hrDir)
}

// relocateIntoGeoDirLocked moves src from HydraRoute into awg-manager/geo.
// Caller must hold s.mu (write lock).
func (s *GeoDataStore) relocateIntoGeoDirLocked(src string) (string, error) {
	src = filepath.Clean(src)
	if !s.isHRPath(src) {
		return "", fmt.Errorf("file is not under HydraRoute directory")
	}
	if err := os.MkdirAll(s.geoDir, 0o755); err != nil {
		return "", fmt.Errorf("create geo dir: %w", err)
	}
	dest := s.resolveConflict(filepath.Join(s.geoDir, filepath.Base(src)))
	if err := os.Rename(src, dest); err != nil {
		return "", fmt.Errorf("relocate %s -> %s: %w", src, dest, err)
	}
	return dest, nil
}
