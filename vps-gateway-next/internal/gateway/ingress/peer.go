package ingress

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
)

type PeerConfig struct {
	ClientID     string
	PublicKey    string
	AllowedIP    netip.Prefix
	presharedKey string
}

func BuildPeerConfig(client *Client, presharedKey string) (PeerConfig, error) {
	if client == nil || client.Status != StatusActive {
		return PeerConfig{}, fmt.Errorf("client is not active")
	}
	return PeerConfig{ClientID: client.ID, PublicKey: client.PublicKey, AllowedIP: netip.PrefixFrom(client.Address, 32), presharedKey: presharedKey}, nil
}

func (p PeerConfig) String() string {
	return fmt.Sprintf("peer client=%s publicKey=%s allowedIP=%s", p.ClientID, p.PublicKey, p.AllowedIP)
}

// PeerApplier is the narrow runtime boundary for an AWG/WireGuard server.
type PeerApplier interface {
	ApplyPeer(ctx context.Context, peer PeerConfig) error
	RemovePeer(ctx context.Context, publicKey string) error
}

func (r *Registry) SyncClient(ctx context.Context, id, presharedKey string, applier PeerApplier) error {
	if applier == nil {
		return fmt.Errorf("peer applier is nil")
	}
	client, ok := r.Get(id)
	if !ok {
		return ErrClientNotFound
	}
	if client.Status != StatusActive {
		return applier.RemovePeer(ctx, client.PublicKey)
	}
	full, ok := r.internal(id)
	if !ok {
		return ErrClientNotFound
	}
	peer, err := BuildPeerConfig(full, presharedKey)
	if err != nil {
		return err
	}
	return applier.ApplyPeer(ctx, peer)
}

// ReconcilePeers makes the live interface match the persisted registry.
// Active clients are applied; disabled and revoked clients are removed.
func (r *Registry) ReconcilePeers(ctx context.Context, applier PeerApplier) error {
	if applier == nil {
		return fmt.Errorf("peer applier is nil")
	}
	clients := r.List()
	ids := make([]string, 0, len(clients))
	for _, client := range clients {
		ids = append(ids, client.ID)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := r.SyncClient(ctx, id, "", applier); err != nil {
			return fmt.Errorf("reconcile gateway peer %s: %w", id, err)
		}
	}
	return nil
}

func (r *Registry) internal(id string) (*Client, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	client, ok := r.clients[id]
	if !ok {
		return nil, false
	}
	return cloneClient(client), true
}
