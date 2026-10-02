package dnsroute

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"unicode"

	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

const (
	dnsRouteURLTestURL      = "https://www.gstatic.com/generate_204"
	dnsRouteURLTestInterval = "30s"
	// The fork's supported urltest keeps the earlier member when delays differ
	// by no more than tolerance. Its probe timeout is 15s, so 30s makes stored
	// order the sticky initial availability preference rather than a
	// fastest-server policy. Runtime recovery does not guarantee automatic
	// failback to an earlier member.
	dnsRouteURLTestTolerance = 30000
)

// SingboxCompileOptions supplies deployment-specific values without coupling
// compilation to environment, storage, or process state. ResolveTarget returns
// an existing outbound tag for the opaque persisted target.
type SingboxCompileOptions struct {
	DNSUpstream          string
	ResolveTarget        func(RouteTarget) (string, error)
	ReservedOutboundTags map[string]struct{}
	ReservedDNSTags      map[string]struct{}
}

type singboxDNSRouteFragment struct {
	DNS       *router.DNS       `json:"dns,omitempty"`
	Outbounds []router.Outbound `json:"outbounds,omitempty"`
	Route     *singboxRoute     `json:"route,omitempty"`
}

type singboxRoute struct {
	Rules []router.Rule `json:"rules"`
}

type compiledSingboxList struct {
	list            DomainList
	domains         []string
	excludeDomains  []string
	cidrs           []string
	excludeCIDRs    []string
	baseTag         string
	groupTag        string
	dnsTag          string
	directDNSTag    string
	resolvedTargets []string
	routeOutbound   string
	routeAction     string
	needsGroup      bool
	needsDirectDNS  bool
	needsTargetDNS  bool
}

// CompileSingbox deterministically compiles enabled sing-box DomainLists into
// one standalone config.d fragment. It is pure: it only resolves outbound tags
// and never reads, rewrites, or interprets endpoint JSON or AWG fields.
func CompileSingbox(lists []DomainList, options SingboxCompileOptions) ([]byte, error) {
	compiled, managedTags, err := prepareSingboxLists(lists, options)
	if err != nil {
		return nil, err
	}
	if len(compiled) == 0 {
		return []byte("{}\n"), nil
	}

	for i := range compiled {
		if err := resolveSingboxList(&compiled[i], options.ResolveTarget, managedTags); err != nil {
			return nil, err
		}
		compiled[i].needsDirectDNS = len(compiled[i].domains) > 0 && compiled[i].routeOutbound == "direct"
		compiled[i].needsTargetDNS = len(compiled[i].domains) > 0 && compiled[i].routeAction != "reject" && compiled[i].routeOutbound != "direct"
	}

	needsDNS := false
	for i := range compiled {
		needsDNS = needsDNS || compiled[i].needsDirectDNS || compiled[i].needsTargetDNS
	}
	upstream := strings.TrimSpace(options.DNSUpstream)
	if needsDNS {
		if upstream == "" {
			return nil, fmt.Errorf("sing-box DNS routes: DNS upstream is required")
		}
		if err := validateUDPUpstream(upstream); err != nil {
			return nil, err
		}
	}

	fragment := buildSingboxFragment(compiled, upstream)
	data, err := json.MarshalIndent(fragment, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sing-box DNS routes: %w", err)
	}
	return append(data, '\n'), nil
}

