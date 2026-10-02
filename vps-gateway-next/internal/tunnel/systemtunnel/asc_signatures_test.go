package systemtunnel

import (
	"encoding/json"
	"strings"
	"testing"
)

const containerI1 = "<b 0xc3><b 0x00000001><b 0x08><r 8><b 0x00><b 0x00><b 0x449e><r 4><r 1178>"

func TestSplitASCSignatures_SplitsOversizedTag(t *testing.T) {
	in := json.RawMessage(`{"jc":3,"s1":18,"i1":"` + containerI1 + `","i5":"<r 32>"}`)
	raw, note := splitASCSignatures(in)
	got := string(raw)

	if strings.Contains(got, "<r 1178>") {
		t.Errorf("oversized tag survived: %s", got)
	}
	if !strings.Contains(got, "<r 1000><r 178>") {
		t.Errorf("expected split tag: %s", got)
	}
	if !strings.Contains(note, "I1: <r 1178> → <r 1000><r 178>") {
		t.Errorf("rewrite must be described for the log, got %q", note)
	}
	// Angle brackets must not be HTML-escaped on the way back out: NDMS needs
	// the literal <>, and encoding/json escapes them by default.
	if strings.Contains(got, `\u003c`) || strings.Contains(got, `\u003e`) {
		t.Errorf("signature was HTML-escaped: %s", got)
	}
	// Untouched fields survive.
	var obj map[string]any
	if err := json.Unmarshal([]byte(got), &obj); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if obj["jc"] != float64(3) || obj["s1"] != float64(18) || obj["i5"] != "<r 32>" {
		t.Errorf("unrelated fields altered: %s", got)
	}
}

func TestSplitASCSignatures_PassesThroughUnchanged(t *testing.T) {
	for _, in := range []string{
		`{"jc":3,"i1":"<b 0x170303><r 32><t>"}`, // nothing to fix
		`{"jc":3}`,                              // no signature fields
		`not json at all`,                       // caller's problem, not ours
		`[1,2,3]`,                               // not an object
		`{"i1":42}`,                             // wrong type in slot
	} {
		if got, note := splitASCSignatures(json.RawMessage(in)); string(got) != in || note != "" {
			t.Errorf("splitASCSignatures(%q)=%q, %q; want unchanged, empty note", in, got, note)
		}
	}
}
