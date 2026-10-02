package query

import (
	"context"
	"testing"
)

func TestRunningConfigStore_GetInterfaceHotspotPolicy(t *testing.T) {
	fg := newFakeGetter()
	fg.SetRaw("/show/running-config", []byte(`{"message":[
		"!",
		"ip hotspot",
		" policy Home Policy0",
		" policy Wireguard0 Policy1",
		"!"
	]}`))
	s := NewRunningConfigStore(fg, NopLogger())

	policy, err := s.GetInterfaceHotspotPolicy(context.Background(), "Wireguard0")
	if err != nil {
		t.Fatalf("GetInterfaceHotspotPolicy: %v", err)
	}
	if policy != "Policy1" {
		t.Fatalf("policy = %q, want Policy1", policy)
	}

	none, err := s.GetInterfaceHotspotPolicy(context.Background(), "Wireguard9")
	if err != nil {
		t.Fatalf("GetInterfaceHotspotPolicy missing: %v", err)
	}
	if none != "none" {
		t.Fatalf("missing policy = %q, want none", none)
	}
}
