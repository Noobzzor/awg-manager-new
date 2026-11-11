package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/gateway/routerpolicy"
	"golang.org/x/sys/unix"
)

func TestLocalCleanupOwnerAdmissionBeforeUninstall(t *testing.T) {
	for _, mode := range []string{"busy", "original", "unclaimed"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if mode == "busy" {
				f, err := os.OpenFile(filepath.Join(root, ".policy-owner.lock"), os.O_CREATE|os.O_RDWR, 0600)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
					t.Fatal(err)
				}
			} else {
				dir := filepath.Join(root, "original-router")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if mode == "original" {
					if err := os.WriteFile(filepath.Join(dir, "owner.json"), []byte(`{"version":1,"mode":"original"}`), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			called := false
			err := runLocalCleanupWith(runtimeSettings{dataDir: root}, func() error { called = true; return nil })
			if err == nil || called {
				t.Fatal("cleanup reached uninstall before owner admission", err, called)
			}
		})
	}
}

func TestLocalCleanupKeepsOwnerUntilUninstallReturns(t *testing.T) {
	root := t.TempDir()
	want := errors.New("uninstall refused")
	err := runLocalCleanupWith(runtimeSettings{dataDir: root}, func() error {
		lease, err := routerpolicy.AcquirePolicyOwner(context.Background(), root, routerpolicy.PolicyOwnerGateway)
		if !errors.Is(err, routerpolicy.ErrPolicyOwnerBusy) {
			if lease != nil {
				lease.Close()
			}
			t.Error("cleanup has no exclusive owner", err)
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
	lease, err := routerpolicy.AcquirePolicyOwner(context.Background(), root, routerpolicy.PolicyOwnerGateway)
	if err != nil {
		t.Fatal("cleanup leaked lease", err)
	}
	lease.Close()
}

func TestLocalOwnerSurvivesRegisteredCleanup(t *testing.T) {
	root := t.TempDir()
	a := &app{dataDir: root}
	if err := a.prepareLocalPolicyOwner(); err != nil {
		t.Fatal(err)
	}
	defer a.localPolicyOwner.Close()
	if err := a.prepareLocalPolicyOwner(); err != nil {
		t.Fatal(err)
	}
	if len(a.onExit) != 1 {
		t.Fatal("repeated admission registered duplicate ownership cleanup")
	}
	cleaned := false
	a.deferOnExit(func() {
		cleaned = true
		if second, err := routerpolicy.AcquirePolicyOwner(context.Background(), root, routerpolicy.PolicyOwnerGateway); !errors.Is(err, routerpolicy.ErrPolicyOwnerBusy) {
			if second != nil {
				second.Close()
			}
			t.Error("ownership released before producer cleanup", err)
		}
	})
	a.runOnExit()
	if !cleaned {
		t.Fatal("producer cleanup not called")
	}
	next, err := routerpolicy.AcquirePolicyOwner(context.Background(), root, routerpolicy.PolicyOwnerGateway)
	if err != nil {
		t.Fatal("cleanup leaked owner", err)
	}
	next.Close()
}

func ownerStartupTree(t *testing.T, root string) []byte {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		value := fmt.Sprint(info.Mode())
		if info.Mode().IsRegular() {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += "|" + string(raw)
		}
		tree[rel] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestNativeLocalOwnerAdmissionBeforeCore(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_STARTUP_OWNER_PROOF") != "1" {
		t.Skip("requires isolated compiled application control")
	}
	appBinary := os.Getenv("AWGM_STARTUP_APP")
	if appBinary == "" {
		t.Fatal("AWGM_STARTUP_APP required")
	}
	for _, mode := range []string{"busy", "original", "unclaimed"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "settings.json"), []byte("{"), 0600); err != nil {
				t.Fatal(err)
			}
			if mode == "busy" {
				f, err := os.OpenFile(filepath.Join(root, ".policy-owner.lock"), os.O_CREATE|os.O_RDWR, 0600)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
					t.Fatal(err)
				}
			} else {
				dir := filepath.Join(root, "original-router")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if mode == "original" {
					if err := os.WriteFile(filepath.Join(dir, "owner.json"), []byte(`{"version":1,"mode":"original"}`), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			before := ownerStartupTree(t, root)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, appBinary)
			cmd.Env = append(os.Environ(), "AWG_MODE=singbox", "AWG_DATA_DIR="+root, "SINGBOX_BIN=/usr/local/bin/sing-box", "SINGBOX_CONFIG_DIR="+filepath.Join(root, "sing-box/config.d"), "AWG_HTTP_ADDR=127.0.0.1:2222", "AWG_PROXY_ADDR=127.0.0.1:1080")
			output, err := cmd.CombinedOutput()
			if err == nil || ctx.Err() != nil {
				t.Fatal("compiled application did not refuse startup promptly", err, ctx.Err())
			}
			if !bytes.Contains(output, []byte("policy owner")) {
				t.Error("compiled application failed before reaching owner admission")
			}
			if !bytes.Equal(before, ownerStartupTree(t, root)) {
				t.Error("compiled application touched protected state before owner admission")
			}
		})
	}
}

