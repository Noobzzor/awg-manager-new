package warp

import (
	"bytes"
	"context"
	"errors"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/storage"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type importOrch struct {
	serviceSaver
	preflightErr    error
	preflightReject bool
	checks          int
}

func (o *importOrch) CheckMerged(_ orchestrator.Slot, _ []byte) (orchestrator.ValidationResult, error) {
	o.checks++
	if o.preflightReject {
		return orchestrator.ValidationResult{Errors: []orchestrator.ValidationError{{Message: "SECRET"}}}, nil
	}
	return orchestrator.ValidationResult{}, o.preflightErr
}
func TestImportStoreTransaction(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "initial", true: "replacement"}[existing], func(t *testing.T) {
			st := NewStore(t.TempDir())
			p := sampleProfile()
			if existing {
				if err := st.Save(p); err != nil {
					t.Fatal(err)
				}
			}
			old, _ := os.ReadFile(st.path)
			p.MTU = 1400
			called := false
			err := st.saveAndApply(p, func() error { called = true; return errors.New("apply failure") })
			if err == nil || !called {
				t.Fatal("apply failure not reported")
			}
			now, readerr := os.ReadFile(st.path)
			if existing {
				if readerr != nil || !bytes.Equal(now, old) {
					t.Fatal("prior encrypted envelope not restored")
				}
			} else if !os.IsNotExist(readerr) {
				t.Fatal("failed first import persisted")
			}
			if err = st.saveAndApply(p, func() error { return nil }); err != nil {
				t.Fatal(err)
			}
			loaded, err := st.Load()
			if err != nil || loaded == nil || loaded.MTU != 1400 {
				t.Fatal("candidate missing after successful apply")
			}
		})
	}
}
func TestImportStoreWriteFailureNeverApplies(t *testing.T) {
	st := NewStore(t.TempDir())
	st.writeAtomic = func(string, []byte, os.FileMode) error { return errors.New("disk") }
	called := false
	err := st.saveAndApply(sampleProfile(), func() error { called = true; return nil })
	if err == nil || called {
		t.Fatal("write failure applied candidate")
	}
}
func TestImportServiceCommitAndRollback(t *testing.T) {
	for _, mode := range []string{"success", "preflight reject", "preflight infra", "apply reject", "apply infra", "rollback failure"} {
		t.Run(mode, func(t *testing.T) {
			st := NewStore(t.TempDir())
			p := sampleProfile()
			if err := st.Save(p); err != nil {
				t.Fatal(err)
			}
			old, _ := os.ReadFile(st.path)
			o := &importOrch{}
			s := NewService(st, o)
			if err := s.Sync(); err != nil {
				t.Fatal(err)
			}
			p.MTU = 1400
			switch mode {
			case "preflight reject":
				o.preflightReject = true
			case "preflight infra":
				o.preflightErr = errors.New("SECRET")
			case "apply reject":
				o.reject = true
			case "apply infra":
				o.err = errors.New("SECRET")
			case "rollback failure":
				o.err = errors.New("SECRET")
				writes := 0
				st.writeAtomic = func(path string, b []byte, m os.FileMode) error {
					writes++
					if writes == 2 {
						return errors.New("rollback disk")
					}
					return storage.AtomicWritePerm(path, b, m)
				}
			}
			err := s.Import(p)
			cfg, cfgerr := s.WARPConfig()
			now, _ := os.ReadFile(st.path)
			if mode == "success" {
				if err != nil || cfgerr != nil || cfg == nil || cfg.MTU != 1400 {
					t.Fatal("committed provider unavailable")
				}
				d, e := s.Describe()
				if e != nil || !d.Configured || d.MTU != 1400 {
					t.Fatal("safe description missing")
				}
				return
			}
			if err == nil {
				t.Fatal("failed transaction accepted")
			}
			if mode == "rollback failure" {
				if !errors.Is(err, ErrRollbackFailed) || cfgerr == nil {
					t.Fatal("failed rollback did not poison provider")
				}
				return
			}
			if !bytes.Equal(old, now) || cfgerr != nil || cfg == nil || cfg.MTU != 1280 {
				t.Fatal("prior provider or encrypted envelope changed")
			}
		})
	}
}
func TestImportRejectsInvalidStaleOrUnsynchronized(t *testing.T) {
	st := NewStore(t.TempDir())
	o := &importOrch{}
	s := NewService(st, o)
	p := sampleProfile()
	if s.Import(p) == nil {
		t.Fatal("unsynchronized import accepted")
	}
	if err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	p.MTU = 1
	if s.Import(p) == nil {
		t.Fatal("invalid import accepted")
	}
	if _, err := os.Stat(st.path); !os.IsNotExist(err) {
		t.Fatal("invalid import persisted")
	}
	p = sampleProfile()
	if err := st.Save(p); err != nil {
		t.Fatal(err)
	}
	if s.Import(p) == nil {
		t.Fatal("stale state overwritten")
	}
}
func TestImportConcurrentProjection(t *testing.T) {
	st := NewStore(t.TempDir())
	o := &importOrch{}
	s := NewService(st, o)
	if err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Import(sampleProfile()); err != nil {
				t.Error("import failed")
			}
			if _, err := s.Describe(); err != nil {
				t.Error("projection failed")
			}
		}()
	}
	wg.Wait()
}

