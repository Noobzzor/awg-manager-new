package subscription

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidSubscription = errors.New("invalid client subscription")
	ErrNotFound            = errors.New("client subscription not found")
	ErrUnavailable         = errors.New("client subscription unavailable")
)

type Metadata struct {
	ID        string     `json:"id"`
	ClientID  string     `json:"clientId"`
	CreatedAt time.Time  `json:"createdAt"`
	ExpiresAt time.Time  `json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
}

type Issued struct {
	Subscription Metadata `json:"subscription"`
	Token        string   `json:"token"`
}

type StoredSubscription struct {
	Metadata
	TokenHash string `json:"tokenHash"`
}

type Store interface {
	Load() ([]StoredSubscription, error)
	Save([]StoredSubscription) error
}

type Manager struct {
	mu      sync.RWMutex
	store   Store
	records []StoredSubscription
	now     func() time.Time
}

func NewManager(store Store) (*Manager, error) {
	if store == nil {
		return nil, fmt.Errorf("subscription store is not configured")
	}
	records, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("load client subscriptions: %w", err)
	}
	manager := &Manager{store: store, now: time.Now, records: make([]StoredSubscription, 0, len(records))}
	seenIDs := make(map[string]struct{}, len(records))
	seenHashes := make(map[string]struct{}, len(records))
	for _, record := range records {
		if err := validateStored(record); err != nil {
			return nil, err
		}
		if _, ok := seenIDs[record.ID]; ok {
			return nil, fmt.Errorf("duplicate client subscription id")
		}
		if _, ok := seenHashes[record.TokenHash]; ok {
			return nil, fmt.Errorf("duplicate client subscription token hash")
		}
		seenIDs[record.ID] = struct{}{}
		seenHashes[record.TokenHash] = struct{}{}
		manager.records = append(manager.records, cloneStored(record))
	}
	return manager, nil
}

func (m *Manager) Create(clientID string, expiresAt time.Time) (Issued, error) {
	if m == nil || strings.TrimSpace(clientID) == "" {
		return Issued{}, ErrInvalidSubscription
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now().UTC()
	if !expiresAt.After(now) {
		return Issued{}, ErrInvalidSubscription
	}
	for attempt := 0; attempt < 4; attempt++ {
		idBytes := make([]byte, 16)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(idBytes); err != nil {
			return Issued{}, fmt.Errorf("generate subscription id: %w", err)
		}
		if _, err := rand.Read(tokenBytes); err != nil {
			return Issued{}, fmt.Errorf("generate subscription token: %w", err)
		}
		id := hex.EncodeToString(idBytes)
		token := base64.RawURLEncoding.EncodeToString(tokenBytes)
		hash := sha256.Sum256([]byte(token))
		record := StoredSubscription{
			Metadata:  Metadata{ID: id, ClientID: clientID, CreatedAt: now, ExpiresAt: expiresAt.UTC()},
			TokenHash: hex.EncodeToString(hash[:]),
		}
		if m.containsID(id) || m.containsHash(record.TokenHash) {
			continue
		}
		candidate := cloneRecords(m.records)
		candidate = append(candidate, record)
		if err := m.store.Save(candidate); err != nil {
			return Issued{}, fmt.Errorf("persist client subscription: %w", err)
		}
		m.records = candidate
		return Issued{Subscription: cloneMetadata(record.Metadata), Token: token}, nil
	}
	return Issued{}, fmt.Errorf("generate unique client subscription: %w", ErrInvalidSubscription)
}

func (m *Manager) Resolve(token string) (Metadata, error) {
	if m == nil || !validTokenFormat(token) {
		return Metadata{}, ErrUnavailable
	}
	hash := sha256.Sum256([]byte(token))
	hashText := hex.EncodeToString(hash[:])
	now := m.now().UTC()
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, record := range m.records {
		if subtle.ConstantTimeCompare([]byte(record.TokenHash), []byte(hashText)) != 1 {
			continue
		}
		if record.RevokedAt != nil || !record.ExpiresAt.After(now) {
			return Metadata{}, ErrUnavailable
		}
		return cloneMetadata(record.Metadata), nil
	}
	return Metadata{}, ErrUnavailable
}

func (m *Manager) Revoke(id string) error {
	if m == nil || strings.TrimSpace(id) == "" {
		return ErrNotFound
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	index := -1
	for i := range m.records {
		if m.records[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return ErrNotFound
	}
	if m.records[index].RevokedAt != nil {
		return nil
	}
	candidate := cloneRecords(m.records)
	now := m.now().UTC()
	candidate[index].RevokedAt = &now
	if err := m.store.Save(candidate); err != nil {
		return fmt.Errorf("persist client subscription revocation: %w", err)
	}
	m.records = candidate
	return nil
}

func (m *Manager) Get(id string) (Metadata, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, record := range m.records {
		if record.ID == id {
			return cloneMetadata(record.Metadata), true
		}
	}
	return Metadata{}, false
}

func (m *Manager) List() []Metadata {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Metadata, 0, len(m.records))
	for _, record := range m.records {
		out = append(out, cloneMetadata(record.Metadata))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func validTokenFormat(token string) bool {
	if len(token) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	return err == nil && len(decoded) == 32
}

func validateStored(record StoredSubscription) error {
	if strings.TrimSpace(record.ID) == "" || strings.TrimSpace(record.ClientID) == "" || record.CreatedAt.IsZero() || !record.ExpiresAt.After(record.CreatedAt) {
		return ErrInvalidSubscription
	}
	hash, err := hex.DecodeString(record.TokenHash)
	if err != nil || len(hash) != sha256.Size {
		return ErrInvalidSubscription
	}
	if record.RevokedAt != nil && record.RevokedAt.IsZero() {
		return ErrInvalidSubscription
	}
	return nil
}

func cloneRecords(records []StoredSubscription) []StoredSubscription {
	out := make([]StoredSubscription, len(records))
	for i, record := range records {
		out[i] = cloneStored(record)
	}
	return out
}

func cloneStored(record StoredSubscription) StoredSubscription {
	record.Metadata = cloneMetadata(record.Metadata)
	return record
}

func cloneMetadata(metadata Metadata) Metadata {
	if metadata.RevokedAt != nil {
		revokedAt := *metadata.RevokedAt
		metadata.RevokedAt = &revokedAt
	}
	return metadata
}

func (m *Manager) containsID(id string) bool {
	for _, record := range m.records {
		if record.ID == id {
			return true
		}
	}
	return false
}

func (m *Manager) containsHash(hash string) bool {
	for _, record := range m.records {
		if record.TokenHash == hash {
			return true
		}
	}
	return false
}
