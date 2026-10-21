//go:build linux

package ingress

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var _ PeerApplier = (*UserspaceAWGInterface)(nil)
var _ PeerStatusReader = (*UserspaceAWGInterface)(nil)

func validOwnedPeerKey(key string) bool {
	decoded, err := base64.StdEncoding.DecodeString(key)
	return err == nil && len(decoded) == 32 && base64.StdEncoding.EncodeToString(decoded) == key && !bytes.Equal(decoded, make([]byte, 32))
}

func (m *UserspaceAWGInterface) peerReadyLocked() error {
	if err := m.ownedReadyLocked(); err != nil {
		return err
	}
	if !m.configured || m.run == nil {
		return fmt.Errorf("owned AWG server is not configured")
	}
	return nil
}

// Hold the lifecycle lock through each bounded command, so Close cannot remove
// the owned child/socket while a peer operation is using it.
func (m *UserspaceAWGInterface) ApplyPeer(ctx context.Context, peer PeerConfig) error {
	if m == nil {
		return fmt.Errorf("owned AWG peer controller unavailable")
	}
	if !validOwnedPeerKey(peer.PublicKey) || !peer.AllowedIP.IsValid() || !peer.AllowedIP.Addr().Is4() || peer.AllowedIP.Bits() != 32 || peer.presharedKey != "" {
		return fmt.Errorf("invalid owned AWG peer or unsupported preshared key")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.peerReadyLocked(); err != nil {
		return err
	}
	if _, err := m.run(ctx, m.config.ToolBinary, "set", m.Name(), "peer", peer.PublicKey, "allowed-ips", peer.AllowedIP.String()); err != nil {
		return fmt.Errorf("owned AWG peer apply failed")
	}
	return nil
}

func (m *UserspaceAWGInterface) RemovePeer(ctx context.Context, key string) error {
	if m == nil {
		return fmt.Errorf("owned AWG peer controller unavailable")
	}
	if !validOwnedPeerKey(key) {
		return fmt.Errorf("invalid owned AWG peer public key")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := m.peerReadyLocked(); err != nil {
		return err
	}
	if _, err := m.run(ctx, m.config.ToolBinary, "set", m.Name(), "peer", key, "remove"); err != nil {
		return fmt.Errorf("owned AWG peer remove failed")
	}
	return nil
}

func (m *UserspaceAWGInterface) publicPeerFieldLocked(ctx context.Context, key, field string, width int) ([]string, error) {
	if err := m.peerReadyLocked(); err != nil {
		return nil, err
	}
	if m.statusRun == nil {
		return nil, fmt.Errorf("owned AWG public status reader unavailable")
	}
	output, err := m.statusRun(ctx, m.config.ToolBinary, m.Name(), field, key)
	if err != nil {
		return nil, fmt.Errorf("owned AWG public peer status read failed")
	}
	var found []string
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != key {
			continue
		}
		if len(fields) != width || found != nil {
			return nil, fmt.Errorf("invalid owned AWG public peer status")
		}
		found = fields
	}
	return found, nil
}

// Read only public fields. Never request dump/private-key/preshared-keys.
// Three bounded reads are not an atomic statistics snapshot; lifecycle is
// serialized, while packet counters may advance normally between reads.
func (m *UserspaceAWGInterface) ReadPeerStatus(parent context.Context, key string) (PeerStatus, error) {
	if m == nil {
		return PeerStatus{}, fmt.Errorf("owned AWG peer controller unavailable")
	}
	if !validOwnedPeerKey(key) {
		return PeerStatus{}, fmt.Errorf("invalid owned AWG peer public key")
	}
	ctx, cancel := context.WithTimeout(parent, 6*time.Second)
	defer cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return PeerStatus{}, err
	}
	endpoint, err := m.publicPeerFieldLocked(ctx, key, "endpoints", 2)
	if err != nil {
		return PeerStatus{}, err
	}
	handshake, err := m.publicPeerFieldLocked(ctx, key, "latest-handshakes", 2)
	if err != nil {
		return PeerStatus{}, err
	}
	transfer, err := m.publicPeerFieldLocked(ctx, key, "transfer", 3)
	if err != nil {
		return PeerStatus{}, err
	}
	if endpoint == nil && handshake == nil && transfer == nil {
		return PeerStatus{}, ErrPeerNotFound
	}
	if endpoint == nil || handshake == nil || transfer == nil {
		return PeerStatus{}, fmt.Errorf("inconsistent owned AWG public peer status")
	}
	seconds, err := strconv.ParseInt(handshake[1], 10, 64)
	if err != nil || seconds < 0 {
		return PeerStatus{}, fmt.Errorf("invalid owned AWG handshake timestamp")
	}
	rx, err := strconv.ParseUint(transfer[1], 10, 64)
	if err != nil {
		return PeerStatus{}, fmt.Errorf("invalid owned AWG received byte count")
	}
	tx, err := strconv.ParseUint(transfer[2], 10, 64)
	if err != nil {
		return PeerStatus{}, fmt.Errorf("invalid owned AWG sent byte count")
	}
	status := PeerStatus{PublicKey: key, Endpoint: endpoint[1], RXBytes: rx, TXBytes: tx}
	if status.Endpoint == "(none)" {
		status.Endpoint = ""
	}
	if seconds > 0 {
		status.Handshaken = true
		status.LastHandshake = time.Unix(seconds, 0)
	}
	return status, nil
}
