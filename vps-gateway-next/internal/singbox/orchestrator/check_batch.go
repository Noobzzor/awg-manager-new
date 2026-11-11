package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// SlotCandidate is a proposed runtime slot, never an authoring draft. Nil Data
// means absent. Enabled controls participation in the merged configuration.
type SlotCandidate struct {
	Data    []byte
	Enabled bool
}

// CheckMergedBatch validates a replacement set as one read-only candidate.
// It does not commit files, toggle slots, prune references or request reload.
func (o *Orchestrator) CheckMergedBatch(ctx context.Context, replacements map[Slot]SlotCandidate) (ValidationResult, error) {
	return o.checkMergedBatch(ctx, replacements, false)
}

// CheckMergedBatchNative refuses logical-only validation. Product transactions
// must have a real DraftValidator wired before accepting runtime candidates.
func (o *Orchestrator) CheckMergedBatchNative(ctx context.Context, replacements map[Slot]SlotCandidate) (ValidationResult, error) {
	return o.checkMergedBatch(ctx, replacements, true)
}

func (o *Orchestrator) checkMergedBatch(ctx context.Context, replacements map[Slot]SlotCandidate, requireNative bool) (ValidationResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if requireNative && o.validator == nil {
		return ValidationResult{}, fmt.Errorf("native candidate validator is required")
	}
	if err := ctx.Err(); err != nil {
		return ValidationResult{}, err
	}
	for slot, candidate := range replacements {
		meta, ok := o.slots[slot]
		if !ok {
			return ValidationResult{}, ErrUnknownSlot
		}
		if !candidate.Enabled && meta.AlwaysOn && candidate.Data != nil {
			return ValidationResult{}, ErrSlotAlwaysOn
		}
	}
	// Capture every candidate and untouched active sibling once. No draft or
	// virtual selector pruning is substituted for what would actually run.
	data := make(map[Slot][]byte, len(o.slots))
	enabled := make(map[Slot]bool, len(o.slots))
	var ordered []SlotMeta
	for slot, meta := range o.slots {
		ordered = append(ordered, meta)
		if candidate, ok := replacements[slot]; ok {
			enabled[slot] = candidate.Enabled
			if candidate.Data != nil {
				data[slot] = append([]byte{}, candidate.Data...)
			}
		} else {
			enabled[slot] = o.enabled[slot]
			if !enabled[slot] {
				continue
			}
			var err error
			data[slot], err = o.readActiveBytes(slot)
			if err != nil {
				return ValidationResult{}, fmt.Errorf("snapshot batch slot %s: %w", slot, err)
			}
		}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Filename < ordered[j].Filename })
	res := o.validateWithEnabled(func(slot Slot) ([]byte, error) { return data[slot], nil }, func(slot Slot) bool { return enabled[slot] })
	if !res.Ok() {
		return res, nil
	}
	if err := ctx.Err(); err != nil {
		return ValidationResult{}, err
	}
	if o.validator == nil {
		return res, nil
	}
	dir, err := os.MkdirTemp(o.configDir, ".batch-check-*")
	if err != nil {
		return ValidationResult{}, fmt.Errorf("batch check tmpdir: %w", err)
	}
	defer os.RemoveAll(dir)
	var primary Slot
	for _, meta := range ordered {
		if _, ok := replacements[meta.Slot]; ok && primary == "" {
			primary = meta.Slot
		}
		if !enabled[meta.Slot] || data[meta.Slot] == nil {
			continue
		}
		if err := writeAtomic(filepath.Join(dir, meta.Filename), data[meta.Slot]); err != nil {
			return ValidationResult{}, fmt.Errorf("batch check snapshot %s: %w", meta.Slot, err)
		}
	}
	if err := o.validator.Validate(ctx, dir); err != nil {
		if ctx.Err() != nil {
			return ValidationResult{}, ctx.Err()
		}
		ve := ValidationError{Slot: primary, Kind: "sing-box check", Message: validatorPublicMessage}
		if slot, index, ok := o.attributeOutboundIndex(err.Error(), dir); ok {
			ve.OutboundSlot, ve.OutboundIndex = slot, &index
		}
		res.Errors = append(res.Errors, ve)
	}
	if err := ctx.Err(); err != nil {
		return ValidationResult{}, err
	}
	return res, nil
}
