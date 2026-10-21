package warp

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

type storedProfile struct {
	Version    int    `json:"version"`
	Ciphertext string `json:"ciphertext"`
}

// Store serializes one installation's profile access within this instance.
// Other installation secret stores retain their existing coordination rules.
type Store struct {
	mu            sync.Mutex
	path          string
	cipher        *storage.DeviceCipher
	writeAtomic   func(string, []byte, os.FileMode) error
	syncDirectory func(string) error
}

func NewStore(dataDir string) *Store {
	return &Store{path: filepath.Join(dataDir, "gateway-warp.json"), cipher: storage.NewDeviceCipher(dataDir), writeAtomic: storage.AtomicWritePerm, syncDirectory: syncProfileDirectory}
}

func (s *Store) Save(p Profile) error {
	if err := p.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// Do not let Encrypt's new-key path overwrite a recoverable existing profile
	// after key loss/corruption. Reads neither replace nor create device keys.
	if _, err := s.load(); err != nil {
		return err
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return errors.New("cannot encode WARP profile")
	}
	token, err := s.cipher.Encrypt(string(raw))
	if err != nil {
		return fmt.Errorf("encrypt WARP profile: %w", err)
	}
	envelope, err := json.Marshal(storedProfile{Version: 1, Ciphertext: token})
	if err != nil {
		return errors.New("cannot encode WARP storage")
	}
	if err := s.writeAtomic(s.path, envelope, storage.SecretFilePermission); err != nil {
		return fmt.Errorf("save WARP profile: %w", err)
	}
	return nil
}

func (s *Store) Load() (*Profile, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.load() }

func (s *Store) load() (*Profile, error) {
	file, err := os.Open(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open WARP profile: %w", err)
	}
	defer file.Close()
	raw, err := readBounded(file)
	if err != nil {
		return nil, err
	}
	var envelope storedProfile
	if err := decodeStrict(raw, &envelope); err != nil || envelope.Version != 1 || envelope.Ciphertext == "" {
		return nil, errors.New("invalid encrypted WARP profile")
	}
	plaintext, err := s.cipher.Decrypt(envelope.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("decrypt WARP profile: %w", err)
	}
	p, err := DecodeProfile(strings.NewReader(plaintext))
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) Describe() (Description, error) {
	p, err := s.Load()
	if err != nil {
		return Description{}, err
	}
	if p == nil {
		return Description{}, nil
	}
	return p.Description(), nil
}
