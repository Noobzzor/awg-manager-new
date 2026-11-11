package routerpolicy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

var fixtureApplyError = errors.New("runtime fixture refused operation")

func TestJournalRequiresDirectoryDurability(t *testing.T) {
	old, next := applyStates()
	path := filepath.Join(t.TempDir(), "router-apply.json")
	j := fileApplyJournal{path: path, syncParent: func(string) error { return fixtureApplyError }}
	err := j.Store(applyJournal{Version: 1, Phase: "prepared", Before: old, After: next})
	if !errors.Is(err, fixtureApplyError) {
		t.Fatalf("directory sync failure was acknowledged: %v", err)
	}
	// Replacement may already have happened. The caller must keep this
	// recoverable state and must not commit the candidate after this error.
	if recovered, err := j.Load(); err != nil || recovered == nil {
		t.Fatalf("recovery journal unavailable: %v", err)
	}
}

type applyBackendFixture struct {
	disk, live                     ApplyState
	fail                           string
	commits, restores, activations int
	onCommit                       func()
	crashCommit                    bool
}

func (b *applyBackendFixture) failure(stage string) error {
	if b.fail == stage {
		b.fail = ""
		return fixtureApplyError
	}
	return nil
}
func (b *applyBackendFixture) Validate(context.Context, ApplyState) error {
	return b.failure("validate")
}
func (b *applyBackendFixture) Snapshot(context.Context) (ApplyState, error) {
	if err := b.failure("snapshot"); err != nil {
		return ApplyState{}, err
	}
	return b.disk, nil
}
func (b *applyBackendFixture) Commit(_ context.Context, s ApplyState) error {
	b.commits++
	if b.onCommit != nil {
		b.onCommit()
	}
	b.disk = s
	if b.crashCommit {
		panic("simulated process loss after partial commit")
	}
	return b.failure("commit")
}
func (b *applyBackendFixture) Activate(context.Context) error {
	b.activations++
	b.live = b.disk
	return b.failure("activate")
}
func (b *applyBackendFixture) Confirm(_ context.Context, s ApplyState) error {
	if err := b.failure("confirm"); err != nil {
		return err
	}
	if !reflect.DeepEqual(b.live, s) {
		return fixtureApplyError
	}
	return nil
}
func (b *applyBackendFixture) Restore(_ context.Context, s ApplyState) error {
	b.restores++
	if err := b.failure("restore"); err != nil {
		return err
	}
	b.disk = s
	return nil
}

func applyStates() (ApplyState, ApplyState) {
	return ApplyState{Editor: []byte(`{"route":{"final":"direct"}}`), Runtime: []byte(`{"route":{"rules":[{"outbound":"direct"}]}}`), Enabled: true}, ApplyState{Editor: []byte(`{"route":{"final":"block"}}`), Runtime: []byte(`{"route":{"rules":[{"action":"reject"}]}}`), Enabled: true}
}
func applyFixture(t *testing.T) (*Applier, *applyBackendFixture, string, ApplyState, ApplyState) {
	t.Helper()
	old, next := applyStates()
	b := &applyBackendFixture{disk: old, live: old}
	path := filepath.Join(t.TempDir(), "router-apply.json")
	a, err := NewApplier(path, b, &sync.Mutex{})
	if err != nil {
		t.Fatal(err)
	}
	return a, b, path, old, next
}

func TestConfirmedApplyRejectsEngineFailure(t *testing.T) {
	for _, stage := range []string{"validate", "snapshot", "commit", "activate", "confirm"} {
		t.Run(stage, func(t *testing.T) {
			a, b, path, old, next := applyFixture(t)
			b.fail = stage
			out, err := a.Apply(context.Background(), next)
			if !errors.Is(err, fixtureApplyError) || out.Applied {
				t.Fatalf("failed %s acknowledged as applied: %+v %v", stage, out, err)
			}
			if !reflect.DeepEqual(b.disk, old) || !reflect.DeepEqual(b.live, old) {
				t.Fatal("previous working state was not restored")
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("completed rollback left journal", err)
			}
			if stage == "validate" && (b.commits != 0 || b.activations != 0) {
				t.Fatal("invalid candidate touched runtime")
			}
		})
	}
}

