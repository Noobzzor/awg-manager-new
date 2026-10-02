package query

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hoaxisr/awg-manager/internal/ndms/cache"
)

// NATEntry is one row from /show/rc/ip/nat.
type NATEntry struct {
	Interface string `json:"interface"`
}

const natTTL = 30 * time.Second

// NATStore caches /show/rc/ip/nat.
type NATStore struct {
	*cache.ListStore[[]NATEntry]
	getter Getter
}

func NewNATStore(g Getter, log Logger) *NATStore {
	s := &NATStore{getter: g}
	s.ListStore = cache.NewListStore(natTTL, log, "ip-nat", s.fetch)
	return s
}

// HasInterface reports whether NAT is enabled for the given NDMS interface name.
func (s *NATStore) HasInterface(ctx context.Context, iface string) (bool, error) {
	entries, err := s.List(ctx)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if e.Interface == iface {
			return true, nil
		}
	}
	return false, nil
}

func (s *NATStore) fetch(ctx context.Context) ([]NATEntry, error) {
	raw, err := s.getter.GetRaw(ctx, "/show/rc/ip/nat")
	if err != nil {
		return nil, fmt.Errorf("fetch ip nat: %w", err)
	}
	if len(raw) == 0 {
		return nil, nil
	}
	var entries []NATEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		// Some firmware returns a single object instead of an array.
		var single NATEntry
		if err2 := json.Unmarshal(raw, &single); err2 != nil {
			return nil, fmt.Errorf("decode ip nat: %w", err)
		}
		if single.Interface != "" {
			return []NATEntry{single}, nil
		}
		return nil, nil
	}
	return entries, nil
}
