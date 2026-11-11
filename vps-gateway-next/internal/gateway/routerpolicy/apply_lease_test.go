package routerpolicy

import (
	"context"
	"errors"
	"testing"
)

// This backend has an independently scheduled engine. Every operation must
// remain protected, including journal recovery and cancellation rollback.
type leasedApplyFixture struct {
	*applyBackendFixture
	t                      *testing.T
	held                   bool
	acquisitions, releases int
	acquireError           error
	nilRelease             bool
	recoveryRequired       bool
	observations           int
}

func (b *leasedApplyFixture) SetRecoveryRequired(required bool) {
	b.check()
	b.recoveryRequired = required
	b.observations++
}

func (b *leasedApplyFixture) AcquireApplyLease(ctx context.Context) (func(), error) {
	b.acquisitions++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b.acquireError != nil {
		return nil, b.acquireError
	}
	if b.nilRelease {
		return nil, nil
	}
	if b.held {
		b.t.Fatal("nested runtime lease acquisition")
	}
	b.held = true
	return func() {
		if !b.held {
			b.t.Fatal("runtime lease released twice")
		}
		b.held = false
		b.releases++
	}, nil
}
func (b *leasedApplyFixture) check() {
	b.t.Helper()
	if !b.held {
		b.t.Fatal("backend operation outside runtime lease")
	}
}
func (b *leasedApplyFixture) Validate(c context.Context, s ApplyState) error {
	b.check()
	return b.applyBackendFixture.Validate(c, s)
}
func (b *leasedApplyFixture) Snapshot(c context.Context) (ApplyState, error) {
	b.check()
	return b.applyBackendFixture.Snapshot(c)
}
func (b *leasedApplyFixture) Commit(c context.Context, s ApplyState) error {
	b.check()
	return b.applyBackendFixture.Commit(c, s)
}
func (b *leasedApplyFixture) Activate(c context.Context) error {
	b.check()
	return b.applyBackendFixture.Activate(c)
}
func (b *leasedApplyFixture) Confirm(c context.Context, s ApplyState) error {
	b.check()
	return b.applyBackendFixture.Confirm(c, s)
}
func (b *leasedApplyFixture) Restore(c context.Context, s ApplyState) error {
	b.check()
	return b.applyBackendFixture.Restore(c, s)
}

func TestApplyLeaseSpansTransactionAndRollback(t *testing.T) {
	for _, stage := range []string{"", "validate", "snapshot", "commit", "activate", "confirm"} {
		t.Run("stage_"+stage, func(t *testing.T) {
			a, base, _, _, next := applyFixture(t)
			b := &leasedApplyFixture{applyBackendFixture: base, t: t}
			a.backend = b
			base.fail = stage
			out, err := a.Apply(context.Background(), next)
			if stage == "" {
				if err != nil || !out.Applied {
					t.Fatalf("apply: %+v %v", out, err)
				}
			} else if !errors.Is(err, fixtureApplyError) || out.Applied {
				t.Fatalf("failure lost: %+v %v", out, err)
			}
			if b.held || b.acquisitions != 1 || b.releases != 1 {
				t.Fatalf("lease leaked: %+v", b)
			}
		})
	}
}

func TestApplyLeaseSpansBothRecoveryPhases(t *testing.T) {
	for _, phase := range []string{"prepared", "accepted"} {
		t.Run(phase, func(t *testing.T) {
			a, base, _, old, next := applyFixture(t)
			if err := a.journal.Store(applyJournal{Version: 1, Phase: phase, Before: old, After: next}); err != nil {
				t.Fatal(err)
			}
			b := &leasedApplyFixture{applyBackendFixture: base, t: t}
			a.backend = b
			if err := a.Recover(context.Background()); err != nil {
				t.Fatal(err)
			}
			if b.held || b.acquisitions != 1 || b.releases != 1 {
				t.Fatal("recovery did not release runtime lease")
			}
		})
	}
}

