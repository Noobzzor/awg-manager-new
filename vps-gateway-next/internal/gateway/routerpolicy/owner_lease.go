package routerpolicy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

var ErrPolicyOwnerBusy = errors.New("policy state is already owned by another manager")
var ErrPolicyOwnerAdmission = errors.New("policy owner lease admission denied")

// FreshClaim reports whether this acquisition created the original owner marker.
// It records the decision made while holding the exclusive lock.
func (l *PolicyOwnerLease) FreshClaim() bool { return l.fresh }

const policyOwnerLockName = ".policy-owner.lock"

// PolicyOwnerLease excludes cooperating manager processes for its entire
// lifetime. It proves policy ownership, not ownership of a surviving engine.
// Never unlink the lock on Close: another opener must see the same inode.
type PolicyOwnerLease struct {
	mu        sync.Mutex
	file      *os.File
	dataDir   string
	mode      PolicyOwner
	fresh     bool
	dataInfo  os.FileInfo
	stateInfo os.FileInfo
	ownerInfo os.FileInfo
}

func ownerAdmission(reason string) error {
	return fmt.Errorf("%w: %s", ErrPolicyOwnerAdmission, reason)
}

func policyDataDirectory(dataDir string) (os.FileInfo, error) {
	if !filepath.IsAbs(dataDir) {
		return nil, ownerAdmission("data directory must be absolute")
	}
	info, err := os.Lstat(dataDir)
	if err != nil || !info.IsDir() || info.Mode().Perm()&0022 != 0 {
		return nil, ownerAdmission("data directory must be regular and not writable by other users")
	}
	return info, nil
}

func originalFreshState(dataDir string) error {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		return ownerAdmission("cannot inspect fresh state")
	}
	for _, entry := range entries {
		if entry.Name() != policyOwnerLockName {
			return ownerAdmission("original mode requires fresh state; migration is not automatic")
		}
	}
	return nil
}

func preflightPolicyOwner(ctx context.Context, dataDir string, mode PolicyOwner) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if mode == PolicyOwnerGateway {
		return false, RequireGatewayOwner(ctx, dataDir)
	}
	if mode != PolicyOwnerOriginal {
		return false, ownerAdmission("unknown requested owner")
	}
	if _, err := os.Lstat(OriginalStateDir(dataDir)); os.IsNotExist(err) {
		return true, originalFreshState(dataDir)
	}
	actual, err := ReadPolicyOwner(dataDir)
	if err != nil || actual != PolicyOwnerOriginal {
		return false, ownerAdmission("existing state cannot be claimed or converted")
	}
	return false, nil
}

