package policy

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

// Action is the egress decision for a gateway policy rule.
type Action string

const (
	ActionVPN    Action = "vpn"
	ActionWARP   Action = "warp"
	ActionDirect Action = "direct"
	ActionBlock  Action = "block"
)

// Rule describes one deterministic, source-aware gateway routing rule.
type Rule struct {
	ID             string   `json:"id"`
	Action         Action   `json:"action"`
	Outbound       string   `json:"outbound,omitempty"`
	Priority       int      `json:"priority,omitempty"`
	ClientIDs      []string `json:"clientIds,omitempty"`
	GroupIDs       []string `json:"groupIds,omitempty"`
	SourceCIDRs    []string `json:"sourceCidrs,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	DomainSuffixes []string `json:"domainSuffixes,omitempty"`
	RuleSets       []string `json:"ruleSets,omitempty"`
	CIDRs          []string `json:"cidrs,omitempty"`
	Ports          []int    `json:"ports,omitempty"`
	Protocols      []string `json:"protocols,omitempty"`
	Enabled        bool     `json:"enabled"`
}

// Profile is the portable policy assigned to a client or client group.
type Profile struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	DefaultAction Action `json:"defaultAction"`
	Rules         []Rule `json:"rules,omitempty"`
}

// Validate checks the portable contract before any compiler or runtime apply.
func (p Profile) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("profile id is required")
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("profile name is required")
	}
	if err := validateAction(p.DefaultAction); err != nil {
		return fmt.Errorf("default action: %w", err)
	}

	seen := make(map[string]struct{}, len(p.Rules))
	for i, rule := range p.Rules {
		if strings.TrimSpace(rule.ID) == "" {
			return fmt.Errorf("rule %d: id is required", i)
		}
		if _, ok := seen[rule.ID]; ok {
			return fmt.Errorf("duplicate rule id %q", rule.ID)
		}
		seen[rule.ID] = struct{}{}
		if err := rule.validate(i); err != nil {
			return err
		}
	}
	return nil
}

func (r Rule) validate(index int) error {
	if r.Priority < 0 {
		return fmt.Errorf("rule %d: priority must not be negative", index)
	}
	if err := validateAction(r.Action); err != nil {
		return fmt.Errorf("rule %d: %w", index, err)
	}
	if r.Action == ActionBlock {
		if strings.TrimSpace(r.Outbound) != "" {
			return fmt.Errorf("rule %d: block action must not define an outbound", index)
		}
	} else if strings.TrimSpace(r.Outbound) == "" {
		return fmt.Errorf("rule %d: %s action requires an outbound", index, r.Action)
	}

	for _, value := range append(append([]string{}, r.SourceCIDRs...), r.CIDRs...) {
		if _, err := netip.ParsePrefix(value); err != nil {
			return fmt.Errorf("rule %d: invalid CIDR %q", index, value)
		}
	}
	for _, port := range r.Ports {
		if port < 1 || port > 65535 {
			return fmt.Errorf("rule %d: invalid port %d", index, port)
		}
	}
	for _, protocol := range r.Protocols {
		switch strings.TrimSpace(protocol) {
		case "tcp", "udp", "icmp":
		default:
			return fmt.Errorf("rule %d: invalid protocol %q", index, protocol)
		}
	}
	return nil
}

// OrderedRules validates the profile and returns a stable priority ordering.
// Rules with equal priority retain their original order.
func (p Profile) OrderedRules() ([]Rule, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	ordered := append([]Rule(nil), p.Rules...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Priority < ordered[j].Priority
	})
	return ordered, nil
}

func validateAction(action Action) error {
	switch action {
	case ActionVPN, ActionWARP, ActionDirect, ActionBlock:
		return nil
	default:
		return fmt.Errorf("unsupported action %q", action)
	}
}
