package routerpolicy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatewayOwnerReadAdmission(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		mode       PolicyOwner
		bad        bool
	}{
		{"gateway", `{"version":1,"mode":"gateway"}`, PolicyOwnerGateway, false},
		{"original", `{"version":1,"mode":"original"}`, PolicyOwnerOriginal, false},
		{"malformed", "{", "", true},
		{"version", `{"version":2,"mode":"gateway"}`, "", true},
		{"unknown-owner", `{"version":1,"mode":"unknown"}`, "", true},
		{"duplicate", `{"version":1,"mode":"original","mode":"gateway"}`, "", true},
		{"trailing", `{"version":1,"mode":"gateway"}{}`, "", true},
		{"oversize", strings.Repeat(" ", 4097) + `{"version":1,"mode":"gateway"}`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := OriginalStateDir(root)
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "owner.json")
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := ReadPolicyOwner(root)
			if (err != nil) != tc.bad || (!tc.bad && got != tc.mode) {
				t.Fatal("owner inspection accepted invalid state or changed the owner", err)
			}
			raw, _ := os.ReadFile(path)
			if string(raw) != tc.body {
				t.Fatal("inspection changed owner state")
			}
		})
	}
}

func TestGatewayOwnerRejectsUnsafeLayout(t *testing.T) {
	for _, kind := range []string{"unclaimed", "public-directory", "public-file", "linked-file", "linked-directory", "gateway-journal"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			dir := OriginalStateDir(root)
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "owner.json")
			body := []byte(`{"version":1,"mode":"gateway"}`)
			if kind != "unclaimed" {
				if err := os.WriteFile(path, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "public-directory":
				if err := os.Chmod(dir, 0755); err != nil {
					t.Fatal(err)
				}
			case "public-file":
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			case "linked-file":
				target := filepath.Join(t.TempDir(), "owner")
				if err := os.WriteFile(target, body, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "linked-directory":
				target := filepath.Join(root, "moved")
				if err := os.Rename(dir, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, dir); err != nil {
					t.Fatal(err)
				}
			case "gateway-journal":
				if err := os.WriteFile(filepath.Join(dir, "apply-journal.json"), []byte("recovery pending"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := ReadPolicyOwner(root); err == nil {
				t.Fatal("unsafe or unresolved original state accepted by legacy reader")
			}
		})
	}
}

func TestGatewayOwnerMissingStateDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	got, err := ReadPolicyOwner(root)
	if err != nil || got != PolicyOwnerGateway {
		t.Fatal("absent state no longer preserves legacy owner", err)
	}
	if _, err := os.Stat(OriginalStateDir(root)); !os.IsNotExist(err) {
		t.Fatal("owner inspection created state")
	}
}
func TestGatewayOwnerRequiresLegacyAndContext(t *testing.T) {
	root := t.TempDir()
	dir := OriginalStateDir(root)
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "owner.json"), []byte(`{"version":1,"mode":"original"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(RequireGatewayOwner(context.Background(), root), ErrGatewayPolicyOwnership) {
		t.Fatal("legacy writer admitted under original owner")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !errors.Is(RequireGatewayOwner(ctx, t.TempDir()), context.Canceled) {
		t.Fatal("cancelled owner admission accepted")
	}
}
