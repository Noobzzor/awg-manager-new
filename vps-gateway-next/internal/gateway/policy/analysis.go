package policy

import (
	"encoding/json"
	"fmt"
)

// IssueKind identifies a deterministic policy analysis warning.
type IssueKind string

const (
	IssueDuplicateMatcher IssueKind = "duplicate_matcher"
	IssueShadowed         IssueKind = "shadowed_by_catch_all"
)

// Issue describes a rule that may never be reached as written.
type Issue struct {
	Kind       IssueKind `json:"kind"`
	RuleID     string    `json:"ruleId"`
	ShadowedBy string    `json:"shadowedBy,omitempty"`
}

// Analyze validates a profile and reports deterministic shadowing hazards. It
// intentionally returns warnings instead of rejecting the profile: an explicit
// overlap may be deliberate and can later be reviewed by Jev/TypeSafe.
func Analyze(profile Profile) ([]Issue, error) {
	ordered, err := profile.OrderedRules()
	if err != nil {
		return nil, err
	}

	issues := make([]Issue, 0)
	seen := make(map[string]string, len(ordered))
	var previousActiveRule *Rule
	for _, rule := range ordered {
		if !rule.Enabled {
			continue
		}
		fingerprint, err := matcherFingerprint(rule)
		if err != nil {
			return nil, fmt.Errorf("rule %q matcher: %w", rule.ID, err)
		}
		if previous, ok := seen[fingerprint]; ok {
			issues = append(issues, Issue{Kind: IssueDuplicateMatcher, RuleID: rule.ID, ShadowedBy: previous})
		} else {
			seen[fingerprint] = rule.ID
		}
		if previousActiveRule != nil && isCatchAll(*previousActiveRule) {
			issues = append(issues, Issue{Kind: IssueShadowed, RuleID: rule.ID, ShadowedBy: previousActiveRule.ID})
		}
		activeRule := rule
		previousActiveRule = &activeRule
	}
	return issues, nil
}

type matcherShape struct {
	ClientIDs      []string `json:"clientIds,omitempty"`
	GroupIDs       []string `json:"groupIds,omitempty"`
	SourceCIDRs    []string `json:"sourceCidrs,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	DomainSuffixes []string `json:"domainSuffixes,omitempty"`
	RuleSets       []string `json:"ruleSets,omitempty"`
	CIDRs          []string `json:"cidrs,omitempty"`
	Ports          []int    `json:"ports,omitempty"`
	Protocols      []string `json:"protocols,omitempty"`
}

func matcherFingerprint(rule Rule) (string, error) {
	data, err := json.Marshal(matcherShape{
		ClientIDs:      rule.ClientIDs,
		GroupIDs:       rule.GroupIDs,
		SourceCIDRs:    rule.SourceCIDRs,
		Domains:        rule.Domains,
		DomainSuffixes: rule.DomainSuffixes,
		RuleSets:       rule.RuleSets,
		CIDRs:          rule.CIDRs,
		Ports:          rule.Ports,
		Protocols:      rule.Protocols,
	})
	return string(data), err
}

func isCatchAll(rule Rule) bool {
	return len(rule.ClientIDs) == 0 && len(rule.GroupIDs) == 0 &&
		len(rule.SourceCIDRs) == 0 && len(rule.Domains) == 0 &&
		len(rule.DomainSuffixes) == 0 && len(rule.RuleSets) == 0 &&
		len(rule.CIDRs) == 0 && len(rule.Ports) == 0 && len(rule.Protocols) == 0
}
