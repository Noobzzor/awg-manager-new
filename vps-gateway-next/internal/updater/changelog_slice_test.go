package updater

import (
	"reflect"
	"testing"
)

func TestSlice_OnlyNewerThanFrom(t *testing.T) {
	entries := map[string]Entry{
		"2.8.0":  {Version: "2.8.0", Date: "2026-04-17"},
		"2.7.11": {Version: "2.7.11", Date: "2026-04-16"},
		"2.7.10": {Version: "2.7.10", Date: "2026-04-15"},
	}
	got := Slice(entries, "2.7.10", "2.8.0")
	want := []Entry{
		{Version: "2.8.0", Date: "2026-04-17"},
		{Version: "2.7.11", Date: "2026-04-16"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestSlice_IncludesTo(t *testing.T) {
	entries := map[string]Entry{
		"1.0.0": {Version: "1.0.0"},
		"1.0.1": {Version: "1.0.1"},
	}
	got := Slice(entries, "1.0.0", "1.0.1")
	if len(got) != 1 || got[0].Version != "1.0.1" {
		t.Errorf("to-bound must be included, from must be excluded; got %+v", got)
	}
}

func TestSlice_FromGreaterOrEqualReturnsEmpty(t *testing.T) {
	entries := map[string]Entry{"1.0.0": {Version: "1.0.0"}}
	if got := Slice(entries, "1.0.0", "1.0.0"); len(got) != 0 {
		t.Errorf("from==to must return empty, got %+v", got)
	}
	if got := Slice(entries, "2.0.0", "1.0.0"); len(got) != 0 {
		t.Errorf("from>to must return empty, got %+v", got)
	}
}

func TestSlice_MissingToIgnored(t *testing.T) {
	entries := map[string]Entry{
		"1.0.0": {Version: "1.0.0"},
		"2.0.0": {Version: "2.0.0"},
	}
	got := Slice(entries, "1.0.0", "9.9.9")
	if len(got) != 1 || got[0].Version != "2.0.0" {
		t.Errorf("missing 'to' should still slice by version comparison: %+v", got)
	}
}

func TestSlice_BuildRevisionSameAsRelease(t *testing.T) {
	entries := map[string]Entry{
		"2.11.2": {Version: "2.11.2", Date: "2026-05-23"},
		"2.11.1": {Version: "2.11.1", Date: "2026-05-23"},
	}
	got := Slice(entries, "2.11.2+r70", "2.11.2")
	if len(got) != 0 {
		t.Errorf("same release + build revision must not list older entries: %+v", got)
	}
}

func TestSlice_BuildRevisionUpgradeToNextPatch(t *testing.T) {
	entries := map[string]Entry{
		"2.11.3": {Version: "2.11.3"},
		"2.11.2": {Version: "2.11.2"},
		"2.11.1": {Version: "2.11.1"},
	}
	got := Slice(entries, "2.11.2+r70", "2.11.3")
	if len(got) != 1 || got[0].Version != "2.11.3" {
		t.Errorf("want only 2.11.3, got %+v", got)
	}
}

func TestMinorLine_IncludesPatchLineUpToCurrent(t *testing.T) {
	entries := map[string]Entry{
		"2.11.2": {Version: "2.11.2"},
		"2.11.1": {Version: "2.11.1"},
		"2.11.0": {Version: "2.11.0"},
		"2.10.9": {Version: "2.10.9"},
	}
	got := MinorLine(entries, "2.11.2+r70")
	if len(got) != 3 {
		t.Fatalf("want 2.11.x through 2.11.2, got %+v", got)
	}
	if got[0].Version != "2.11.2" || got[2].Version != "2.11.0" {
		t.Errorf("order/newest-first: %+v", got)
	}
}

func TestSingle_BuildRevisionLookup(t *testing.T) {
	entries := map[string]Entry{
		"2.11.2": {Version: "2.11.2", Groups: []Group{{Heading: "Fixed", Items: []string{"x"}}}},
	}
	got := Single(entries, "2.11.2+r70")
	if got == nil || got.Version != "2.11.2" {
		t.Fatalf("Single(2.11.2+r70) = %+v, want 2.11.2 entry", got)
	}
}

func TestSlice_DevelopRevisionRange(t *testing.T) {
	entries := map[string]Entry{
		"2.11.2+r93": {Version: "2.11.2+r93"},
		"2.11.2+r94": {Version: "2.11.2+r94"},
		"2.11.2+r95": {Version: "2.11.2+r95"},
	}
	got := Slice(entries, "2.11.2+r93", "2.11.2+r95")
	// fromVer < v <= toVer, newest first → r95, r94 (NOT r93)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2; got %+v", len(got), got)
	}
	if got[0].Version != "2.11.2+r95" || got[1].Version != "2.11.2+r94" {
		t.Errorf("order/content wrong: %s, %s", got[0].Version, got[1].Version)
	}
}

func TestSlice_StableUnchanged(t *testing.T) {
	entries := map[string]Entry{
		"2.11.0": {Version: "2.11.0"},
		"2.11.1": {Version: "2.11.1"},
		"2.11.2": {Version: "2.11.2"},
	}
	got := Slice(entries, "2.11.0", "2.11.2")
	if len(got) != 2 || got[0].Version != "2.11.2" || got[1].Version != "2.11.1" {
		t.Fatalf("stable slice regressed: %+v", got)
	}
}

func TestMinorLine_DevelopRevisions(t *testing.T) {
	entries := map[string]Entry{
		"2.11.2+r93": {Version: "2.11.2+r93"},
		"2.11.2+r95": {Version: "2.11.2+r95"},
		"2.10.9+r4":  {Version: "2.10.9+r4"},  // other minor → excluded
		"2.11.2+r99": {Version: "2.11.2+r99"}, // newer than current → excluded
	}
	got := MinorLine(entries, "2.11.2+r95")
	// same 2.11.x, version <= current(r95), newest first → r95, r93
	if len(got) != 2 {
		t.Fatalf("len=%d want 2; got %+v", len(got), got)
	}
	if got[0].Version != "2.11.2+r95" || got[1].Version != "2.11.2+r93" {
		t.Errorf("order/content wrong: %+v", got)
	}
}