// AcquirePolicyOwner performs admission before touching the lock, repeats it
// under an exclusive nonblocking OS lock, and retains that lock until Close.
// Original mode only claims an empty data directory; it never converts a
// gateway marker or repairs unclaimed/ambiguous crash state automatically.
func AcquirePolicyOwner(ctx context.Context, dataDir string, mode PolicyOwner) (*PolicyOwnerLease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := policyDataDirectory(dataDir)
	if err != nil {
		return nil, err
	}
	if _, err := preflightPolicyOwner(ctx, dataDir, mode); err != nil {
		return nil, err
	}
	path := filepath.Join(dataDir, policyOwnerLockName)
	fd, err := unix.Open(path, unix.O_RDWR|unix.O_CREAT|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, ownerAdmission("cannot open private owner lock")
	}
	lease := &PolicyOwnerLease{file: os.NewFile(uintptr(fd), path), dataDir: dataDir, mode: mode, dataInfo: info}
	accepted := false
	defer func() {
		if !accepted {
			_ = lease.Close()
		}
	}()
	stat, err := lease.file.Stat()
	if err != nil || !stat.Mode().IsRegular() || stat.Mode().Perm()&0077 != 0 || stat.Size() != 0 {
		return nil, ownerAdmission("owner lock must be an empty private regular file")
	}
	var kernelStat unix.Stat_t
	if err := unix.Fstat(fd, &kernelStat); err != nil || kernelStat.Nlink != 1 {
		return nil, ownerAdmission("owner lock has multiple or unverifiable links")
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrPolicyOwnerBusy
		}
		return nil, ownerAdmission("cannot lock policy state")
	}
	if err := lease.checkLock(); err != nil {
		return nil, err
	}
	fresh, err := preflightPolicyOwner(ctx, dataDir, mode)
	if err != nil {
		return nil, err
	}
	lease.fresh = fresh
	if err := lease.file.Sync(); err != nil {
		return nil, ownerAdmission("cannot sync owner lock")
	}
	if err := syncPolicyDirectory(dataDir); err != nil {
		return nil, err
	}
	if fresh {
		dir := OriginalStateDir(dataDir)
		if err := os.Mkdir(dir, 0700); err != nil {
			return nil, ownerAdmission("cannot create fresh original owner directory")
		}
		// Persist the new directory entry before accepting files below it.
		if err := syncPolicyDirectory(dataDir); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(filepath.Join(dir, "owner.json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return nil, ownerAdmission("cannot create original owner marker")
		}
		_, writeErr := f.Write([]byte("{\"version\":1,\"mode\":\"original\"}\n"))
		syncErr := f.Sync()
		closeErr := f.Close()
		if writeErr != nil || syncErr != nil || closeErr != nil {
			return nil, ownerAdmission("cannot persist original owner marker")
		}
		if err := syncPolicyDirectory(dir); err != nil {
			return nil, err
		}
	}
	if mode == PolicyOwnerOriginal {
		lease.stateInfo, err = os.Lstat(OriginalStateDir(dataDir))
		if err != nil {
			return nil, ownerAdmission("cannot inspect original owner root")
		}
		lease.ownerInfo, err = os.Lstat(filepath.Join(OriginalStateDir(dataDir), "owner.json"))
		if err != nil {
			return nil, ownerAdmission("cannot inspect original owner marker")
		}
	}
	if err := lease.Verify(ctx); err != nil {
		return nil, err
	}
	accepted = true
	return lease, nil
}

func syncPolicyDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return ownerAdmission("cannot open owner directory for sync")
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return ownerAdmission("cannot sync owner directory")
	}
	return nil
}

func (l *PolicyOwnerLease) checkLock() error {
	info, err := policyDataDirectory(l.dataDir)
	if err != nil || !os.SameFile(l.dataInfo, info) {
		return ownerAdmission("data directory identity changed")
	}
	path, err := os.Lstat(filepath.Join(l.dataDir, policyOwnerLockName))
	if err != nil || !path.Mode().IsRegular() || path.Mode().Perm()&0077 != 0 || path.Size() != 0 {
		return ownerAdmission("owner lock identity changed")
	}
	held, err := l.file.Stat()
	if err != nil || !os.SameFile(path, held) {
		return ownerAdmission("owner lock was replaced")
	}
	return nil
}

func (l *PolicyOwnerLease) Verify(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return ownerAdmission("owner lease is closed")
	}
	if err := l.checkLock(); err != nil {
		return err
	}
	owner, err := ReadPolicyOwner(l.dataDir)
	if err != nil || owner != l.mode {
		return ownerAdmission("persisted policy owner changed")
	}
	if l.mode == PolicyOwnerOriginal {
		state, err := os.Lstat(OriginalStateDir(l.dataDir))
		if err != nil || !os.SameFile(state, l.stateInfo) {
			return ownerAdmission("original owner directory was replaced")
		}
		marker, err := os.Lstat(filepath.Join(OriginalStateDir(l.dataDir), "owner.json"))
		if err != nil || !os.SameFile(marker, l.ownerInfo) {
			return ownerAdmission("original owner marker was replaced")
		}
	}
	return ctx.Err()
}

func (l *PolicyOwnerLease) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	f := l.file
	l.file = nil
	return f.Close()
}
