package ingress

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"
)

var (
	ErrInvalidPool    = errors.New("invalid client address pool")
	ErrPoolExhausted  = errors.New("client address pool exhausted")
	ErrClientNotFound = errors.New("client allocation not found")
)

// Allocator deterministically assigns usable IPv4 addresses to client IDs.
// The network and broadcast addresses are never returned.
type Allocator struct {
	mu     sync.Mutex
	prefix netip.Prefix
	byID   map[string]netip.Addr
	byAddr map[netip.Addr]string
}

const reservedAddressOwner = "__gateway_reserved__"

func NewAllocator(prefix netip.Prefix) (*Allocator, error) {
	prefix = prefix.Masked()
	if !prefix.IsValid() || !prefix.Addr().Is4() || prefix.Bits() > 30 {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPool, prefix)
	}
	return &Allocator{prefix: prefix, byID: make(map[string]netip.Addr), byAddr: make(map[netip.Addr]string)}, nil
}

func (a *Allocator) Allocate(clientID string) (netip.Addr, error) {
	if clientID == "" {
		return netip.Addr{}, fmt.Errorf("client id is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if addr, ok := a.byID[clientID]; ok {
		return addr, nil
	}
	for addr := a.prefix.Addr().Next(); a.prefix.Contains(addr) && a.prefix.Contains(addr.Next()); addr = addr.Next() {
		if _, used := a.byAddr[addr]; used {
			continue
		}
		a.byID[clientID] = addr
		a.byAddr[addr] = clientID
		return addr, nil
	}
	return netip.Addr{}, ErrPoolExhausted
}

func (a *Allocator) ReserveAddress(addr netip.Addr) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !addr.IsValid() || !a.prefix.Contains(addr) || !a.prefix.Contains(addr.Next()) || addr == a.prefix.Addr() {
		return fmt.Errorf("%w: reserved address %s is not usable", ErrInvalidPool, addr)
	}
	if owner, exists := a.byAddr[addr]; exists && owner != reservedAddressOwner {
		return fmt.Errorf("address %s already allocated to %s", addr, owner)
	}
	a.byAddr[addr] = reservedAddressOwner
	return nil
}

func (a *Allocator) Release(clientID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	addr, ok := a.byID[clientID]
	if !ok {
		return ErrClientNotFound
	}
	delete(a.byID, clientID)
	delete(a.byAddr, addr)
	return nil
}

func (a *Allocator) Lookup(clientID string) (netip.Addr, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	addr, ok := a.byID[clientID]
	return addr, ok
}

func (a *Allocator) reserve(clientID string, addr netip.Addr) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.prefix.Contains(addr) || !a.prefix.Contains(addr.Next()) || addr == a.prefix.Addr() {
		return fmt.Errorf("%w: address %s is not usable", ErrInvalidPool, addr)
	}
	if _, exists := a.byID[clientID]; exists {
		return ErrClientExists
	}
	if owner, exists := a.byAddr[addr]; exists {
		return fmt.Errorf("address %s already allocated to %s", addr, owner)
	}
	a.byID[clientID] = addr
	a.byAddr[addr] = clientID
	return nil
}