func prepareSingboxLists(lists []DomainList, options SingboxCompileOptions) ([]compiledSingboxList, map[string]string, error) {
	compiled := make([]compiledSingboxList, 0, len(lists))
	managed := make(map[string]string)
	for _, list := range lists {
		if list.Backend != BackendSingbox || !list.Enabled {
			continue
		}
		domains, err := normalizeSingboxDomains(list.Domains)
		if err != nil {
			return nil, nil, fmt.Errorf("sing-box DNS route %q: domains: %w", list.ID, err)
		}
		excludeDomains, err := normalizeSingboxDomains(list.Excludes)
		if err != nil {
			return nil, nil, fmt.Errorf("sing-box DNS route %q: excludes: %w", list.ID, err)
		}
		cidrs, err := normalizeSingboxCIDRs(list.Subnets)
		if err != nil {
			return nil, nil, fmt.Errorf("sing-box DNS route %q: subnets: %w", list.ID, err)
		}
		excludeCIDRs, err := normalizeSingboxCIDRs(list.ExcludeSubnets)
		if err != nil {
			return nil, nil, fmt.Errorf("sing-box DNS route %q: exclude subnets: %w", list.ID, err)
		}
		if len(domains)+len(cidrs) == 0 {
			continue
		}
		base := managedTagPart(list.ID)
		if base == "" {
			return nil, nil, fmt.Errorf("sing-box DNS route: enabled list has empty managed tag id")
		}
		item := compiledSingboxList{
			list: list, domains: domains, excludeDomains: excludeDomains,
			cidrs: cidrs, excludeCIDRs: excludeCIDRs, baseTag: "dnsroute-" + base,
		}
		item.groupTag = item.baseTag + "-outbound"
		item.dnsTag = item.baseTag + "-dns"
		item.directDNSTag = item.baseTag + "-direct-dns"
		for _, tag := range []string{item.groupTag, item.dnsTag, item.directDNSTag} {
			if owner, exists := managed[tag]; exists {
				return nil, nil, fmt.Errorf("sing-box DNS route managed tag collision %q between lists %q and %q", tag, owner, list.ID)
			}
			managed[tag] = list.ID
		}
		if _, exists := options.ReservedOutboundTags[item.groupTag]; exists {
			return nil, nil, fmt.Errorf("sing-box DNS route outbound tag collision %q with reserved tag", item.groupTag)
		}
		for _, tag := range []string{item.dnsTag, item.directDNSTag} {
			if _, exists := options.ReservedDNSTags[tag]; exists {
				return nil, nil, fmt.Errorf("sing-box DNS route DNS tag collision %q with reserved tag", tag)
			}
		}
		compiled = append(compiled, item)
	}
	return compiled, managed, nil
}

func resolveSingboxList(item *compiledSingboxList, resolver func(RouteTarget) (string, error), managed map[string]string) error {
	if len(item.list.Routes) == 0 {
		return fmt.Errorf("sing-box DNS route %q: missing target", item.list.ID)
	}
	if resolver == nil {
		return fmt.Errorf("sing-box DNS route %q: target resolver is required", item.list.ID)
	}
	seen := make(map[string]struct{}, len(item.list.Routes))
	for _, target := range item.list.Routes {
		tag, err := resolver(target)
		if err != nil {
			return fmt.Errorf("sing-box DNS route %q: resolve target: %w", item.list.ID, err)
		}
		if tag == "" {
			return fmt.Errorf("sing-box DNS route %q: missing target for tunnel %q", item.list.ID, target.TunnelID)
		}
		if owner, collision := managed[tag]; collision {
			return fmt.Errorf("sing-box DNS route target tag collision %q with managed list %q", tag, owner)
		}
		if _, duplicate := seen[tag]; duplicate {
			return fmt.Errorf("sing-box DNS route %q: duplicate target tag collision %q", item.list.ID, tag)
		}
		seen[tag] = struct{}{}
		item.resolvedTargets = append(item.resolvedTargets, tag)
	}

	first := item.resolvedTargets[0]
	if first == "reject" || first == "block" {
		if len(item.resolvedTargets) > 1 {
			return fmt.Errorf("sing-box DNS route %q: terminal target %q cannot have trailing targets", item.list.ID, first)
		}
		item.routeAction = "reject"
		return nil
	}
	if first == "direct" {
		if len(item.resolvedTargets) > 1 {
			return fmt.Errorf("sing-box DNS route %q: terminal target %q cannot have trailing targets", item.list.ID, first)
		}
		item.routeAction = "route"
		item.routeOutbound = "direct"
		return nil
	}
	for _, tag := range item.resolvedTargets[1:] {
		if tag == "direct" || tag == "reject" || tag == "block" {
			return fmt.Errorf("sing-box DNS route %q: terminal target %q must be first or expressed as fallback", item.list.ID, tag)
		}
	}

	members := append([]string(nil), item.resolvedTargets...)
	fallback := item.list.Routes[len(item.list.Routes)-1].Fallback
	switch fallback {
	case "", "reject":
	case "auto":
		members = append(members, "direct")
	default:
		return fmt.Errorf("sing-box DNS route %q: unsupported fallback %q", item.list.ID, fallback)
	}
	item.resolvedTargets = members
	item.routeAction = "route"
	item.needsGroup = len(members) > 1
	if item.needsGroup {
		item.routeOutbound = item.groupTag
	} else {
		item.routeOutbound = members[0]
	}
	return nil
}

