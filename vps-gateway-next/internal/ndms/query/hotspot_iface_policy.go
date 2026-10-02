package query

import (
	"context"
	"strings"
)

// GetInterfaceHotspotPolicy returns the ip hotspot policy bound to iface.
// "none" when the interface uses default-permit (no explicit policy line).
func (s *RunningConfigStore) GetInterfaceHotspotPolicy(ctx context.Context, iface string) (string, error) {
	lines, err := s.Lines(ctx)
	if err != nil {
		return "", err
	}
	inHotspot := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "ip hotspot" {
			inHotspot = true
			continue
		}
		if !inHotspot {
			continue
		}
		if trimmed == "!" {
			break
		}
		if !strings.HasPrefix(trimmed, "policy ") {
			continue
		}
		parts := strings.Fields(trimmed)
		// policy <interface> <access|policyName>
		if len(parts) < 3 || parts[1] != iface {
			continue
		}
		return parts[2], nil
	}
	return "none", nil
}