type importSecondCheckReject struct{ calls int }

func (v *importSecondCheckReject) Validate(context.Context, string) error {
	v.calls++
	if v.calls == 2 {
		return errors.New("SECRET_VALIDATOR")
	}
	return nil
}
func TestImportRealOrchestratorFailureRetainsSlotAndEnvelope(t *testing.T) {
	dir := t.TempDir()
	st := NewStore(dir)
	p := sampleProfile()
	if err := st.Save(p); err != nil {
		t.Fatal(err)
	}
	orch := orchestrator.NewWithAppliedPath(filepath.Join(dir, "config"), nil, filepath.Join(dir, "applied.json"))
	t.Cleanup(orch.Close)
	for _, meta := range orchestrator.KnownSlots() {
		if err := orch.Register(meta); err != nil {
			t.Fatal(err)
		}
	}
	if err := orch.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	release := orch.HoldReloads()
	defer release()
	s := NewService(st, orch)
	if err := s.Sync(); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(st.path)
	oldslot, enabled, err := orch.SnapshotSlot(orchestrator.SlotWARP)
	if err != nil || !enabled {
		t.Fatal("initial slot missing")
	}
	validator := &importSecondCheckReject{}
	orch.SetValidator(validator)
	p.MTU = 1400
	if s.Import(p) == nil {
		t.Fatal("second validation rejection accepted")
	}
	after, _ := os.ReadFile(st.path)
	newslot, _, err := orch.SnapshotSlot(orchestrator.SlotWARP)
	if err != nil || !bytes.Equal(oldslot, newslot) || !bytes.Equal(before, after) || validator.calls != 2 {
		t.Fatal("rejected commit changed prior resources")
	}
	cfg, err := s.WARPConfig()
	if err != nil || cfg == nil || cfg.MTU != 1280 {
		t.Fatal("prior provider unavailable")
	}
	orch.SetValidator(nil)
	if err = s.Import(p); err != nil {
		t.Fatal("valid retry failed")
	}
	cfg, err = s.WARPConfig()
	if err != nil || cfg == nil || cfg.MTU != 1400 {
		t.Fatal("valid retry did not update provider")
	}
}

func TestImportRollbackDirectorySyncFailurePoisonsProvider(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "first", true: "replacement"}[existing], func(t *testing.T) {
			dir := t.TempDir()
			st := NewStore(dir)
			p := sampleProfile()
			if existing {
				if err := st.Save(p); err != nil {
					t.Fatal(err)
				}
			}
			o := &importOrch{}
			s := NewService(st, o)
			if err := s.Sync(); err != nil {
				t.Fatal(err)
			}
			o.err = errors.New("apply refusal")
			syncCalls := 0
			st.syncDirectory = func(got string) error {
				syncCalls++
				if got != dir {
					t.Fatal("wrong directory synced")
				}
				if !existing {
					if _, err := os.Stat(st.path); !os.IsNotExist(err) {
						t.Fatal("sync occurred before removal")
					}
				}
				return errors.New("directory flush refused")
			}
			p.MTU = 1400
			err := s.Import(p)
			if !errors.Is(err, ErrRollbackFailed) || syncCalls != 1 {
				t.Fatal("rollback durability failure not reported")
			}
			if _, err := s.WARPConfig(); err == nil {
				t.Fatal("uncertain rollback provider remained usable")
			}
		})
	}
}
func TestImportRollbackDirectorySyncSuccess(t *testing.T) {
	st := NewStore(t.TempDir())
	calls := 0
	st.syncDirectory = func(dir string) error {
		calls++
		if _, err := os.Stat(st.path); !os.IsNotExist(err) {
			t.Fatal("profile still present before directory sync")
		}
		return syncProfileDirectory(dir)
	}
	err := st.saveAndApply(sampleProfile(), func() error { return errors.New("apply") })
	if err == nil || errors.Is(err, ErrRollbackFailed) || calls != 1 {
		t.Fatal("first rollback did not confirm directory durability")
	}
}
func TestProfileDirectorySyncReturnsFilesystemFailure(t *testing.T) {
	if err := syncProfileDirectory(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := syncProfileDirectory(filepath.Join(t.TempDir(), "absent")); err == nil {
		t.Fatal("missing directory flush reported success")
	}
}
