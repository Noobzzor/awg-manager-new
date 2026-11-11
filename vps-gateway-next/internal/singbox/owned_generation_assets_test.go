package singbox

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

func TestOwnedGenerationRejectsChangedRuleSet(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join("assets", "test.srs")
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0700); err != nil {
		t.Fatal(err)
	}
	original := []byte("initial")
	path := filepath.Join(dir, name)
	if err := writeOwnedGenerationFile(path, original); err != nil {
		t.Fatal(err)
	}
	g := ownedPreparedGeneration{
		path:   filepath.Join(dir, "config.json"),
		assets: map[string][sha256.Size]byte{name: sha256.Sum256(original)},
	}
	if err := g.verifyAssets(); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("altered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0400); err != nil {
		t.Fatal(err)
	}
	if err := g.verifyAssets(); err == nil {
		t.Fatal("same-size replacement passed owned generation confirmation")
	}
}
