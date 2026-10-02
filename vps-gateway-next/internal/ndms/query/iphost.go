package query

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hoaxisr/awg-manager/internal/ndms/cache"
)

// IPHostEntry — one row from /show/rc/ip/host (`ip host <domain> <address>`).
type IPHostEntry struct {
	Domain  string `json:"domain"`
	Address string `json:"address"`
}

const ipHostTTL = 10 * time.Second

// IPHostStore caches /show/rc/ip/host. Writers (ip host POSTs) MUST call
// Invalidate to drop the cache before the next read so a fresh entry is
// visible immediately instead of waiting for TTL.
type IPHostStore struct {
	*cache.ListStore[[]IPHostEntry]
	getter Getter
}

func NewIPHostStore(g Getter, log Logger) *IPHostStore {
	return NewIPHostStoreWithTTL(g, log, ipHostTTL)
}

func NewIPHostStoreWithTTL(g Getter, log Logger, ttl time.Duration) *IPHostStore {
	s := &IPHostStore{getter: g}
	s.ListStore = cache.NewListStore(ttl, log, "ip-host", s.fetch)
	return s
}

// Invalidate drops the cached list. Call after a successful POST that
// adds/removes an ip-host entry so the next List reflects the write.
func (s *IPHostStore) Invalidate() { s.InvalidateAll() }

// Lookup returns the address for a given domain, or ("", false) if absent.
// Errors are swallowed: a transient NDMS hiccup is indistinguishable from
// a missing entry at this layer, and callers retry via createIPHost either
// way (matches the pre-cache semantics in dnscheck.lookupIPHost).
func (s *IPHostStore) Lookup(ctx context.Context, domain string) (string, bool) {
	entries, err := s.List(ctx)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.Domain == domain {
			return e.Address, true
		}
	}
	return "", false
}

func (s *IPHostStore) fetch(ctx context.Context) ([]IPHostEntry, error) {
	raw, err := s.getter.GetRaw(ctx, "/show/rc/ip/host")
	if err != nil {
		return nil, fmt.Errorf("fetch ip-host: %w", err)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, nil
	}
	var entries []IPHostEntry
	if err := json.Unmarshal(trimmed, &entries); err != nil {
		return nil, fmt.Errorf("decode ip-host: %w", err)
	}
	return entries, nil
}
