package ingress

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateServerIdentityPersistsKeyAcrossRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "server-identity.json")
	calls := 0
	generate := func() (string, string, error) { calls++; return "private", "public", nil }
	first, err := LoadOrCreateServerIdentity(path, generate)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadOrCreateServerIdentity(path, generate)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || calls != 1 {
		t.Fatalf("first=%+v second=%+v calls=%d", first, second, calls)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%v err=%v", info.Mode().Perm(), err)
	}
}
