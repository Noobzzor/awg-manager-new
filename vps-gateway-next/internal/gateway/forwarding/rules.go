package forwarding

import (
	"fmt"
	"net/netip"
	"regexp"
)

var linuxName = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,15}$`)

func RenderRuleset(clientPool netip.Prefix, wanInterface string, interfaceNames ...string) (string, error) {
	clientPool = clientPool.Masked()
	if !clientPool.IsValid() || !clientPool.Addr().Is4() || clientPool.Bits() > 30 {
		return "", fmt.Errorf("invalid client pool")
	}
	if !linuxName.MatchString(wanInterface) {
		return "", fmt.Errorf("invalid WAN interface")
	}
	ingressInterface := "awg0"
	policyInterface := "awgm0"
	if len(interfaceNames) > 0 && interfaceNames[0] != "" {
		ingressInterface = interfaceNames[0]
	}
	if len(interfaceNames) > 1 && interfaceNames[1] != "" {
		policyInterface = interfaceNames[1]
	}
	if !linuxName.MatchString(ingressInterface) || !linuxName.MatchString(policyInterface) || ingressInterface == policyInterface || ingressInterface == wanInterface || policyInterface == wanInterface {
		return "", fmt.Errorf("invalid gateway ingress or policy interface")
	}
	return fmt.Sprintf(`table inet awgm_gateway {
	 comment "AWGM_GATEWAY_OWNER"
	 chain input {
	  type filter hook input priority filter; policy accept;
	  iifname "%s" ip saddr %s meta l4proto tcp ct status dnat fib daddr type local accept
	  iifname "%s" drop
	 }
	 chain forward {
	  type filter hook forward priority filter; policy drop;
	  iifname "%s" ip saddr != %s drop
	  iifname "%s" ip saddr %s ip daddr %s drop
	  iifname "%s" ip saddr %s oifname "%s" accept
	  iifname "%s" drop
	  iifname "%s" ip daddr %s oifname "%s" ct state established,related accept
	  iifname "%s" oifname "%s" ct state established,related accept
	 }
	 chain postrouting {
	  type nat hook postrouting priority srcnat; policy accept;
	 }
}
`, ingressInterface, clientPool, ingressInterface, ingressInterface, clientPool, ingressInterface, clientPool, clientPool, ingressInterface, clientPool, policyInterface, ingressInterface, policyInterface, clientPool, ingressInterface, ingressInterface, policyInterface), nil
}
