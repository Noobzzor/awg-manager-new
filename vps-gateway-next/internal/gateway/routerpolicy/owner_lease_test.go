package routerpolicy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPolicyOwnerLeaseExcludesBothModes(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	gateway, err := AcquirePolicyOwner(ctx, root, PolicyOwnerGateway)
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close()
	if err := gateway.Verify(ctx); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []PolicyOwner{PolicyOwnerGateway, PolicyOwnerOriginal} {
		if second, err := AcquirePolicyOwner(ctx, root, mode); !errors.Is(err, ErrPolicyOwnerBusy) {
			if second != nil {
				second.Close()
			}
			t.Fatal("concurrent owner admitted", mode, err)
		}
	}
	lock, err := os.Stat(filepath.Join(root, ".policy-owner.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if err := gateway.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gateway.Close(); err != nil {
		t.Fatal("close not idempotent", err)
	}
	if err := gateway.Verify(ctx); err == nil {
		t.Fatal("closed lease verifies")
	}
	original, err := AcquirePolicyOwner(ctx, root, PolicyOwnerOriginal)
	if err != nil {
		t.Fatal(err)
	}
	defer original.Close()
	if err := original.Verify(ctx); err != nil {
		t.Fatal(err)
	}
	if owner, err := ReadPolicyOwner(root); err != nil || owner != PolicyOwnerOriginal {
		t.Fatal("original claim absent", owner, err)
	}
	after, err := os.Stat(filepath.Join(root, ".policy-owner.lock"))
	if err != nil || !os.SameFile(lock, after) {
		t.Fatal("lock inode replaced across owner lifecycle")
	}
	if second, err := AcquirePolicyOwner(ctx, root, PolicyOwnerGateway); err == nil {
		second.Close()
		t.Fatal("legacy mode accepted original owner")
	}
}

func TestPolicyOwnerFreshClaimRefusesExistingState(t *testing.T) {
	for _, mode := range []string{"legacy-file", "unclaimed-root", "gateway-marker", "bad-marker"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			dir := OriginalStateDir(root)
			if mode == "legacy-file" {
				if err := os.WriteFile(filepath.Join(root, "gateway-clients.json"), []byte("protected"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				raw := []byte(`{"version":1,"mode":"gateway"}`)
				if mode == "bad-marker" {
					raw = []byte(`{"version":999,"mode":"original"}`)
				}
				if mode != "unclaimed-root" {
					if err := os.WriteFile(filepath.Join(dir, "owner.json"), raw, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if lease, err := AcquirePolicyOwner(context.Background(), root, PolicyOwnerOriginal); err == nil {
				lease.Close()
				t.Fatal("existing state claimed")
			}
			if _, err := os.Lstat(filepath.Join(root, ".policy-owner.lock")); !os.IsNotExist(err) {
				t.Fatal("refused state created lock")
			}
			if mode == "legacy-file" {
				raw, err := os.ReadFile(filepath.Join(root, "gateway-clients.json"))
				if err != nil || string(raw) != "protected" {
					t.Fatal("legacy state changed")
				}
			}
		})
	}
}

func TestPolicyOwnerLeaseRejectsUnsafeLock(t *testing.T) {
	for _, mode := range []string{"link", "wide", "hardlink", "nonempty", "directory"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			lock := filepath.Join(root, ".policy-owner.lock")
			protected := filepath.Join(t.TempDir(), "protected")
			content := "protected"
			if mode == "hardlink" {
				content = ""
			}
			if err := os.WriteFile(protected, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			var err error
			switch mode {
			case "link":
				err = os.Symlink(protected, lock)
			case "hardlink":
				err = os.Link(protected, lock)
			case "directory":
				err = os.Mkdir(lock, 0700)
			case "wide":
				err = os.WriteFile(lock, nil, 0644)
			case "nonempty":
				err = os.WriteFile(lock, []byte("foreign"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if lease, err := AcquirePolicyOwner(context.Background(), root, PolicyOwnerGateway); err == nil {
				lease.Close()
				t.Fatal("unsafe lock accepted")
			}
			raw, err := os.ReadFile(protected)
			if err != nil || string(raw) != content {
				t.Fatal("protected resource changed")
			}
		})
	}
}

func TestPolicyOwnerLeaseRejectsReplacedResources(t *testing.T) {
	for _, resource := range []string{"lock", "owner", "root"} {
		t.Run(resource, func(t *testing.T) {
			root := t.TempDir()
			lease, err := AcquirePolicyOwner(context.Background(), root, PolicyOwnerOriginal)
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			path := filepath.Join(root, ".policy-owner.lock")
			if resource == "owner" {
				path = filepath.Join(OriginalStateDir(root), "owner.json")
			}
			if resource == "root" {
				path = OriginalStateDir(root)
			}
			if err := os.Rename(path, path+".protected"); err != nil {
				t.Fatal(err)
			}
			if resource == "root" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(path, "owner.json")
			}
			raw := []byte(nil)
			if resource != "lock" {
				raw = []byte(`{"version":1,"mode":"original"}`)
			}
			if err := os.WriteFile(path, raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := lease.Verify(context.Background()); err == nil {
				t.Fatal("replaced ownership resource accepted")
			}
			if err := lease.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(path); err != nil {
				t.Fatal("close removed replacement", err)
			}
		})
	}
}

func TestPolicyOwnerLeaseCancellationDoesNotWrite(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if lease, err := AcquirePolicyOwner(ctx, root, PolicyOwnerOriginal); !errors.Is(err, context.Canceled) {
		if lease != nil {
			lease.Close()
		}
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("cancelled owner acquisition wrote state")
	}
}

func TestPolicyOwnerLeaseChild(t *testing.T) {
	root := os.Getenv("AWGM_POLICY_OWNER_CHILD")
	if root == "" {
		t.Skip("subprocess ownership fixture")
	}
	lease, err := AcquirePolicyOwner(context.Background(), root, PolicyOwnerOriginal)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	fmt.Fprintln(os.Stdout, "LEASE_HELD")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestPolicyOwnerLeaseCrashReleasesKernelLock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	root := t.TempDir()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestPolicyOwnerLeaseChild$", "-test.v")
	child.Env = append(os.Environ(), "AWGM_POLICY_OWNER_CHILD="+root)
	out, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	in, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill(); _ = child.Wait() }()
	scanner := bufio.NewScanner(out)
	ready := false
	for scanner.Scan() {
		if scanner.Text() == "LEASE_HELD" {
			ready = true
			break
		}
	}
	if !ready {
		t.Fatal("child did not acquire owner", scanner.Err())
	}
	if second, err := AcquirePolicyOwner(context.Background(), root, PolicyOwnerOriginal); !errors.Is(err, ErrPolicyOwnerBusy) {
		if second != nil {
			second.Close()
		}
		t.Fatal("cross-process exclusion failed", err)
	}
	_ = child.Process.Kill()
	_ = child.Wait()
	resumed, err := AcquirePolicyOwner(context.Background(), root, PolicyOwnerOriginal)
	if err != nil {
		t.Fatal("kernel lock survived child crash", err)
	}
	defer resumed.Close()
	if err := resumed.Verify(context.Background()); err != nil {
		t.Fatal(err)
	}
}
