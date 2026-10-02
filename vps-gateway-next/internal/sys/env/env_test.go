package env

import (
	"os"
	"testing"
	"time"
)

func TestIntDefault_Unset(t *testing.T) {
	os.Unsetenv("AWG_TEST_INT")
	if got := IntDefault("AWG_TEST_INT", 42); got != 42 {
		t.Errorf("unset: want 42, got %d", got)
	}
}

func TestIntDefault_Valid(t *testing.T) {
	t.Setenv("AWG_TEST_INT", "7")
	if got := IntDefault("AWG_TEST_INT", 42); got != 7 {
		t.Errorf("valid: want 7, got %d", got)
	}
}

func TestIntDefault_NonNumeric(t *testing.T) {
	t.Setenv("AWG_TEST_INT", "abc")
	if got := IntDefault("AWG_TEST_INT", 42); got != 42 {
		t.Errorf("non-numeric: want default 42, got %d", got)
	}
}

func TestIntDefault_Zero(t *testing.T) {
	t.Setenv("AWG_TEST_INT", "0")
	if got := IntDefault("AWG_TEST_INT", 42); got != 42 {
		t.Errorf("zero: want default 42, got %d", got)
	}
}

func TestIntDefault_Negative(t *testing.T) {
	t.Setenv("AWG_TEST_INT", "-5")
	if got := IntDefault("AWG_TEST_INT", 42); got != 42 {
		t.Errorf("negative: want default 42, got %d", got)
	}
}

func TestIntDefault_Empty(t *testing.T) {
	t.Setenv("AWG_TEST_INT", "")
	if got := IntDefault("AWG_TEST_INT", 42); got != 42 {
		t.Errorf("empty: want default 42, got %d", got)
	}
}

func TestDurationDefault_Unset(t *testing.T) {
	os.Unsetenv("AWG_TEST_DUR")
	want := 2 * time.Second
	if got := DurationDefault("AWG_TEST_DUR", want); got != want {
		t.Errorf("unset: want %s, got %s", want, got)
	}
}

func TestDurationDefault_Valid(t *testing.T) {
	t.Setenv("AWG_TEST_DUR", "500ms")
	want := 500 * time.Millisecond
	if got := DurationDefault("AWG_TEST_DUR", 2*time.Second); got != want {
		t.Errorf("valid: want %s, got %s", want, got)
	}
}

func TestDurationDefault_Zero(t *testing.T) {
	t.Setenv("AWG_TEST_DUR", "0s")
	if got := DurationDefault("AWG_TEST_DUR", 2*time.Second); got != 0 {
		t.Errorf("zero: want 0 (feature off), got %s", got)
	}
}

func TestDurationDefault_Invalid(t *testing.T) {
	t.Setenv("AWG_TEST_DUR", "notduration")
	want := 2 * time.Second
	if got := DurationDefault("AWG_TEST_DUR", want); got != want {
		t.Errorf("invalid: want default %s, got %s", want, got)
	}
}

func TestDurationDefault_Negative(t *testing.T) {
	t.Setenv("AWG_TEST_DUR", "-3s")
	want := 2 * time.Second
	if got := DurationDefault("AWG_TEST_DUR", want); got != want {
		t.Errorf("negative: want default %s, got %s", want, got)
	}
}

func TestDurationDefault_Empty(t *testing.T) {
	t.Setenv("AWG_TEST_DUR", "")
	want := 2 * time.Second
	if got := DurationDefault("AWG_TEST_DUR", want); got != want {
		t.Errorf("empty: want default %s, got %s", want, got)
	}
}
