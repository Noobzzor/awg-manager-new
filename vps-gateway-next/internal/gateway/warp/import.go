package warp

import (
	"encoding/json"
	"errors"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/storage"
	"os"
	"path/filepath"
)

var ErrRollbackFailed = errors.New("WARP profile rollback failed; resynchronization required")

// saveAndApply persists before the slot commit. On ordinary apply failure the
// exact prior encrypted envelope is restored. A crash may leave an unacknowledged
// candidate; startup Sync reconciles it. apply must not reenter this Store.
func (s *Store) saveAndApply(p Profile, apply func() error) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if apply == nil {
		return errors.New("WARP apply callback unavailable")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	previous, err := s.load()
	if err != nil {
		return err
	}
	var prior []byte
	if previous != nil {
		f, e := os.Open(s.path)
		if e != nil {
			return e
		}
		prior, e = readBounded(f)
		_ = f.Close()
		if e != nil {
			return e
		}
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return errors.New("cannot encode WARP profile")
	}
	token, err := s.cipher.Encrypt(string(raw))
	if err != nil {
		return errors.New("cannot encrypt WARP profile")
	}
	envelope, err := json.Marshal(storedProfile{Version: 1, Ciphertext: token})
	if err != nil {
		return errors.New("cannot encode WARP storage")
	}
	if err = s.writeAtomic(s.path, envelope, storage.SecretFilePermission); err != nil {
		return err
	}
	if err = apply(); err == nil {
		return nil
	}
	var rollback error
	if previous == nil {
		rollback = os.Remove(s.path)
	} else {
		rollback = s.writeAtomic(s.path, prior, storage.SecretFilePermission)
	}
	if rollback == nil {
		if s.syncDirectory == nil {
			rollback = ErrRollbackFailed
		} else {
			rollback = s.syncDirectory(filepath.Dir(s.path))
		}
	}
	if rollback != nil {
		return ErrRollbackFailed
	}
	return errors.New("WARP endpoint apply failed; prior profile restored")
}

// Import commits configuration, not connectivity or a synchronous engine reload.
// Only the owned, synchronized installation is mutable; concurrent projections
// cannot observe its intermediate persisted candidate.
func (s *Service) Import(p Profile) error {
	raw, err := RenderEndpoint(p)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.synced || s.store == nil || s.orch == nil {
		return errors.New("WARP provider is not synchronized")
	}
	previous, err := s.store.Load()
	if err != nil {
		return err
	}
	if profileHash(previous) != s.hash {
		return errors.New("WARP profile changed; synchronization required")
	}
	checker, ok := s.orch.(interface {
		CheckMerged(orchestrator.Slot, []byte) (orchestrator.ValidationResult, error)
	})
	if !ok {
		return errors.New("WARP validation unavailable")
	}
	result, err := checker.CheckMerged(orchestrator.SlotWARP, raw)
	if err != nil || !result.Ok() {
		return errors.New("WARP endpoint validation failed")
	}
	err = s.store.saveAndApply(p, func() error {
		result, e := s.orch.SaveAndValidate(orchestrator.SlotWARP, raw)
		if e != nil || !result.Ok() {
			return errors.New("WARP endpoint apply failed")
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrRollbackFailed) {
			s.synced = false
		}
		return err
	}
	s.hash = profileHash(&p)
	s.synced = true
	return nil
}

func (s *Service) Describe() (Description, error) {
	cfg, err := s.WARPConfig()
	if err != nil {
		return Description{}, err
	}
	if cfg == nil {
		return Description{}, nil
	}
	return Description{Configured: true, Tag: cfg.Tag, Endpoint: cfg.Endpoint, IPv4: cfg.IPv4, MTU: cfg.MTU}, nil
}

// syncProfileDirectory makes a rollback rename/unlink durable before success.
// Unlike the shared best-effort writer, failure is reported to the provider.
func syncProfileDirectory(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
