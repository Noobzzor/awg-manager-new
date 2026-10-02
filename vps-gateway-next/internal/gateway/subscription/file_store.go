package subscription

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fileStoreVersion = 1

type fileStorePayload struct {
	Version       int                  `json:"version"`
	Subscriptions []StoredSubscription `json:"subscriptions"`
}

type FileStore struct {
	path string
}

func NewFileStore(path string) (*FileStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("subscription store path is empty")
	}
	return &FileStore{path: path}, nil
}

func (s *FileStore) Load() ([]StoredSubscription, error) {
	encoded, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []StoredSubscription{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read client subscription store: %w", err)
	}
	var payload fileStorePayload
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, fmt.Errorf("decode client subscription store: %w", err)
	}
	if payload.Version != fileStoreVersion {
		return nil, fmt.Errorf("unsupported client subscription store version %d", payload.Version)
	}
	if payload.Subscriptions == nil {
		payload.Subscriptions = []StoredSubscription{}
	}
	return payload.Subscriptions, nil
}

func (s *FileStore) Save(records []StoredSubscription) error {
	encoded, err := json.MarshalIndent(fileStorePayload{Version: fileStoreVersion, Subscriptions: records}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode client subscription store: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create client subscription store directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".gateway-subscriptions-*.tmp")
	if err != nil {
		return fmt.Errorf("create client subscription store temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("protect client subscription store temp: %w", err)
	}
	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write client subscription store: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync client subscription store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close client subscription store: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("commit client subscription store: %w", err)
	}
	return nil
}
