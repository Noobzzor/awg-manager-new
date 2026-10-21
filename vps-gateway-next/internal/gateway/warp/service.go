package warp

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"github.com/hoaxisr/awg-manager/internal/gateway/policy"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"sync"
)

type Orchestrator interface {
	SaveAndValidate(orchestrator.Slot, []byte) (orchestrator.ValidationResult, error)
}
type Service struct {
	mu     sync.Mutex
	store  *Store
	orch   Orchestrator
	synced bool
	hash   [32]byte
}

func NewService(store *Store, orch Orchestrator) *Service { return &Service{store: store, orch: orch} }
func profileHash(p *Profile) [32]byte {
	if p == nil {
		return [32]byte{}
	}
	raw, _ := json.Marshal(p)
	return sha256.Sum256(raw)
}
func (s *Service) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.store == nil || s.orch == nil {
		return errors.New("WARP provider dependencies unavailable")
	}
	p, err := s.store.Load()
	if err != nil {
		return err
	}
	raw := []byte(`{"endpoints": []}`)
	if p != nil {
		raw, err = RenderEndpoint(*p)
		if err != nil {
			return err
		}
	}
	result, err := s.orch.SaveAndValidate(orchestrator.SlotWARP, raw)
	if err != nil || !result.Ok() {
		return errors.New("WARP endpoint validation failed")
	}
	s.hash = profileHash(p)
	s.synced = true
	return nil
}

// WARPConfig exposes only a profile that matches the last validated owned slot.
// It is a configuration projection, not a network health or Cloudflare proof.
func (s *Service) WARPConfig() (*policy.WARPConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.synced {
		return nil, errors.New("WARP provider is not synchronized")
	}
	p, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	if profileHash(p) != s.hash {
		return nil, errors.New("WARP profile changed; synchronization required")
	}
	if p == nil {
		return nil, nil
	}
	return &policy.WARPConfig{Tag: Tag, Endpoint: p.Endpoint, MTU: p.MTU, IPv4: p.IPv4}, nil
}
