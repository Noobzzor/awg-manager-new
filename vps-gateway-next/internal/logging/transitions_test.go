package logging

import "testing"

func TestTransitionTracker_Sequence(t *testing.T) {
	tr := NewTransitionTracker()

	steps := []struct {
		ok           bool
		wantKind     TransitionKind
		wantFailures int
	}{
		{true, TransitionFirstOK, 0},
		{true, TransitionStillOK, 0},
		{false, TransitionNowFailing, 1},
		{false, TransitionStillFailing, 2},
		{false, TransitionStillFailing, 3},
		{true, TransitionRecovered, 3},
		{true, TransitionStillOK, 0},
		{false, TransitionNowFailing, 1},
		{true, TransitionRecovered, 1},
	}
	for i, st := range steps {
		got := tr.Observe("t1", st.ok)
		if got.Kind != st.wantKind || got.Failures != st.wantFailures {
			t.Fatalf("step %d: got {%v %d}, want {%v %d}", i, got.Kind, got.Failures, st.wantKind, st.wantFailures)
		}
	}
}

func TestTransitionTracker_FirstResultIsFail(t *testing.T) {
	tr := NewTransitionTracker()
	got := tr.Observe("t1", false)
	if got.Kind != TransitionNowFailing || got.Failures != 1 {
		t.Fatalf("first fail: got {%v %d}, want {NowFailing 1}", got.Kind, got.Failures)
	}
}

func TestTransitionTracker_TargetsIndependent(t *testing.T) {
	tr := NewTransitionTracker()
	tr.Observe("a", false)
	got := tr.Observe("b", false)
	if got.Kind != TransitionNowFailing || got.Failures != 1 {
		t.Fatalf("target b must be independent: got {%v %d}", got.Kind, got.Failures)
	}
	if got := tr.Observe("a", false); got.Failures != 2 {
		t.Fatalf("target a streak: got %d, want 2", got.Failures)
	}
}

func TestTransitionTracker_Forget(t *testing.T) {
	tr := NewTransitionTracker()
	tr.Observe("t1", false)
	tr.Observe("t1", false)
	tr.Forget("t1")
	// После сброса отказ — снова «новый» (Warn), а не продолжение серии.
	if got := tr.Observe("t1", false); got.Kind != TransitionNowFailing || got.Failures != 1 {
		t.Fatalf("after Forget: got {%v %d}, want {NowFailing 1}", got.Kind, got.Failures)
	}
	tr.Forget("t1")
	// И успех после сброса — FirstOK, а не Recovered.
	if got := tr.Observe("t1", true); got.Kind != TransitionFirstOK {
		t.Fatalf("after Forget: got %v, want FirstOK", got.Kind)
	}
}
