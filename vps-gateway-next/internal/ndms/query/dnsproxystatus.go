package query

import (
	"context"
	"fmt"
	"time"

	"github.com/hoaxisr/awg-manager/internal/ndms/cache"
)

// dnsProxyStatusTTL — short: this backs a manual-refresh diagnostics tab,
// so coalesce rapid clicks but never serve a stale hot cache.
const dnsProxyStatusTTL = 5 * time.Second

// DNSProxyStatusStore caches raw /show/dns-proxy bytes (the running ndnproxy
// status: upstreams, per-server stats, static records, rebind config). Unlike
// DNSProxyStore (/show/sc/dns-proxy/route) this endpoint exists on all OS
// versions, so there is no OS5 gate.
type DNSProxyStatusStore struct {
	*cache.ListStore[[]byte]
	getter Getter
}

func NewDNSProxyStatusStore(g Getter, log Logger) *DNSProxyStatusStore {
	return NewDNSProxyStatusStoreWithTTL(g, log, dnsProxyStatusTTL)
}

func NewDNSProxyStatusStoreWithTTL(g Getter, log Logger, ttl time.Duration) *DNSProxyStatusStore {
	s := &DNSProxyStatusStore{getter: g}
	s.ListStore = cache.NewListStore(ttl, log, "dns-proxy-status", s.fetch)
	return s
}

func (s *DNSProxyStatusStore) fetch(ctx context.Context) ([]byte, error) {
	raw, err := s.getter.GetRaw(ctx, "/show/dns-proxy")
	if err != nil {
		return nil, fmt.Errorf("fetch dns-proxy status: %w", err)
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out, nil
}
