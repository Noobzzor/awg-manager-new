package router

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

// EditorDocuments stores canonical authored documents separately from the
// engine's derived runtime slots. All writers use the portable coordinator.
type EditorDocuments interface {
	LoadEffective() ([]byte, error)
	LoadApplied() ([]byte, error)
	SaveDraft([]byte) error
	DiscardDraft() error
}

type FileEditorDocuments struct{ dir string }

func OpenEditorDocuments(dir string) (*FileEditorDocuments, error) {
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("editor document directory must be absolute")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	d := &FileEditorDocuments{dir: dir}
	if err := d.checkDirectory(); err != nil {
		return nil, err
	}
	return d, nil
}
func (d *FileEditorDocuments) LoadEffective() ([]byte, error) {
	raw, err := d.read("draft.json")
	if err != nil || raw != nil {
		return raw, err
	}
	return d.LoadApplied()
}
func (d *FileEditorDocuments) LoadApplied() ([]byte, error) { return d.read("applied.json") }
func (d *FileEditorDocuments) SaveDraft(raw []byte) error   { return d.write("draft.json", raw) }
func (d *FileEditorDocuments) DiscardDraft() error          { return d.remove("draft.json") }

// CommitApplied is for the journaled apply backend, never ordinary CRUD.
func (d *FileEditorDocuments) CommitApplied(raw []byte) error {
	if raw == nil {
		return d.remove("applied.json")
	}
	return d.write("applied.json", raw)
}

func (d *FileEditorDocuments) checkDirectory() error {
	info, err := os.Lstat(d.dir)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("editor document directory must be private and regular")
	}
	return nil
}

func (d *FileEditorDocuments) checkFile(name string) (bool, error) {
	if err := d.checkDirectory(); err != nil {
		return false, err
	}
	info, err := os.Lstat(filepath.Join(d.dir, name))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return false, fmt.Errorf("editor document must be a private regular file")
	}
	return true, nil
}

func validEditorDocument(raw []byte) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return fmt.Errorf("editor document is malformed")
	}
	return nil
}

func (d *FileEditorDocuments) read(name string) ([]byte, error) {
	present, err := d.checkFile(name)
	if err != nil || !present {
		return nil, err
	}
	raw, err := os.ReadFile(filepath.Join(d.dir, name))
	if err != nil {
		return nil, err
	}
	if err := validEditorDocument(raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (d *FileEditorDocuments) write(name string, raw []byte) error {
	if err := validEditorDocument(raw); err != nil {
		return err
	}
	if _, err := d.checkFile(name); err != nil {
		return err
	}
	if err := storage.AtomicWritePerm(filepath.Join(d.dir, name), raw, storage.SecretFilePermission); err != nil {
		return err
	}
	return d.syncDirectory()
}

func (d *FileEditorDocuments) remove(name string) error {
	present, err := d.checkFile(name)
	if err != nil || !present {
		return err
	}
	if err := os.Remove(filepath.Join(d.dir, name)); err != nil {
		return err
	}
	return d.syncDirectory()
}

func (d *FileEditorDocuments) syncDirectory() error {
	f, err := os.Open(d.dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
