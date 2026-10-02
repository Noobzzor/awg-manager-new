package query

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hoaxisr/awg-manager/internal/ndms/cache"
)

// dnsProxyConfigTTL — short by design. The running-config rarely changes
// at runtime, but the DNS-encryption check is a one-shot user-triggered
// diagnostic, so we don't want a hot cache for an hour either.
const dnsProxyConfigTTL = 60 * time.Second

// DNSProxyConfigStore caches /show/rc/dns-proxy raw bytes. It exists so
// dnscheck can answer "is encrypted DNS configured?" without re-fetching
// the full running config on every diagnostic run.
type DNSProxyConfigStore struct {
	*cache.ListStore[[]byte]
	getter Getter
}

func NewDNSProxyConfigStore(g Getter, log Logger) *DNSProxyConfigStore {
	return NewDNSProxyConfigStoreWithTTL(g, log, dnsProxyConfigTTL)
}

func NewDNSProxyConfigStoreWithTTL(g Getter, log Logger, ttl time.Duration) *DNSProxyConfigStore {
	s := &DNSProxyConfigStore{getter: g}
	s.ListStore = cache.NewListStore(ttl, log, "dns-proxy-config", s.fetch)
	return s
}

// HasEncryptedTransport reports whether the cached dns-proxy config
// mentions any encrypted-transport keyword (DoT / DoH / TLS / HTTPS).
// Matches the legacy substring scan in dnscheck.checkEncryption.
func (s *DNSProxyConfigStore) HasEncryptedTransport(ctx context.Context) (bool, error) {
	raw, err := s.List(ctx)
	if err != nil {
		return false, err
	}
	lower := strings.ToLower(string(raw))
	for _, kw := range []string{"dot", "tls", "doh", "https"} {
		if strings.Contains(lower, kw) {
			return true, nil
		}
	}
	return false, nil
}

func (s *DNSProxyConfigStore) fetch(ctx context.Context) ([]byte, error) {
	raw, err := s.getter.GetRaw(ctx, "/show/rc/dns-proxy")
	if err != nil {
		return nil, fmt.Errorf("fetch dns-proxy config: %w", err)
	}
	out := make([]byte, len(raw))
	copy(out, raw)
	return out, nil
}
