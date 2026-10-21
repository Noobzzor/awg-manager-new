package ingress

import (
	"fmt"
	"net/netip"
	"strings"
)

// ParseClientDNS validates the DNS destination advertised to IPv4-only clients.
// Gateway DNS interception still owns resolution and its upstream configuration.
func ParseClientDNS(raw string) (netip.Addr, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "1.1.1.1"
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil || !addr.Is4() || !addr.IsGlobalUnicast() || addr.IsLoopback() {
		return netip.Addr{}, fmt.Errorf("client DNS must be a usable IPv4 address")
	}
	return addr, nil
}

// UseClientDNS is startup-only, like UseAWGParameters. It changes no client
// identity or persistence format and cannot replace a configured live value.
func (r *Registry) UseClientDNS(addr netip.Addr) error {
	if r == nil {
		return fmt.Errorf("client registry is not configured")
	}
	if _, err := ParseClientDNS(addr.String()); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.clientDNS.IsValid() && r.clientDNS != addr {
		return fmt.Errorf("client DNS cannot change while registry is live")
	}
	r.clientDNS = addr
	return nil
}
