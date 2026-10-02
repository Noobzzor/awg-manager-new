package forwarding

import (
	"fmt"
	"net/netip"
	"strings"
)

type Policy struct {
	DNSUpstream netip.Addr
	MTU         int
}

func NewPolicy(dnsUpstream netip.Addr, mtu int) (Policy, error) {
	if !dnsUpstream.IsValid() || !dnsUpstream.Is4() {
		return Policy{}, fmt.Errorf("DNS upstream must be IPv4")
	}
	if mtu < 1280 || mtu > 1500 {
		return Policy{}, fmt.Errorf("MTU must be between 1280 and 1500")
	}
	return Policy{DNSUpstream: dnsUpstream, MTU: mtu}, nil
}

func RenderPolicyRuleset(policy Policy, clientPool netip.Prefix, wanInterface string) (string, error) {
	if _, err := NewPolicy(policy.DNSUpstream, policy.MTU); err != nil {
		return "", err
	}
	base, err := RenderRuleset(clientPool, wanInterface)
	if err != nil {
		return "", err
	}
	mss := policy.MTU - 40
	fragment := fmt.Sprintf("\t  iifname \"awg0\" ip daddr %s udp dport 53 accept\n\t  iifname \"awg0\" ip daddr %s tcp dport 53 accept\n\t  iifname \"awg0\" tcp flags syn tcp option maxseg size set %d\n", policy.DNSUpstream, policy.DNSUpstream, mss)
	return strings.Replace(base, "\t chain postrouting {", fragment+"\t chain postrouting {", 1), nil
}
