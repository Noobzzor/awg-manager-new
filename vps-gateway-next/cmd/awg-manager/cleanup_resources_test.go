package main

import (
	"context"
	"errors"
	"github.com/hoaxisr/awg-manager/internal/storage"
	"reflect"
	"testing"
)

type cleanupResourceStub struct {
	calls    *[]string
	failures map[string]error
}

func (s cleanupResourceStub) call(name string) error {
	*s.calls = append(*s.calls, name)
	return s.failures[name]
}
func (s cleanupResourceStub) Cleanup(context.Context) error        { return s.call("singbox") }
func (s cleanupResourceStub) CleanupAll(context.Context) error     { return s.call("auxiliary") }
func (s cleanupResourceStub) DeleteIfExists(context.Context) error { return s.call("managed") }
func (s cleanupResourceStub) Save(context.Context) error           { return s.call("save") }
func (s cleanupResourceStub) Delete(context.Context, string) error { return s.call("delete") }
func (s cleanupResourceStub) List() ([]storage.AWGTunnel, error) {
	err := s.call("list")
	return []storage.AWGTunnel{{ID: "fixture"}}, err
}

func TestCleanupResourcesRetainsAllFailuresAndOrder(t *testing.T) {
	var calls []string
	failures := map[string]error{}
	for _, name := range []string{"singbox", "delete", "auxiliary", "managed", "save"} {
		failures[name] = errors.New(name + " failed")
	}
	s := cleanupResourceStub{&calls, failures}
	err := cleanupResources(t.Context(), s, s, s, s, s, s, s, s)
	want := []string{"singbox", "list", "delete", "auxiliary", "auxiliary", "managed", "auxiliary", "save"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
	for _, failure := range failures {
		if !errors.Is(err, failure) {
			t.Errorf("error=%v lost %v", err, failure)
		}
	}
}

func TestCleanupResourcesListFailureStillRunsAuxiliaryStages(t *testing.T) {
	var calls []string
	failure := errors.New("list failed")
	s := cleanupResourceStub{&calls, map[string]error{"list": failure}}
	err := cleanupResources(t.Context(), s, s, s, s, s, s, nil, s)
	if !errors.Is(err, failure) {
		t.Fatalf("error=%v lost list failure", err)
	}
	if want := []string{"list", "auxiliary", "auxiliary", "managed", "auxiliary", "save"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
}
