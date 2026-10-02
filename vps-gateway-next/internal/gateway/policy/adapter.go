package policy

import (
	"errors"
	"fmt"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

// SlotSaver is the narrow validated-write contract of the sing-box
// orchestrator. A slot is made active only after its candidate has passed
// merged-config validation.
type SlotSaver interface {
	SaveAndValidate(slot orchestrator.Slot, jsonBytes []byte) (orchestrator.ValidationResult, error)
	SnapshotSlot(slot orchestrator.Slot) ([]byte, bool, error)
	RestoreSlot(slot orchestrator.Slot, jsonBytes []byte, enabled bool) error
	SetEnabled(slot orchestrator.Slot, enabled bool) error
}

// ApplyToSlot renders and hands the gateway policy to the orchestrator-owned
// slot. The candidate is validated before activation; the orchestrator owns
// file placement and schedules reloads after each successful state change.
func ApplyToSlot(saver SlotSaver, compiled CompiledPolicy) error {
	data, err := RenderSlot(compiled)
	if err != nil {
		return err
	}
	previousData, previousEnabled, err := saver.SnapshotSlot(orchestrator.SlotGatewayPolicy)
	if err != nil {
		return fmt.Errorf("snapshot gateway policy: %w", err)
	}
	result, err := saver.SaveAndValidate(orchestrator.SlotGatewayPolicy, data)
	if err != nil {
		return fmt.Errorf("validate gateway policy: %w", err)
	}
	if !result.Ok() {
		return fmt.Errorf("validate gateway policy: %s", result.Error())
	}
	if err := saver.SetEnabled(orchestrator.SlotGatewayPolicy, true); err != nil {
		activationErr := fmt.Errorf("enable gateway policy: %w", err)
		if rollbackErr := saver.RestoreSlot(orchestrator.SlotGatewayPolicy, previousData, previousEnabled); rollbackErr != nil {
			return errors.Join(activationErr, fmt.Errorf("rollback gateway policy: %w", rollbackErr))
		}
		return activationErr
	}
	return nil
}