func TestConfirmedApplyJournalsBeforeMutation(t *testing.T) {
	a, b, path, _, next := applyFixture(t)
	b.onCommit = func() {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal("runtime changed before durable journal", err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatal("journal may contain secrets and must be0600")
		}
	}
	out, err := a.Apply(context.Background(), next)
	if err != nil || !out.Applied || out.CleanupPending {
		t.Fatalf("apply outcome=%+v err=%v", out, err)
	}
	if b.commits != 1 || b.activations != 1 || !reflect.DeepEqual(b.live, next) {
		t.Fatal("no confirmed runtime activation")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("successful apply left journal", err)
	}
}

func TestPreparedApplyRecoversAfterFreshProcess(t *testing.T) {
	a, b, path, old, next := applyFixture(t)
	b.crashCommit = true
	crashed := false
	func() {
		defer func() {
			if recover() != nil {
				crashed = true
			}
		}()
		_, _ = a.Apply(context.Background(), next)
	}()
	if !crashed {
		t.Fatal("interruption fixture did not run")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("interruption lost recovery journal", err)
	}
	b.crashCommit = false
	fresh, err := NewApplier(path, b, &sync.Mutex{})
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.disk, old) || !reflect.DeepEqual(b.live, old) {
		t.Fatal("fresh process did not recover previous state")
	}
}

type journalFixture struct {
	applyJournalStore
	afterStore func(applyJournal)
	clearErr   bool
}

func (j journalFixture) Store(v applyJournal) error {
	if err := j.applyJournalStore.Store(v); err != nil {
		return err
	}
	if j.afterStore != nil {
		j.afterStore(v)
	}
	return nil
}
func (j journalFixture) Clear() error {
	if j.clearErr {
		return fixtureApplyError
	}
	return j.applyJournalStore.Clear()
}

func TestAcceptedApplySurvivesCleanupInterruption(t *testing.T) {
	a, b, path, _, next := applyFixture(t)
	a.journal = journalFixture{applyJournalStore: a.journal, clearErr: true}
	out, err := a.Apply(context.Background(), next)
	if !out.Applied || !out.CleanupPending || err == nil {
		t.Fatalf("accepted/cleanup state lost: %+v %v", out, err)
	}
	fresh, e := NewApplier(path, b, &sync.Mutex{})
	if e != nil {
		t.Fatal(e)
	}
	if err := fresh.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.live, next) {
		t.Fatal("acknowledged candidate was rolled back on restart")
	}
}

func TestFailedRecoveryKeepsJournalAndBlocksNextApply(t *testing.T) {
	a, b, path, _, next := applyFixture(t)
	b.crashCommit = true
	func() { defer func() { _ = recover() }(); _, _ = a.Apply(context.Background(), next) }()
	b.crashCommit = false
	b.fail = "restore"
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(context.Background(), next); err == nil {
		t.Fatal("failed recovery allowed another apply")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("failed recovery removed or replaced the journal")
	}
}

