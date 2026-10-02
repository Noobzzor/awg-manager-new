package policy

import (
	"fmt"
	"sort"

	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
)

// AWG3Catalog is the existing durable AWG3 endpoint service projection.
type AWG3Catalog interface {
	ListTags() []awg3endpoint.TagInfo
}

// CompileOptionsFromAWG3 builds compiler inputs from the managed AWG3 catalog.
// selectedTag is explicit when non-empty; otherwise the lexicographically first
// AWG3 endpoint is selected for deterministic behavior. DIRECT is always
// available, but VPN is only registered when a real AWG3 endpoint exists.
func CompileOptionsFromAWG3(catalog AWG3Catalog, selectedTag string) (CompileOptions, error) {
	if catalog == nil {
		return CompileOptions{}, fmt.Errorf("AWG3 catalog is required")
	}

	tags := make([]string, 0)
	seen := make(map[string]struct{})
	for _, info := range catalog.ListTags() {
		if info.Kind != "awg3" || info.Tag == "" {
			continue
		}
		if info.Tag == "direct" {
			return CompileOptions{}, fmt.Errorf("AWG3 endpoint tag %q is reserved", info.Tag)
		}
		if _, ok := seen[info.Tag]; ok {
			continue
		}
		seen[info.Tag] = struct{}{}
		tags = append(tags, info.Tag)
	}
	sort.Strings(tags)
	if selectedTag != "" {
		if _, ok := seen[selectedTag]; !ok {
			return CompileOptions{}, fmt.Errorf("selected AWG3 endpoint %q is not available", selectedTag)
		}
	}
	if selectedTag == "" && len(tags) > 0 {
		selectedTag = tags[0]
	}

	options := CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	}
	for _, tag := range tags {
		options.Outbounds[tag] = struct{}{}
		options.OutboundActions[tag] = ActionVPN
	}
	if selectedTag != "" {
		options.DefaultOutbounds[ActionVPN] = selectedTag
	}
	return options, nil
}

// AddWARP adds a validated WARP provider to compiler inputs. It is explicit:
// the compiler never invents a WARP outbound or redirects WARP failures to
// DIRECT.
func AddWARP(options *CompileOptions, cfg WARPConfig) error {
	if options == nil {
		return fmt.Errorf("compile options are required")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if cfg.Tag == "direct" {
		return fmt.Errorf("WARP outbound tag %q is reserved", cfg.Tag)
	}
	if _, exists := options.Outbounds[cfg.Tag]; exists && options.OutboundActions[cfg.Tag] != ActionWARP {
		return fmt.Errorf("WARP outbound tag %q is already assigned to another action", cfg.Tag)
	}
	if options.Outbounds == nil {
		options.Outbounds = make(map[string]struct{})
	}
	if options.DefaultOutbounds == nil {
		options.DefaultOutbounds = make(map[Action]string)
	}
	if options.OutboundActions == nil {
		options.OutboundActions = make(map[string]Action)
	}
	options.Outbounds[cfg.Tag] = struct{}{}
	options.OutboundActions[cfg.Tag] = ActionWARP
	options.DefaultOutbounds[ActionWARP] = cfg.Tag
	return nil
}
