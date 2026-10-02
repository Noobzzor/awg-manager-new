package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fileStoreVersion = 2

const legacyFileStoreVersion = 1

type fileStorePayload struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

// Store persists validated policy profiles.
type Store interface {
	Save([]Profile) error
	Load() ([]Profile, error)
}

// FileStore stores policy profiles in a private, versioned JSON file.
type FileStore struct {
	path string
}

func NewFileStore(path string) (*FileStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("policy profile store path is empty")
	}
	return &FileStore{path: path}, nil
}

func (s *FileStore) Load() ([]Profile, error) {
	encoded, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []Profile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read policy profile store: %w", err)
	}
	var payload fileStorePayload
	if err := json.Unmarshal(encoded, &payload); err != nil {
		return nil, fmt.Errorf("decode policy profile store: %w", err)
	}
	if payload.Version == legacyFileStoreVersion {
		// Version 1 persisted Enabled before the compiler honored it. Preserve
		// the previously active behavior when upgrading those existing rules.
		for i := range payload.Profiles {
			for j := range payload.Profiles[i].Rules {
				payload.Profiles[i].Rules[j].Enabled = true
			}
		}
		if payload.Profiles == nil {
			payload.Profiles = []Profile{}
		}
		if err := s.Save(payload.Profiles); err != nil {
			return nil, fmt.Errorf("migrate policy profile store: %w", err)
		}
		return cloneProfiles(payload.Profiles), nil
	}
	if payload.Version != fileStoreVersion {
		return nil, fmt.Errorf("unsupported policy profile store version %d", payload.Version)
	}
	if payload.Profiles == nil {
		payload.Profiles = []Profile{}
	}
	if err := validateProfiles(payload.Profiles); err != nil {
		return nil, fmt.Errorf("validate policy profile store: %w", err)
	}
	return cloneProfiles(payload.Profiles), nil
}

func (s *FileStore) Save(profiles []Profile) error {
	if err := validateProfiles(profiles); err != nil {
		return fmt.Errorf("validate policy profiles: %w", err)
	}
	encoded, err := json.MarshalIndent(fileStorePayload{Version: fileStoreVersion, Profiles: cloneProfiles(profiles)}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode policy profile store: %w", err)
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create policy profile store directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".gateway-policy-*.tmp")
	if err != nil {
		return fmt.Errorf("create policy profile store temp: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("protect policy profile store temp: %w", err)
	}
	if _, err := tmp.Write(encoded); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write policy profile store: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync policy profile store: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close policy profile store: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("commit policy profile store: %w", err)
	}
	return nil
}

func validateProfiles(profiles []Profile) error {
	seen := make(map[string]struct{}, len(profiles))
	for _, profile := range profiles {
		if err := profile.Validate(); err != nil {
			return fmt.Errorf("profile %q: %w", profile.ID, err)
		}
		if _, ok := seen[profile.ID]; ok {
			return fmt.Errorf("duplicate profile id %q", profile.ID)
		}
		seen[profile.ID] = struct{}{}
	}
	return nil
}

func cloneProfiles(profiles []Profile) []Profile {
	cloned := make([]Profile, len(profiles))
	for i, profile := range profiles {
		cloned[i] = profile
		cloned[i].Rules = make([]Rule, len(profile.Rules))
		for j, rule := range profile.Rules {
			cloned[i].Rules[j] = rule
			cloned[i].Rules[j].ClientIDs = append([]string(nil), rule.ClientIDs...)
			cloned[i].Rules[j].GroupIDs = append([]string(nil), rule.GroupIDs...)
			cloned[i].Rules[j].SourceCIDRs = append([]string(nil), rule.SourceCIDRs...)
			cloned[i].Rules[j].Domains = append([]string(nil), rule.Domains...)
			cloned[i].Rules[j].DomainSuffixes = append([]string(nil), rule.DomainSuffixes...)
			cloned[i].Rules[j].RuleSets = append([]string(nil), rule.RuleSets...)
			cloned[i].Rules[j].CIDRs = append([]string(nil), rule.CIDRs...)
			cloned[i].Rules[j].Ports = append([]int(nil), rule.Ports...)
			cloned[i].Rules[j].Protocols = append([]string(nil), rule.Protocols...)
		}
	}
	return cloned
}
