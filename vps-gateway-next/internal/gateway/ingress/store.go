package ingress

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"time"
)

const storeVersion = 1

type persistedClient struct {
	ID         string       `json:"id"`
	Label      string       `json:"label"`
	Address    string       `json:"address"`
	PublicKey  string       `json:"publicKey"`
	PrivateKey string       `json:"privateKey"`
	Status     ClientStatus `json:"status"`
	CreatedAt  int64        `json:"createdAt"`
	UpdatedAt  int64        `json:"updatedAt"`
}

type persistedStore struct {
	Version int               `json:"version"`
	Clients []persistedClient `json:"clients"`
}

func Save(r *Registry, path string) error {
	if r == nil {
		return fmt.Errorf("registry is nil")
	}
	r.mu.RLock()
	data := persistedStore{Version: storeVersion, Clients: make([]persistedClient, 0, len(r.clients))}
	for _, client := range r.clients {
		data.Clients = append(data.Clients, persistedClient{ID: client.ID, Label: client.Label, Address: client.Address.String(), PublicKey: client.PublicKey, PrivateKey: client.privateKey, Status: client.Status, CreatedAt: client.CreatedAt.UnixNano(), UpdatedAt: client.UpdatedAt.UnixNano()})
	}
	r.mu.RUnlock()
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ingress store: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create ingress store directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".clients-*.tmp")
	if err != nil {
		return fmt.Errorf("create ingress store temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("protect ingress store temp: %w", err)
	}
	if _, err := tmp.Write(encoded); err != nil {
		tmp.Close()
		return fmt.Errorf("write ingress store: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync ingress store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close ingress store: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("commit ingress store: %w", err)
	}
	return nil
}

func Load(path string, allocator *Allocator, keygen KeyGenerator) (*Registry, error) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read ingress store: %w", err)
	}
	var data persistedStore
	if err := json.Unmarshal(encoded, &data); err != nil {
		return nil, fmt.Errorf("decode ingress store: %w", err)
	}
	if data.Version != storeVersion {
		return nil, fmt.Errorf("unsupported ingress store version %d", data.Version)
	}
	registry := NewRegistry(allocator, keygen)
	for _, item := range data.Clients {
		address, err := netip.ParseAddr(item.Address)
		if err != nil {
			return nil, fmt.Errorf("client %s address: %w", item.ID, err)
		}
		client, err := NewClient(item.ID, item.Label, address, item.PrivateKey, item.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("client %s: %w", item.ID, err)
		}
		client.Status = item.Status
		client.CreatedAt = time.Unix(0, item.CreatedAt).UTC()
		client.UpdatedAt = time.Unix(0, item.UpdatedAt).UTC()
		if err := allocator.reserve(client.ID, client.Address); err != nil {
			return nil, fmt.Errorf("client %s allocation: %w", client.ID, err)
		}
		registry.clients[client.ID] = client
	}
	return registry, nil
}