func buildSingboxFragment(lists []compiledSingboxList, upstream string) singboxDNSRouteFragment {
	var dnsServers []router.DNSServer
	var dnsRules []router.DNSRule
	var routeRules []router.Rule
	var outbounds []router.Outbound

	for _, item := range lists {
		if item.needsDirectDNS {
			dnsServers = append(dnsServers, router.DNSServer{Tag: item.directDNSTag, Type: "udp", Server: upstream})
		}
		if item.needsTargetDNS {
			dnsServers = append(dnsServers, router.DNSServer{Tag: item.dnsTag, Type: "udp", Server: upstream, Detour: item.routeOutbound})
		}
		if item.needsGroup {
			outbounds = append(outbounds, router.Outbound{
				Type: "urltest", Tag: item.groupTag, Outbounds: item.resolvedTargets,
				URL: dnsRouteURLTestURL, Interval: dnsRouteURLTestInterval, Tolerance: dnsRouteURLTestTolerance,
			})
		}
	}
	for _, item := range lists {
		if len(item.domains) > 0 {
			match := router.DNSRule{DomainSuffix: item.domains}
			if len(item.excludeDomains) > 0 {
				match = router.DNSRule{Type: "logical", Mode: "and", Rules: []router.DNSRule{
					{DomainSuffix: item.domains},
					{DomainSuffix: item.excludeDomains, Invert: true},
				}}
			}
			if item.routeAction == "reject" {
				match.Action = "reject"
			} else {
				server := item.dnsTag
				if item.routeOutbound == "direct" {
					server = item.directDNSTag
				}
				match.Server = server
			}
			dnsRules = append(dnsRules, match)
		}
		if len(item.domains)+len(item.cidrs) > 0 {
			rule := router.Rule{DomainSuffix: item.domains, IPCIDR: item.cidrs, Action: item.routeAction, Outbound: item.routeOutbound}
			if len(item.excludeDomains)+len(item.excludeCIDRs) > 0 {
				rule = router.Rule{Type: "logical", Mode: "and", Rules: []router.Rule{
					{DomainSuffix: item.domains, IPCIDR: item.cidrs},
					{DomainSuffix: item.excludeDomains, IPCIDR: item.excludeCIDRs, Invert: true},
				}, Action: item.routeAction, Outbound: item.routeOutbound}
			}
			if item.routeAction == "reject" {
				rule.Outbound = ""
			}
			routeRules = append(routeRules, rule)
		}
	}

	fragment := singboxDNSRouteFragment{Outbounds: outbounds}
	if len(dnsServers)+len(dnsRules) > 0 {
		fragment.DNS = &router.DNS{Servers: dnsServers, Rules: dnsRules}
	}
	if len(routeRules) > 0 {
		fragment.Route = &singboxRoute{Rules: routeRules}
	}
	return fragment
}

func normalizeSingboxDomains(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		domain := strings.ToLower(strings.TrimSpace(raw))
		if strings.HasPrefix(domain, "geosite:") {
			return nil, fmt.Errorf("geosite: entries are not supported by the sing-box backend: %q", raw)
		}
		domain = strings.Trim(domain, ".")
		if domain == "" || strings.ContainsAny(domain, " /\\\t\r\n*") {
			return nil, fmt.Errorf("invalid domain %q", raw)
		}
		for _, label := range strings.Split(domain, ".") {
			if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return nil, fmt.Errorf("invalid domain %q", raw)
			}
			for _, r := range label {
				if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_') {
					return nil, fmt.Errorf("invalid domain %q", raw)
				}
			}
		}
		if _, duplicate := seen[domain]; duplicate {
			continue
		}
		seen[domain] = struct{}{}
		out = append(out, domain)
	}
	return out, nil
}

func normalizeSingboxCIDRs(values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if strings.HasPrefix(strings.ToLower(value), "geoip:") {
			return nil, fmt.Errorf("geoip: entries are not supported by the sing-box backend: %q", raw)
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			if addr, addrErr := netip.ParseAddr(value); addrErr == nil {
				prefix = netip.PrefixFrom(addr, addr.BitLen())
				err = nil
			}
		}
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", raw, err)
		}
		cidr := prefix.Masked().String()
		if _, duplicate := seen[cidr]; duplicate {
			continue
		}
		seen[cidr] = struct{}{}
		out = append(out, cidr)
	}
	return out, nil
}

func validateUDPUpstream(upstream string) error {
	if _, err := netip.ParseAddr(upstream); err == nil {
		return nil
	}
	domains, err := normalizeSingboxDomains([]string{upstream})
	if err != nil || len(domains) != 1 || domains[0] != strings.ToLower(upstream) {
		return fmt.Errorf("sing-box DNS routes: invalid UDP DNS upstream %q", upstream)
	}
	return nil
}

func managedTagPart(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	var b strings.Builder
	lastDash := false
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
