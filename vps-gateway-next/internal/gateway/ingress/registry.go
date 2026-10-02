package ingress

import (
	"errors"
	"fmt"
	"sync"
)

type KeyGenerator func(clientID string) (privateKey, publicKey string, err error)

var ErrClientExists = errors.New("ingress client already exists")

// Registry owns the gateway client lifecycle and keeps allocator state in sync.
type Registry struct {
	mu        sync.RWMutex
	allocator *Allocator
	keygen    KeyGenerator
	clients   map[string]*Client
}

func NewRegistry(allocator *Allocator, keygen KeyGenerator) *Registry {
	return &Registry{allocator: allocator, keygen: keygen, clients: make(map[string]*Client)}
}

func (r *Registry) Create(id, label string) (*Client, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.clients[id]; exists {
		return nil, ErrClientExists
	}
	if r.allocator == nil || r.keygen == nil {
		return nil, fmt.Errorf("registry dependencies are not configured")
	}
	address, err := r.allocator.Allocate(id)
	if err != nil {
		return nil, err
	}
	privateKey, publicKey, err := r.keygen(id)
	if err != nil {
		_ = r.allocator.Release(id)
		return nil, err
	}
	client, err := NewClient(id, label, address, privateKey, publicKey)
	if err != nil {
		_ = r.allocator.Release(id)
		return nil, err
	}
	r.clients[id] = client
	return cloneClient(client), nil
}

func (r *Registry) Get(id string) (*Client, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	client, ok := r.clients[id]
	if !ok {
		return nil, false
	}
	return cloneClient(client), true
}

func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.clients[id]; !ok {
		return ErrClientNotFound
	}
	if r.allocator == nil {
		return fmt.Errorf("registry allocator is not configured")
	}
	if err := r.allocator.Release(id); err != nil {
		return err
	}
	delete(r.clients, id)
	return nil
}

func (r *Registry) List() []ClientPublic {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ClientPublic, 0, len(r.clients))
	for _, client := range r.clients {
		out = append(out, client.Public())
	}
	return out
}

func (r *Registry) Disable(id string) error { return r.transition(id, func(c *Client) { c.Disable() }) }

func (r *Registry) Revoke(id string) error { return r.transition(id, func(c *Client) { c.Revoke() }) }

func (r *Registry) Enable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	client, ok := r.clients[id]
	if !ok {
		return ErrClientNotFound
	}
	return client.Enable()
}

func (r *Registry) transition(id string, fn func(*Client)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	client, ok := r.clients[id]
	if !ok {
		return ErrClientNotFound
	}
	fn(client)
	return nil
}

func cloneClient(src *Client) *Client {
	copy := *src
	return &copy
}
