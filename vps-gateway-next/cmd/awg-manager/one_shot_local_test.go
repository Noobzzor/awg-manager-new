package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLocalCleanupRejectsLinkedStateBeforeUninstall(t *testing.T) {
	for _, leaf := range []string{"root", "disabled", "pending"} {
		t.Run(leaf, func(t *testing.T) {
			dataDir, outside := t.TempDir(), t.TempDir()
			sentinel := filepath.Join(outside, "keep.json")
			if err := os.WriteFile(sentinel, []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			configDir := filepath.Join(dataDir, "sing-box", "config.d")
			link := configDir
			if leaf != "root" {
				if err := os.MkdirAll(configDir, 0700); err != nil {
					t.Fatal(err)
				}
				link = filepath.Join(configDir, leaf)
			} else {
				if err := os.MkdirAll(filepath.Dir(configDir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(outside, link); err != nil {
				t.Fatal(err)
			}
			called := false
			err := runLocalCleanupWith(runtimeSettings{dataDir: dataDir}, func() error { called = true; return nil })
			if err == nil || called {
				t.Fatalf("err=%v uninstall called=%v; want early refusal", err, called)
			}
			contents, readErr := os.ReadFile(sentinel)
			if readErr != nil || string(contents) != "untouched" {
				t.Fatalf("external fixture changed: %v", readErr)
			}
		})
	}
}

func TestRunOneShot_KeeneticCleanupPropagatesFailure(t *testing.T) {
	failure := errors.New("cleanup incomplete")
	h := oneShotHooks{legacyCleanup: func(string) error { return failure }}
	handled, err := runOneShot(deploymentModeKeenetic, runtimeSettings{dataDir: t.TempDir()}, true, "", h)
	if !handled || !errors.Is(err, failure) {
		t.Fatalf("handled=%v err=%v, want handled cleanup failure", handled, err)
	}
}

func TestRunOneShot_LocalCleanupNeverCallsLegacyOrService(t *testing.T) {
	var called []string
	h := oneShotHooks{
		localCleanup: func(runtimeSettings) error {
			called = append(called, "local-cleanup")
			return nil
		},
		legacyCleanup: func(string) error { t.Fatal("local cleanup entered legacy NDMS/Entware cleanup"); return nil },
		legacyService: func(string, string) { t.Fatal("local cleanup entered legacy service path") },
	}
	handled, err := runOneShot(deploymentModeSingbox, runtimeSettings{dataDir: t.TempDir()}, true, "", h)
	if err != nil || !handled {
		t.Fatalf("runOneShot handled=%v err=%v", handled, err)
	}
	if want := []string{"local-cleanup"}; !reflect.DeepEqual(called, want) {
		t.Fatalf("calls=%v want=%v", called, want)
	}
}

func TestRunOneShot_LocalServiceFailsBeforeAnyLegacyAction(t *testing.T) {
	h := oneShotHooks{
		localCleanup:  func(runtimeSettings) error { t.Fatal("service request ran cleanup"); return nil },
		legacyCleanup: func(string) error { t.Fatal("local service entered legacy cleanup"); return nil },
		legacyService: func(string, string) { t.Fatal("local service entered legacy service/Entware path") },
	}
	handled, err := runOneShot(deploymentModeSingbox, runtimeSettings{dataDir: t.TempDir()}, false, "start", h)
	if !handled || err == nil {
		t.Fatalf("runOneShot handled=%v err=%v, want handled error", handled, err)
	}
	if msg := err.Error(); !strings.Contains(msg, "--service") || !strings.Contains(msg, "AWG_MODE=singbox") {
		t.Fatalf("unclear local service error: %q", msg)
	}
}

func TestRunOneShot_KeeneticPreservesLegacyDispatch(t *testing.T) {
	var called []string
	h := oneShotHooks{
		localCleanup:  func(runtimeSettings) error { t.Fatal("keenetic entered local cleanup"); return nil },
		legacyCleanup: func(dataDir string) error { called = append(called, "cleanup:"+dataDir); return nil },
		legacyService: func(action, dataDir string) { called = append(called, "service:"+action+":"+dataDir) },
	}
	if handled, err := runOneShot(deploymentModeKeenetic, runtimeSettings{dataDir: "/legacy"}, true, "", h); !handled || err != nil {
		t.Fatalf("legacy cleanup handled=%v err=%v", handled, err)
	}
	if handled, err := runOneShot(deploymentModeKeenetic, runtimeSettings{dataDir: "/legacy"}, false, "status", h); !handled || err != nil {
		t.Fatalf("legacy service handled=%v err=%v", handled, err)
	}
	if want := []string{"cleanup:/legacy", "service:status:/legacy"}; !reflect.DeepEqual(called, want) {
		t.Fatalf("calls=%v want=%v", called, want)
	}
}

func TestRunOneShot_KeeneticCleanupStillWinsWhenBothFlagsAreSet(t *testing.T) {
	var called []string
	h := oneShotHooks{
		localCleanup:  func(runtimeSettings) error { t.Fatal("keenetic entered local cleanup"); return nil },
		legacyCleanup: func(string) error { called = append(called, "cleanup"); return nil },
		legacyService: func(string, string) { called = append(called, "service") },
	}
	handled, err := runOneShot(deploymentModeKeenetic, runtimeSettings{dataDir: "/legacy"}, true, "restart", h)
	if err != nil || !handled {
		t.Fatalf("handled=%v err=%v", handled, err)
	}
	if want := []string{"cleanup"}; !reflect.DeepEqual(called, want) {
		t.Fatalf("calls=%v want=%v", called, want)
	}
}
