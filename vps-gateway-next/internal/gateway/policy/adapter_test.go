package policy

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

type recordingSlotSaver struct {
	slot    orchestrator.Slot
	data    []byte
	enabled bool
}

func (s *recordingSlotSaver) SaveAndValidate(slot orchestrator.Slot, data []byte) (orchestrator.ValidationResult, error) {
	s.slot = slot
	s.data = append([]byte(nil), data...)
	return orchestrator.ValidationResult{}, nil
}

func (s *recordingSlotSaver) SetEnabled(slot orchestrator.Slot, enabled bool) error {
	if slot != s.slot {
		return fmt.Errorf("enabled slot %q before saving it", slot)
	}
	s.enabled = enabled
	return nil
}

func (s *recordingSlotSaver) SnapshotSlot(_ orchestrator.Slot) ([]byte, bool, error) {
	return append([]byte(nil), s.data...), s.enabled, nil
}

func (s *recordingSlotSaver) RestoreSlot(_ orchestrator.Slot, data []byte, enabled bool) error {
	s.data = append([]byte(nil), data...)
	s.enabled = enabled
	return nil
}

func TestApplyToSlotUsesGatewayPolicySlot(t *testing.T) {
	compiled, err := Compile(Profile{
		ID:            "p",
		Name:          "P",
		DefaultAction: ActionDirect,
	}, CompileOptions{
		Outbounds:        map[string]struct{}{"direct": {}},
		OutboundActions:  map[string]Action{"direct": ActionDirect},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	saver := new(recordingSlotSaver)
	if err := ApplyToSlot(saver, compiled); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if saver.slot != orchestrator.SlotGatewayPolicy {
		t.Fatalf("saved slot = %q, want %q", saver.slot, orchestrator.SlotGatewayPolicy)
	}
	if !saver.enabled {
		t.Fatal("adapter did not enable the validated gateway policy slot")
	}
	if len(saver.data) == 0 {
		t.Fatal("adapter saved empty slot")
	}
}

type transactionalSlotSaver struct {
	data        []byte
	enabled     bool
	activation  error
	restoreErr  error
	snapshotErr error
}

func (s *transactionalSlotSaver) SaveAndValidate(_ orchestrator.Slot, data []byte) (orchestrator.ValidationResult, error) {
	s.data = append([]byte(nil), data...)
	return orchestrator.ValidationResult{}, nil
}

func (s *transactionalSlotSaver) SetEnabled(_ orchestrator.Slot, enabled bool) error {
	if enabled && s.activation != nil {
		return s.activation
	}
	s.enabled = enabled
	return nil
}

func (s *transactionalSlotSaver) SnapshotSlot(_ orchestrator.Slot) ([]byte, bool, error) {
	return append([]byte(nil), s.data...), s.enabled, s.snapshotErr
}

func (s *transactionalSlotSaver) RestoreSlot(_ orchestrator.Slot, data []byte, enabled bool) error {
	if s.restoreErr != nil {
		return s.restoreErr
	}
	s.data = append([]byte(nil), data...)
	s.enabled = enabled
	return nil
}

func TestApplyToSlotRestoresPreviousSlotWhenActivationFails(t *testing.T) {
	activationErr := errors.New("activation failed")
	prior := []byte(`{"prior":true}`)
	saver := &transactionalSlotSaver{data: append([]byte(nil), prior...), enabled: false, activation: activationErr}
	compiled, err := Compile(Profile{ID: "p", Name: "P", DefaultAction: ActionDirect}, CompileOptions{
		Outbounds: map[string]struct{}{"direct": {}}, OutboundActions: map[string]Action{"direct": ActionDirect},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = ApplyToSlot(saver, compiled)
	if !errors.Is(err, activationErr) {
		t.Fatalf("ApplyToSlot error = %v, want activation error %v", err, activationErr)
	}
	if string(saver.data) != string(prior) || saver.enabled {
		t.Fatalf("slot after failed activation = (%q, enabled=%t), want (%q, enabled=false)", saver.data, saver.enabled, prior)
	}
}

func TestApplyToSlotReportsActivationAndRollbackFailures(t *testing.T) {
	activationErr := errors.New("activation failed")
	restoreErr := errors.New("rollback failed")
	saver := &transactionalSlotSaver{data: []byte(`{"prior":true}`), activation: activationErr, restoreErr: restoreErr}
	compiled, err := Compile(Profile{ID: "p", Name: "P", DefaultAction: ActionDirect}, CompileOptions{
		Outbounds: map[string]struct{}{"direct": {}}, OutboundActions: map[string]Action{"direct": ActionDirect},
		DefaultOutbounds: map[Action]string{ActionDirect: "direct"},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = ApplyToSlot(saver, compiled)
	if !errors.Is(err, activationErr) || !errors.Is(err, restoreErr) || !strings.Contains(err.Error(), "rollback") {
		t.Fatalf("ApplyToSlot error = %v, want combined activation and rollback errors", err)
	}
}