func TestCorruptJournalNeverMutatesRuntime(t *testing.T) {
	a, b, path, old, next := applyFixture(t)
	if err := os.WriteFile(path, []byte(`{"version":1,"phase":"accepted"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Apply(context.Background(), next); err == nil {
		t.Fatal("corrupt/incomplete journal accepted")
	}
	if b.commits != 0 || b.restores != 0 || b.activations != 0 || !reflect.DeepEqual(b.disk, old) {
		t.Fatal("corrupt journal changed runtime")
	}
}

func TestEmptyExistingFileIsNotAnAbsentSnapshot(t *testing.T) {
	a, b, _, _, next := applyFixture(t)
	b.disk.Editor = []byte{}
	if out, err := a.Apply(context.Background(), next); err == nil || out.Applied {
		t.Fatal("empty existing editor file silently became an absent snapshot")
	}
	if b.commits != 0 || b.activations != 0 {
		t.Fatal("invalid rollback snapshot allowed mutation")
	}
}

func TestJournalChecksumAndFileTypeGuards(t *testing.T) {
	for _, kind := range []string{"checksum", "permissions", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			a, b, path, old, next := applyFixture(t)
			if err := a.journal.Store(applyJournal{Version: 1, Phase: "prepared", Before: old, After: next}); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "checksum":
				raw, _ := os.ReadFile(path)
				var envelope map[string]any
				if err := json.Unmarshal(raw, &envelope); err != nil {
					t.Fatal(err)
				}
				envelope["state"].(map[string]any)["phase"] = "accepted"
				raw, _ = json.Marshal(envelope)
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
			case "permissions":
				if err := os.Chmod(path, 0644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				other := filepath.Join(filepath.Dir(path), "protected-fixture.json")
				if err := os.Rename(path, other); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(other, path); err != nil {
					t.Fatal(err)
				}
			}
			if err := a.Recover(context.Background()); err == nil {
				t.Fatal("untrusted recovery journal accepted")
			}
			if b.commits != 0 || b.restores != 0 || b.activations != 0 {
				t.Fatal("untrusted journal touched runtime")
			}
		})
	}
}

type cancelBackendFixture struct {
	*applyBackendFixture
	cancel               context.CancelFunc
	cancelled            bool
	rollbackContextValid bool
}

func (b *cancelBackendFixture) Activate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.cancelled {
		b.rollbackContextValid = true
	}
	err := b.applyBackendFixture.Activate(ctx)
	if !b.cancelled {
		b.cancelled = true
		b.cancel()
	}
	return err
}

func TestCancelledApplyStillRestoresWithLiveContext(t *testing.T) {
	a, backend, _, old, next := applyFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := &cancelBackendFixture{applyBackendFixture: backend, cancel: cancel}
	a.backend = b
	out, err := a.Apply(ctx, next)
	if !errors.Is(err, context.Canceled) || out.Applied {
		t.Fatalf("cancelled apply=%+v %v", out, err)
	}
	if !b.rollbackContextValid || !reflect.DeepEqual(backend.live, old) {
		t.Fatal("request cancellation cancelled rollback")
	}
}

func TestAcceptedMarkerWriteFailureIsExplicitlyUncertain(t *testing.T) {
	a, b, path, _, next := applyFixture(t)
	a.journal = journalStoreFailure{applyJournalStore: a.journal}
	out, err := a.Apply(context.Background(), next)
	if !out.Applied || !out.CleanupPending || !errors.Is(err, ErrApplyRecoveryRequired) {
		t.Fatalf("acceptance uncertainty hidden: %+v %v", out, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("uncertain acceptance discarded recovery journal")
	}
	if !reflect.DeepEqual(b.live, next) {
		t.Fatal("confirmed candidate state was misreported")
	}
}

type journalStoreFailure struct{ applyJournalStore }

func (j journalStoreFailure) Store(v applyJournal) error {
	if v.Phase == "accepted" {
		return fixtureApplyError
	}
	return j.applyJournalStore.Store(v)
}

func TestAcceptedJournalRecoversAfterProcessLoss(t *testing.T) {
	a, b, path, _, next := applyFixture(t)
	a.journal = journalFixture{applyJournalStore: a.journal, afterStore: func(j applyJournal) {
		if j.Phase == "accepted" {
			panic("simulated process loss after acceptance")
		}
	}}
	crashed := false
	func() {
		defer func() {
			if recover() != nil {
				crashed = true
			}
		}()
		_, _ = a.Apply(context.Background(), next)
	}()
	if !crashed {
		t.Fatal("acceptance interruption fixture did not run")
	}
	fresh, err := NewApplier(path, b, &sync.Mutex{})
	if err != nil {
		t.Fatal(err)
	}
	if err := fresh.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.disk, next) || !reflect.DeepEqual(b.live, next) {
		t.Fatal("accepted state was lost after process restart")
	}
}

type observedApplyGuard struct {
	mu       sync.Mutex
	attempts chan struct{}
}

func (g *observedApplyGuard) Lock()   { g.attempts <- struct{}{}; g.mu.Lock() }
func (g *observedApplyGuard) Unlock() { g.mu.Unlock() }

func TestAppliersShareCoordinatorAcrossInstances(t *testing.T) {
	_, b, path, _, next := applyFixture(t)
	g := &observedApplyGuard{attempts: make(chan struct{}, 2)}
	a, err := NewApplier(path, b, g)
	if err != nil {
		t.Fatal(err)
	}
	other, err := NewApplier(path, b, g)
	if err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	b.onCommit = func() { once.Do(func() { close(entered); <-release }) }
	done := make(chan error, 2)
	go func() { _, err := a.Apply(context.Background(), next); done <- err }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first apply did not reach commit")
	}
	<-g.attempts
	go func() { _, err := other.Apply(context.Background(), next); done <- err }()
	select {
	case <-g.attempts:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("second apply did not use shared coordinator")
	}
	if b.commits != 1 {
		close(release)
		t.Fatal("second apply entered a transaction in progress")
	}
	close(release)
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("apply coordinator deadlocked")
		}
	}
	if b.commits != 2 || !reflect.DeepEqual(b.live, next) {
		t.Fatal("serialized apply did not finish")
	}
}
