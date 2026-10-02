package policy

import (
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

const (
	GatewayInboundTag              = "awg-gateway-in"
	GatewayTUNInterface            = "awgm0"
	GatewayTUNAddress              = "198.18.0.1/30"
	DefaultGatewayClientPool       = "10.66.0.0/24"
	DefaultGatewayIngressInterface = "awg0"
)

var gatewayInterfaceName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,15}$`)

// CompileOptions supplies runtime-owned outbound tags and client address bindings.
type CompileOptions struct {
	Outbounds        map[string]struct{}
	OutboundActions  map[string]Action
	DefaultOutbounds map[Action]string
	Clients          map[string][]string
	Groups           map[string][]string
	InboundTags      map[string]struct{}
	ClientPool       netip.Prefix
	IngressInterface string
	WANInterface     string
}

// CompiledPolicy is the deterministic route-policy output consumed by a
// Sing-box config adapter. Final is an outbound tag, or "block" for a blocked
// default action.
type CompiledPolicy struct {
	Final            string         `json:"final"`
	Rules            []CompiledRule `json:"rules"`
	ClientPool       netip.Prefix   `json:"-"`
	IngressInterface string         `json:"-"`
	WANInterface     string         `json:"-"`
}

// CompiledRule is deliberately limited to portable sing-box route fields.
// Runtime-specific fields belong in the adapter, not in policy validation.
type CompiledRule struct {
	Inbound      []string `json:"inbound,omitempty"`
	ID           string   `json:"-"`
	Domain       []string `json:"domain,omitempty"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
	RuleSet      []string `json:"rule_set,omitempty"`
	IPCIDR       []string `json:"ip_cidr,omitempty"`
	SourceIPCIDR []string `json:"source_ip_cidr,omitempty"`
	Protocol     string   `json:"protocol,omitempty"`
	Port         []int    `json:"port,omitempty"`
	Network      []string `json:"network,omitempty"`
	Action       string   `json:"action,omitempty"`
	Outbound     string   `json:"outbound,omitempty"`
}

// Compile validates and lowers a portable Profile into deterministic route
// rules. It does not write files, reload sing-box, or mutate runtime state.
func Compile(profile Profile, options CompileOptions) (CompiledPolicy, error) {
	ordered, err := profile.OrderedRules()
	if err != nil {
		return CompiledPolicy{}, err
	}

	clientPool := options.ClientPool
	if !clientPool.IsValid() {
		clientPool = netip.MustParsePrefix(DefaultGatewayClientPool)
	}
	if !clientPool.Addr().Is4() || clientPool.Bits() > 30 {
		return CompiledPolicy{}, fmt.Errorf("invalid IPv4 gateway client pool")
	}
	clientPool = clientPool.Masked()

	ingressInterface := strings.TrimSpace(options.IngressInterface)
	if ingressInterface == "" {
		ingressInterface = DefaultGatewayIngressInterface
	}
	if !gatewayInterfaceName.MatchString(ingressInterface) {
		return CompiledPolicy{}, fmt.Errorf("invalid gateway ingress interface")
	}
	wanInterface := strings.TrimSpace(options.WANInterface)
	if wanInterface != "" && !gatewayInterfaceName.MatchString(wanInterface) {
		return CompiledPolicy{}, fmt.Errorf("invalid gateway WAN interface")
	}

	inboundTags := make(map[string]struct{}, len(options.InboundTags)+1)
	for tag := range options.InboundTags {
		inboundTags[tag] = struct{}{}
	}
	inboundTags[GatewayInboundTag] = struct{}{}
	options.InboundTags = inboundTags

	final, err := resolveActionOutbound(profile.DefaultAction, "default", options)
	if err != nil {
		return CompiledPolicy{}, err
	}
	if _, loops := options.InboundTags[final]; loops {
		return CompiledPolicy{}, fmt.Errorf("default action: route loop through inbound tag %q", final)
	}

	compiled := CompiledPolicy{
		Final:            final,
		Rules:            make([]CompiledRule, 0, len(ordered)),
		ClientPool:       clientPool,
		IngressInterface: ingressInterface,
		WANInterface:     wanInterface,
	}
	for _, rule := range ordered {
		if !rule.Enabled {
			continue
		}
		if rule.Action != ActionBlock {
			if _, ok := options.Outbounds[rule.Outbound]; !ok {
				return CompiledPolicy{}, fmt.Errorf("rule %q: missing outbound %q", rule.ID, rule.Outbound)
			}
			if options.OutboundActions[rule.Outbound] != rule.Action {
				return CompiledPolicy{}, fmt.Errorf("rule %q: outbound %q does not match action %q", rule.ID, rule.Outbound, rule.Action)
			}
			if _, loops := options.InboundTags[rule.Outbound]; loops {
				return CompiledPolicy{}, fmt.Errorf("rule %q: route loop through inbound tag %q", rule.ID, rule.Outbound)
			}
		}

		sources, err := resolveSources(rule, options)
		if err != nil {
			return CompiledPolicy{}, fmt.Errorf("rule %q: %w", rule.ID, err)
		}
		out := CompiledRule{
			Inbound:      []string{GatewayInboundTag},
			ID:           rule.ID,
			Domain:       append([]string(nil), rule.Domains...),
			DomainSuffix: append([]string(nil), rule.DomainSuffixes...),
			RuleSet:      append([]string(nil), rule.RuleSets...),
			IPCIDR:       append([]string(nil), rule.CIDRs...),
			SourceIPCIDR: sources,
			Port:         append([]int(nil), rule.Ports...),
			Network:      append([]string(nil), rule.Protocols...),
		}
		for i := range out.Network {
			out.Network[i] = strings.TrimSpace(out.Network[i])
		}
		if rule.Action == ActionBlock {
			out.Action = "reject"
		} else {
			out.Action = "route"
			out.Outbound = rule.Outbound
		}
		compiled.Rules = append(compiled.Rules, out)
	}

	return compiled, nil
}

func resolveActionOutbound(action Action, scope string, options CompileOptions) (string, error) {
	if action == ActionBlock {
		return "block", nil
	}
	outbound := options.DefaultOutbounds[action]
	if outbound == "" {
		return "", fmt.Errorf("%s action %q has no default outbound", scope, action)
	}
	if _, ok := options.Outbounds[outbound]; !ok {
		return "", fmt.Errorf("%s action: missing outbound %q", scope, outbound)
	}
	if options.OutboundActions[outbound] != action {
		return "", fmt.Errorf("%s action: outbound %q does not match action %q", scope, outbound, action)
	}
	return outbound, nil
}

func resolveSources(rule Rule, options CompileOptions) ([]string, error) {
	var sources []string
	sources = append(sources, rule.SourceCIDRs...)
	for _, clientID := range rule.ClientIDs {
		addresses, ok := options.Clients[clientID]
		if !ok || len(addresses) == 0 {
			return nil, fmt.Errorf("unknown client %q", clientID)
		}
		sources = append(sources, addresses...)
	}
	for _, groupID := range rule.GroupIDs {
		addresses, ok := options.Groups[groupID]
		if !ok || len(addresses) == 0 {
			return nil, fmt.Errorf("unknown group %q", groupID)
		}
		sources = append(sources, addresses...)
	}
	return uniqueStrings(sources), nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
