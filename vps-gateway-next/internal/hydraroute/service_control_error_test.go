package hydraroute

import (
	"strings"
	"testing"
)

func TestControl_UnknownActionStoresLastError(t *testing.T) {
	s := &Service{
		status: Status{
			Installed: true,
			Running:   false,
		},
	}

	err := s.Control("bad-action")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	got := s.GetStatus()
	if !strings.Contains(got.LastError, "unknown action: bad-action") {
		t.Fatalf("LastError=%q, want contains %q", got.LastError, "unknown action: bad-action")
	}
}

func TestControl_NotInstalledStoresLastError(t *testing.T) {
	s := &Service{
		status: Status{
			Installed: false,
			Running:   false,
		},
	}

	err := s.Control("start")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	got := s.GetStatus()
	if !strings.Contains(got.LastError, "HydraRoute Neo is not installed") {
		t.Fatalf("LastError=%q, want contains %q", got.LastError, "HydraRoute Neo is not installed")
	}
}
