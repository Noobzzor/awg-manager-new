package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndValidateKeepsLastKnownGoodOnValidationFailure(t *testing.T) {
	o, dir := newTestOrch(t)
	if err := o.Register(SlotMeta{Slot: SlotRouter, Filename: "20-router.json"}); err != nil {
		t.Fatal(err)
	}
	if err := o.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	if err := o.SetEnabled(SlotRouter, true); err != nil {
		t.Fatal(err)
	}

	good := []byte(`{"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"direct"}}`)
	if res, err := o.SaveAndValidate(SlotRouter, good); err != nil || !res.Ok() {
		t.Fatalf("save good: res=%+v err=%v", res, err)
	}

	bad := []byte(`{"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"missing"}}`)
	res, err := o.SaveAndValidate(SlotRouter, bad)
	if err != nil {
		t.Fatalf("invalid candidate must be a validation result, not infrastructure error: %v", err)
	}
	if res.Ok() || !strings.Contains(res.Error(), "unknown-outbound") {
		t.Fatalf("expected unknown outbound validation error, got %+v (%s)", res, res.Error())
	}

	got, err := os.ReadFile(filepath.Join(dir, "20-router.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(good) {
		t.Fatalf("last-known-good was replaced: got %s", got)
	}
}
