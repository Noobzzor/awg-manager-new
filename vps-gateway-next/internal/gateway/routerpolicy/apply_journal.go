package routerpolicy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

type fileApplyJournal struct {
	path       string
	syncParent func(string) error
}
type applyJournalEnvelope struct {
	State  applyJournal `json:"state"`
	SHA256 string       `json:"sha256"`
}

func (j fileApplyJournal) Load() (*applyJournal, error) {
	info, err := os.Lstat(j.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect router journal: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("router journal must be a private regular file")
	}
	data, err := os.ReadFile(j.path)
	if err != nil {
		return nil, fmt.Errorf("read router journal: %w", err)
	}
	var envelope applyJournalEnvelope
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("router recovery journal is malformed")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("router recovery journal has trailing data")
	}
	encoded, err := json.Marshal(envelope.State)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	if envelope.SHA256 != hex.EncodeToString(digest[:]) {
		return nil, fmt.Errorf("router recovery journal checksum mismatch")
	}
	if err := validateApplyJournal(envelope.State); err != nil {
		return nil, err
	}
	return &envelope.State, nil
}

func (j fileApplyJournal) Store(v applyJournal) error {
	if err := validateApplyJournal(v); err != nil {
		return err
	}
	if info, err := os.Lstat(j.path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("router journal target is not a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(encoded)
	data, err := json.Marshal(applyJournalEnvelope{State: v, SHA256: hex.EncodeToString(digest[:])})
	if err != nil {
		return err
	}
	if err := storage.AtomicWritePerm(j.path, data, storage.SecretFilePermission); err != nil {
		return err
	}
	// The shared writer's directory sync is best effort. Acceptance of a
	// recovery journal requires a successful sync, including rename metadata.
	return j.syncDirectory()
}

func (j fileApplyJournal) Clear() error {
	info, err := os.Lstat(j.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("router journal target is not a regular file")
	}
	if err := os.Remove(j.path); err != nil {
		return err
	}
	return j.syncDirectory()
}

func (j fileApplyJournal) syncDirectory() error {
	parent := filepath.Dir(j.path)
	if j.syncParent != nil {
		return j.syncParent(parent)
	}
	dir, err := os.Open(parent)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func validateApplyJournal(j applyJournal) error {
	if j.Version != 1 || (j.Phase != "prepared" && j.Phase != "accepted") {
		return fmt.Errorf("unsupported router recovery journal")
	}
	if err := validApplyState(j.Before, false); err != nil {
		return err
	}
	return validApplyState(j.After, true)
}
