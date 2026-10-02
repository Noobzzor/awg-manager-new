package groups

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidGroup   = errors.New("invalid client group")
	ErrGroupNotFound  = errors.New("client group not found")
	ErrClientNotFound = errors.New("group client not found")
)

type Group struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ClientIDs []string  `json:"clientIds"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Store interface {
	Load() ([]Group, error)
	Save([]Group) error
}

type ClientExists func(string) bool

type Manager struct {
	mu           sync.RWMutex
	store        Store
	clientExists ClientExists
	groups       map[string]Group
	now          func() time.Time
}

func NewManager(store Store, clientExists ClientExists) (*Manager, error) {
	if store == nil || clientExists == nil {
		return nil, fmt.Errorf("group manager dependencies are not configured")
	}
	persisted, err := store.Load()
	if err != nil {
		return nil, fmt.Errorf("load client groups: %w", err)
	}
	manager := &Manager{store: store, clientExists: clientExists, groups: make(map[string]Group), now: time.Now}
	for _, group := range persisted {
		if err := manager.validateGroup(group, group.ID); err != nil {
			return nil, fmt.Errorf("invalid persisted client group %q: %w", group.ID, err)
		}
		if _, exists := manager.groups[group.ID]; exists {
			return nil, fmt.Errorf("duplicate persisted client group id %q", group.ID)
		}
		manager.groups[group.ID] = cloneGroup(group)
	}
	return manager, nil
}

func (m *Manager) Create(name string, clientIDs []string) (Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, err := newID()
	if err != nil {
		return Group{}, err
	}
	group := Group{ID: id, Name: strings.TrimSpace(name), ClientIDs: normalizeClientIDs(clientIDs)}
	if err := m.validateGroup(group, ""); err != nil {
		return Group{}, err
	}
	now := m.now().UTC()
	group.CreatedAt, group.UpdatedAt = now, now
	candidate := cloneGroupMap(m.groups)
	candidate[group.ID] = group
	if err := m.persist(candidate); err != nil {
		return Group{}, err
	}
	m.groups = candidate
	return cloneGroup(group), nil
}

func (m *Manager) Update(id, name string, clientIDs []string) (Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current, exists := m.groups[id]
	if !exists {
		return Group{}, ErrGroupNotFound
	}
	updated := Group{
		ID:        current.ID,
		Name:      strings.TrimSpace(name),
		ClientIDs: normalizeClientIDs(clientIDs),
		CreatedAt: current.CreatedAt,
		UpdatedAt: m.now().UTC(),
	}
	if err := m.validateGroup(updated, id); err != nil {
		return Group{}, err
	}
	candidate := cloneGroupMap(m.groups)
	candidate[id] = updated
	if err := m.persist(candidate); err != nil {
		return Group{}, err
	}
	m.groups = candidate
	return cloneGroup(updated), nil
}

func (m *Manager) Get(id string) (Group, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	group, exists := m.groups[id]
	if !exists {
		return Group{}, false
	}
	return cloneGroup(group), true
}

func (m *Manager) List() []Group {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ids := make([]string, 0, len(m.groups))
	for id := range m.groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Group, 0, len(ids))
	for _, id := range ids {
		out = append(out, cloneGroup(m.groups[id]))
	}
	return out
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.groups[id]; !exists {
		return ErrGroupNotFound
	}
	candidate := cloneGroupMap(m.groups)
	delete(candidate, id)
	if err := m.persist(candidate); err != nil {
		return err
	}
	m.groups = candidate
	return nil
}

func (m *Manager) validateGroup(group Group, allowID string) error {
	if strings.TrimSpace(group.ID) == "" || strings.TrimSpace(group.Name) == "" || group.Name != strings.TrimSpace(group.Name) {
		return ErrInvalidGroup
	}
	if group.ID != allowID {
		if _, exists := m.groups[group.ID]; exists {
			return ErrInvalidGroup
		}
	}
	seen := make(map[string]struct{}, len(group.ClientIDs))
	for _, clientID := range group.ClientIDs {
		if clientID == "" || clientID != strings.TrimSpace(clientID) {
			return ErrInvalidGroup
		}
		if _, exists := seen[clientID]; exists {
			return ErrInvalidGroup
		}
		seen[clientID] = struct{}{}
		if !m.clientExists(clientID) {
			return fmt.Errorf("%w: %s", ErrClientNotFound, clientID)
		}
	}
	return nil
}

func (m *Manager) persist(candidate map[string]Group) error {
	ids := make([]string, 0, len(candidate))
	for id := range candidate {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	groups := make([]Group, 0, len(ids))
	for _, id := range ids {
		groups = append(groups, cloneGroup(candidate[id]))
	}
	return m.store.Save(groups)
}

func newID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate client group id: %w", err)
	}
	return "group-" + hex.EncodeToString(raw[:]), nil
}

func normalizeClientIDs(clientIDs []string) []string {
	out := make([]string, len(clientIDs))
	for index, clientID := range clientIDs {
		out[index] = strings.TrimSpace(clientID)
	}
	return out
}

func cloneGroup(group Group) Group {
	group.ClientIDs = cloneStrings(group.ClientIDs)
	return group
}

func cloneGroups(groups []Group) []Group {
	out := make([]Group, len(groups))
	for index, group := range groups {
		out[index] = cloneGroup(group)
	}
	return out
}

func cloneGroupMap(groups map[string]Group) map[string]Group {
	out := make(map[string]Group, len(groups)+1)
	for id, group := range groups {
		out[id] = cloneGroup(group)
	}
	return out
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}
