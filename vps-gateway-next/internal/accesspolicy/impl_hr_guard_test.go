package accesspolicy

import (
	"context"
	"strings"
	"testing"
)

func TestDelete_RejectsHydraRoutePolicy(t *testing.T) {
	s := &ServiceImpl{}
	err := s.Delete(context.Background(), "HydraRoute")
	if err == nil {
		t.Fatal("expected error for HydraRoute policy delete")
	}
	if !strings.Contains(err.Error(), "HydraRoute") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAssignDevice_RejectsHydraRoutePolicy(t *testing.T) {
	s := &ServiceImpl{}
	err := s.AssignDevice(context.Background(), "aa:bb:cc:dd:ee:ff", "HydraRoute")
	if err == nil {
		t.Fatal("expected error assigning device to HydraRoute policy")
	}
	if !strings.Contains(err.Error(), "HydraRoute") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAssignDevice_RejectsCustomPolicyName(t *testing.T) {
	s := &ServiceImpl{}
	err := s.AssignDevice(context.Background(), "aa:bb:cc:dd:ee:ff", "germany-vpn")
	if err == nil {
		t.Fatal("expected error assigning device to custom policy")
	}
	if !strings.Contains(err.Error(), "germany-vpn") {
		t.Fatalf("unexpected error: %v", err)
	}
}
