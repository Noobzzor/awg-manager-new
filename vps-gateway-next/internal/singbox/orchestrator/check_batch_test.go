package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func batchFixture(t *testing.T) (*Orchestrator, *snapshotRecordingValidator) {
	t.Helper()
	dir := t.TempDir()
	o := NewWithAppliedPath(dir, nil, filepath.Join(t.TempDir(), "applied.json"))
	t.Cleanup(o.Close)
	for _, m := range KnownSlots() {
		if err := o.Register(m); err != nil {
			t.Fatal(err)
		}
	}
	for file, data := range map[string]string{
		"00-base.json":             `{"outbounds":[{"tag":"direct","type":"direct"}]}`,
		"18-z-gateway-policy.json": `{"outbounds":[{"tag":"shared-exit","type":"direct"}],"route":{"rules":[{"outbound":"shared-exit"}]}}`,
		"19-dns-routes.json":       `{"route":{"rules":[{"domain_suffix":["fixture.example"],"outbound":"shared-exit"}]}}`,
	} {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := o.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	v := &snapshotRecordingValidator{}
	o.SetValidator(v)
	return o, v
}

func TestBatchNativeValidationRequiresValidator(t *testing.T) {
	o, _ := batchFixture(t)
	o.SetValidator(nil)
	before := batchTree(t, o.ConfigDir())
	if res, err := o.CheckMergedBatchNative(context.Background(), nil); err == nil && res.Ok() {
		t.Fatal("logical-only validation acknowledged as native")
	}
	if !reflect.DeepEqual(before, batchTree(t, o.ConfigDir())) {
		t.Fatal("refusal changed runtime files")
	}
}

func TestSnapshotSlotPreservesEmptyPresentFile(t *testing.T) {
	o, _ := batchFixture(t)
	path, err := o.ActivePath(SlotGatewayPolicy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	raw, enabled, err := o.SnapshotSlot(SlotGatewayPolicy)
	if err != nil || raw == nil || len(raw) != 0 || !enabled {
		t.Fatal("empty present file became an absent rollback snapshot", err)
	}
}

func batchTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out[rel] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBatchValidationMovesResourcesWithoutTransientDuplicates(t *testing.T) {
	o, v := batchFixture(t)
	before, enabled := batchTree(t, o.ConfigDir()), o.Snapshot()
	replacements := map[Slot]SlotCandidate{
		SlotGatewayPolicy: {Data: []byte(`{"route":{"rules":[{"action":"sniff"}]}}`), Enabled: true},
		SlotRouter:        {Data: []byte(`{"outbounds":[{"tag":"shared-exit","type":"direct"}],"route":{"rules":[{"outbound":"shared-exit"}]}}`), Enabled: true},
	}
	res, err := o.CheckMergedBatch(context.Background(), replacements)
	if err != nil || !res.Ok() {
		t.Fatalf("joint candidate rejected: %v %s", err, res.Error())
	}
	if string(v.files["20-router.json"]) != string(replacements[SlotRouter].Data) || string(v.files["18-z-gateway-policy.json"]) != string(replacements[SlotGatewayPolicy].Data) || len(v.files["19-dns-routes.json"]) == 0 {
		t.Fatal("native validator did not see complete replacement set and untouched sibling")
	}
	if !reflect.DeepEqual(before, batchTree(t, o.ConfigDir())) || !reflect.DeepEqual(enabled, o.Snapshot()) {
		t.Fatal("validation changed live files or enabled state")
	}
}

func TestBatchValidationRejectsInconsistentMergedCandidate(t *testing.T) {
	for _, tc := range []struct{ name, data, kind string }{
		{"duplicate", `{"outbounds":[{"tag":"shared-exit","type":"direct"}]}`, "duplicate-outbound"},
		{"missing-exit", `{"route":{"rules":[{"outbound":"missing"}]}}`, "unknown-outbound"},
		{"bad-json", `{`, "parse-error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, v := batchFixture(t)
			before := batchTree(t, o.ConfigDir())
			res, err := o.CheckMergedBatch(context.Background(), map[Slot]SlotCandidate{SlotRouter: {Data: []byte(tc.data), Enabled: true}})
			if err != nil || res.Ok() || !strings.Contains(res.Error(), tc.kind) {
				t.Fatalf("invalid candidate accepted: %v %s", err, res.Error())
			}
			if v.files != nil || !reflect.DeepEqual(before, batchTree(t, o.ConfigDir())) {
				t.Fatal("invalid candidate reached native validator or changed disk")
			}
		})
	}
}

func TestBatchValidationNeverPrunesSiblingDependencies(t *testing.T) {
	o, _ := batchFixture(t)
	selector := []byte(`{"outbounds":[{"type":"selector","tag":"dependent","outbounds":["shared-exit","direct"],"default":"shared-exit"}]}`)
	if err := os.WriteFile(filepath.Join(o.ConfigDir(), "40-subscriptions.json"), selector, 0600); err != nil {
		t.Fatal(err)
	}
	o.enabled[SlotSubscriptions] = true
	before := batchTree(t, o.ConfigDir())
	res, err := o.CheckMergedBatch(context.Background(), map[Slot]SlotCandidate{SlotGatewayPolicy: {Data: []byte(`{}`), Enabled: true}, SlotDNSRoutes: {Enabled: false}})
	if err != nil || res.Ok() || !strings.Contains(res.Error(), "shared-exit") {
		t.Fatalf("removed dependency silently pruned: %v %s", err, res.Error())
	}
	if !reflect.DeepEqual(before, batchTree(t, o.ConfigDir())) {
		t.Fatal("sibling mutated")
	}
}

func TestBatchValidationGuardsAndCancellation(t *testing.T) {
	o, _ := batchFixture(t)
	if _, err := o.CheckMergedBatch(context.Background(), map[Slot]SlotCandidate{Slot("unknown"): {Data: []byte(`{}`), Enabled: true}}); !errors.Is(err, ErrUnknownSlot) {
		t.Fatal("unknown slot accepted", err)
	}
	if _, err := o.CheckMergedBatch(context.Background(), map[Slot]SlotCandidate{SlotBase: {Data: []byte(`{}`), Enabled: false}}); !errors.Is(err, ErrSlotAlwaysOn) {
		t.Fatal("always-on slot disabled", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := o.CheckMergedBatch(ctx, map[Slot]SlotCandidate{}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled check accepted", err)
	}
}

func TestBatchValidationPreservesEmptyCandidateForNativeCheck(t *testing.T) {
	o, v := batchFixture(t)
	res, err := o.CheckMergedBatch(context.Background(), map[Slot]SlotCandidate{SlotRouter: {Data: []byte{}, Enabled: true}})
	if err != nil || !res.Ok() {
		t.Fatalf("fixture validator rejected: %v %s", err, res.Error())
	}
	if raw, present := v.files["20-router.json"]; !present || len(raw) != 0 {
		t.Fatal("empty candidate silently became an absent file")
	}
}

func TestBatchValidationNativeFailureIsSanitizedAndReadOnly(t *testing.T) {
	o, _ := batchFixture(t)
	before := batchTree(t, o.ConfigDir())
	o.SetValidator(&fakeValidator{err: errors.New("synthetic private validator diagnostic")})
	res, err := o.CheckMergedBatch(context.Background(), map[Slot]SlotCandidate{SlotRouter: {Data: []byte(`{}`), Enabled: true}})
	if err != nil || res.Ok() || strings.Contains(res.Error(), "private validator") {
		t.Fatalf("native failure accepted or disclosed: %v %s", err, res.Error())
	}
	if !reflect.DeepEqual(before, batchTree(t, o.ConfigDir())) {
		t.Fatal("native rejection changed working state")
	}
}