func TestApplyLeaseRefusalPreventsMutation(t *testing.T) {
	for _, recovery := range []bool{false, true} {
		t.Run(map[bool]string{false: "apply", true: "recovery"}[recovery], func(t *testing.T) {
			a, base, _, old, next := applyFixture(t)
			if recovery {
				if err := a.journal.Store(applyJournal{Version: 1, Phase: "prepared", Before: old, After: next}); err != nil {
					t.Fatal(err)
				}
			}
			b := &leasedApplyFixture{applyBackendFixture: base, t: t, acquireError: fixtureApplyError}
			a.backend = b
			var err error
			if recovery {
				err = a.Recover(context.Background())
			} else {
				_, err = a.Apply(context.Background(), next)
			}
			if !errors.Is(err, fixtureApplyError) || base.commits != 0 || base.restores != 0 || base.activations != 0 {
				t.Fatalf("refused lease allowed work: %v", err)
			}
		})
	}
}

func TestApplyLeaseRejectsMissingRelease(t *testing.T) {
	a, base, _, _, next := applyFixture(t)
	b := &leasedApplyFixture{applyBackendFixture: base, t: t, nilRelease: true}
	a.backend = b
	if _, err := a.Apply(context.Background(), next); err == nil {
		t.Fatal("missing lease accepted")
	}
}

func TestApplyLeaseReleasedOnJournalFailureAndPanic(t *testing.T) {
	for _, stage := range []string{"accepted_write", "commit_panic"} {
		t.Run(stage, func(t *testing.T) {
			a, base, _, _, next := applyFixture(t)
			b := &leasedApplyFixture{applyBackendFixture: base, t: t}
			a.backend = b
			if stage == "accepted_write" {
				a.journal = journalStoreFailure{applyJournalStore: a.journal}
			} else {
				base.crashCommit = true
			}
			func() { defer func() { _ = recover() }(); _, _ = a.Apply(context.Background(), next) }()
			if b.held || b.acquisitions != 1 || b.releases != 1 {
				t.Fatal("runtime lease leaked after interruption")
			}
		})
	}
}

func TestApplyLeaseReportsPendingRecoveryBeforeRelease(t *testing.T) {
	a, base, _, old, next := applyFixture(t)
	b := &leasedApplyFixture{applyBackendFixture: base, t: t}
	a.backend = b
	if err := a.journal.Store(applyJournal{Version: 1, Phase: "prepared", Before: old, After: next}); err != nil {
		t.Fatal(err)
	}
	base.fail = "restore"
	if err := a.Recover(context.Background()); !errors.Is(err, ErrApplyRecoveryRequired) {
		t.Fatal("failed recovery accepted", err)
	}
	if !b.recoveryRequired || b.observations != 1 || b.held {
		t.Fatal("recovery barrier not retained before release")
	}
	if err := a.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b.recoveryRequired || b.observations != 2 || b.held {
		t.Fatal("confirmed recovery did not clear barrier")
	}
}

type uncertainClearJournal struct{ applyJournalStore }

func (j uncertainClearJournal) Clear() error {
	if err := j.applyJournalStore.Clear(); err != nil {
		return err
	}
	return fixtureApplyError
}
func TestApplyLeaseRetainsBarrierForUncertainJournalDeletion(t *testing.T) {
	a, base, _, _, next := applyFixture(t)
	b := &leasedApplyFixture{applyBackendFixture: base, t: t}
	a.backend = b
	original := a.journal
	a.journal = uncertainClearJournal{applyJournalStore: original}
	out, err := a.Apply(context.Background(), next)
	if !out.Applied || !out.CleanupPending || !errors.Is(err, ErrApplyRecoveryRequired) || !b.recoveryRequired {
		t.Fatal("uncertain durable deletion cleared recovery barrier", err)
	}
	a.journal = original
	if err := a.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b.recoveryRequired {
		t.Fatal("successful recovery retained barrier")
	}
}
