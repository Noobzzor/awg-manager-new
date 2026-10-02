package groups

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fileStoreVersion = 1

type fileStorePayload struct {
	Version int     `json:"version"`
	Groups  []Group `json:"groups"`
}

type FileStore struct {
	path string
}

func NewFileStore(path string) (*FileStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("client group store path is empty")
	}
	return &FileStore{path: path}, nil
}

func (s *FileStore) Load() ([]Group, error) {
	encoded, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []Group{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read client group store: %w", err)
	}
	var payload fileStorePayload
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, fmt.Errorf("decode client group store: %w", err)
	}
	if payload.Version != fileStoreVersion {
		return nil, fmt.Errorf("unsupported client group store version %d", payload.Version)
	}
	if payload.Groups == nil {
		payload.Groups = []Group{}
	}
	return cloneGroups(payload.Groups), nil
}

func (s *FileStore) Save(groups []Group) error {
	encoded, err := json.MarshalIndent(fileStorePayload{Version: fileStoreVersion, Groups: cloneGroups(groups)}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode client group store: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create client group store directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".gateway-groups-*.tmp")
	if err != nil {
		return fmt.Errorf("create client group store temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("protect client group store temp: %w", err)
	}
	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write client group store: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync client group store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close client group store: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("commit client group store: %w", err)
	}
	return nil
}
