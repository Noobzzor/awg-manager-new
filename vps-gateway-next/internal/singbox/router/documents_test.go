package router

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEditorDocumentsDraftAppliedAndRestart(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "editor")
	d, err := OpenEditorDocuments(dir)
	if err != nil {
		t.Fatal(err)
	}
	old, next := []byte(`{"route":{"final":"direct"}}`), []byte(`{"route":{"final":"block"}}`)
	if err := d.CommitApplied(old); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveDraft(next); err != nil {
		t.Fatal(err)
	}
	d, err = OpenEditorDocuments(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := d.LoadEffective()
	if err != nil || !bytes.Equal(got, next) {
		t.Fatal("draft lost after reopening", err)
	}
	got, err = d.LoadApplied()
	if err != nil || !bytes.Equal(got, old) {
		t.Fatal("edit replaced applied document", err)
	}
	if err := d.DiscardDraft(); err != nil {
		t.Fatal(err)
	}
	got, err = d.LoadEffective()
	if err != nil || !bytes.Equal(got, old) {
		t.Fatal("discard did not restore applied view", err)
	}
	for _, name := range []string{"applied.json"} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil || st.Mode().Perm() != 0600 {
			t.Fatal("authored credentials are not private", err)
		}
	}
	if err := d.CommitApplied(nil); err != nil {
		t.Fatal(err)
	}
	got, err = d.LoadApplied()
	if err != nil || got != nil {
		t.Fatal("absent snapshot not restored", err)
	}
}

func TestEditorDocumentsRefuseCorruptionAndUnsafeFiles(t *testing.T) {
	d, err := OpenEditorDocuments(filepath.Join(t.TempDir(), "editor"))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SaveDraft([]byte(`{`)); err == nil {
		t.Fatal("invalid document saved")
	}
	if err := d.SaveDraft([]byte(`null`)); err == nil {
		t.Fatal("null is not an editor document")
	}
	if err := d.CommitApplied([]byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d.dir, "draft.json"), []byte(`{`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := d.LoadEffective(); err == nil {
		t.Fatal("corrupt draft silently fell back to applied state")
	}
	if err := os.Remove(filepath.Join(d.dir, "draft.json")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.json")
	if err := os.WriteFile(outside, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(d.dir, "draft.json")); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveDraft([]byte(`{}`)); err == nil {
		t.Fatal("symlink draft accepted")
	}
	if err := d.DiscardDraft(); err == nil {
		t.Fatal("unsafe target silently removed")
	}
	if _, err := d.LoadEffective(); err == nil {
		t.Fatal("symlink draft read")
	}
}