func TestNativeLocalOwnerLifetime(t *testing.T) {
	if os.Getenv("AWGM_NATIVE_STARTUP_OWNER_PROOF") != "1" {
		t.Skip("requires isolated compiled application control")
	}
	appBinary := os.Getenv("AWGM_STARTUP_APP")
	if appBinary == "" {
		t.Fatal("AWGM_STARTUP_APP required")
	}
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, appBinary)
	cmd.Env = append(os.Environ(), "AWG_MODE=singbox", "AWG_DATA_DIR="+root, "SINGBOX_BIN=/usr/local/bin/sing-box", "SINGBOX_CONFIG_DIR="+filepath.Join(root, "sing-box/config.d"), "AWG_HTTP_ADDR=127.0.0.1:2222", "AWG_PROXY_ADDR=127.0.0.1:1080", "AWG_GATEWAY_ENABLE=false")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	ready := false
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://127.0.0.1:2222/")
		if err == nil {
			resp.Body.Close()
			ready = true
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !ready {
		t.Fatal("compiled application did not serve local panel")
	}
	if second, err := routerpolicy.AcquirePolicyOwner(context.Background(), root, routerpolicy.PolicyOwnerGateway); !errors.Is(err, routerpolicy.ErrPolicyOwnerBusy) {
		if second != nil {
			second.Close()
		}
		t.Fatal("serving application released owner", err)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal("application did not shut down cleanly", err)
	}
	next, err := routerpolicy.AcquirePolicyOwner(context.Background(), root, routerpolicy.PolicyOwnerGateway)
	if err != nil {
		t.Fatal("application exit leaked owner", err)
	}
	defer next.Close()
	if err := next.Verify(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Log("native application holds owner while serving panel and releases it after shutdown")
}

// Deliberately omit downstream services: denied startup must never reach them.
func TestLocalStartupAdmitsPolicyOwnerBeforeRuntime(t *testing.T) {
	for _, mode := range []string{"busy", "original", "unclaimed"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if mode == "busy" {
				f, err := os.OpenFile(filepath.Join(root, ".policy-owner.lock"), os.O_CREATE|os.O_RDWR, 0600)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
					t.Fatal(err)
				}
			} else {
				dir := filepath.Join(root, "original-router")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if mode == "original" {
					if err := os.WriteFile(filepath.Join(dir, "owner.json"), []byte(`{"version":1,"mode":"original"}`), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			a := &app{dataDir: root}
			defer a.runOnExit()
			defer func() {
				if v := recover(); v != nil {
					t.Errorf("runtime construction reached before policy owner admission: %v", v)
				}
			}()
			if err := a.setupLocal(); err == nil {
				t.Error("startup accepted unavailable policy owner")
			}
			if a.eventBus != nil {
				t.Error("startup created runtime events before owner admission")
			}
			if mode != "busy" {
				if _, err := os.Lstat(filepath.Join(root, ".policy-owner.lock")); !os.IsNotExist(err) {
					t.Error("denied original state created a legacy lock file")
				}
			}
		})
	}
}
