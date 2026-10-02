package main

import (
	"errors"
	"reflect"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/cleanup"
)

func TestCleanupStagesBootstrapFailureContinuesIndependentStages(t *testing.T) {
	failure := errors.New("bootstrap failed")
	var called []string
	var failedCleaner *cleanupResourceStub
	err := runCleanupStages(func() (cleanup.SingboxCleaner, error) { return failedCleaner, failure }, func(c cleanup.SingboxCleaner) error {
		if c != nil {
			t.Fatal("failed bootstrap supplied a cleaner (possibly typed nil)")
		}
		called = append(called, "resources")
		// Exercise the actual service's nil-cleaner contract with no effects.
		return cleanup.New(nil, nil, nil, nil, nil, nil, c, nil).CleanupAll(t.Context())
	}, func() error { called = append(called, "policy-tun"); return nil }, func() error { called = append(called, "files"); return nil })
	if !reflect.DeepEqual(called, []string{"resources", "policy-tun", "files"}) {
		t.Fatalf("independent stages=%v", called)
	}
	if !errors.Is(err, failure) {
		t.Fatalf("error=%v, want bootstrap failure", err)
	}
}

func TestCleanupStagesJoinsFailuresWithoutSkippingStages(t *testing.T) {
	failures := []error{errors.New("resources"), errors.New("policy-tun"), errors.New("files")}
	var called []string
	err := runCleanupStages(func() (cleanup.SingboxCleaner, error) { return nil, nil }, func(cleanup.SingboxCleaner) error { called = append(called, "resources"); return failures[0] }, func() error { called = append(called, "policy-tun"); return failures[1] }, func() error { called = append(called, "files"); return failures[2] })
	if len(called) != 3 {
		t.Fatalf("stages=%v", called)
	}
	for _, failure := range failures {
		if !errors.Is(err, failure) {
			t.Errorf("error=%v lost %v", err, failure)
		}
	}
}
