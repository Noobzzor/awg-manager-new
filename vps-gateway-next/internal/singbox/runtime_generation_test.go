package singbox

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const generationTestPrefix = "awgm-generation-"
const generationBase = `{"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"direct"},"dns":{"servers":[{"type":"udp","tag":"dns","server":"1.1.1.1"}]}}`

func TestRuntimeGenerationDeterministicCompleteJSON(t *testing.T) {
	a, err := PrepareRuntimeGeneration([]byte(generationBase))
	if err != nil {
		t.Fatal(err)
	}
	b, err := PrepareRuntimeGeneration([]byte(`{ "dns":{"servers":[{"server":"1.1.1.1","tag":"dns","type":"udp"}]}, "route":{"final":"direct"}, "outbounds":[{"tag":"direct","type":"direct"}] }`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a.Tag, generationTestPrefix) || len(a.Tag) != len(generationTestPrefix)+64 || a.Tag != b.Tag || !bytes.Equal(a.Config, b.Config) {
		t.Fatal("generation not deterministic or not configuration-derived")
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(a.Config, &doc); err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	if err := json.Unmarshal(doc["outbounds"], &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0]["tag"] != "direct" || out[1]["tag"] != a.Tag || out[1]["type"] != "direct" || len(out[1]) != 2 {
		t.Fatal("witness changed the default or added a selector")
	}
	if string(doc["route"]) != `{"final":"direct"}` || len(doc["dns"]) == 0 {
		t.Fatal("runtime fields lost")
	}
}

func TestRuntimeGenerationIncludesEverySuppliedSection(t *testing.T) {
	base, err := PrepareRuntimeGeneration([]byte(generationBase))
	if err != nil {
		t.Fatal(err)
	}
	for _, section := range []string{"dns", "route", "outbounds", "experimental"} {
		t.Run(section, func(t *testing.T) {
			var doc map[string]any
			if err := json.Unmarshal([]byte(generationBase), &doc); err != nil {
				t.Fatal(err)
			}
			switch section {
			case "dns":
				doc[section] = map[string]any{"final": "changed"}
			case "route":
				doc[section] = map[string]any{"rules": []any{map[string]any{"action": "reject"}}}
			case "outbounds":
				doc[section] = []any{map[string]any{"type": "direct", "tag": "other"}}
			default:
				doc[section] = map[string]any{"cache_file": map[string]any{"enabled": true}}
			}
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			got, err := PrepareRuntimeGeneration(raw)
			if err != nil || got.Tag == base.Tag {
				t.Fatal("changed runtime section retained the old generation", err)
			}
		})
	}
}

func TestRuntimeGenerationRejectsAmbiguityDefaultAndReserved(t *testing.T) {
	cases := map[string]string{
		"malformed": "{", "null": "null", "array": "[]", "missing-outbounds": "{}", "empty-outbounds": `{"outbounds":[]}`, "null-outbounds": `{"outbounds":null}`,
		"trailing":           generationBase + `{}`,
		"duplicate":          `{"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"direct","final":"other"}}`,
		"reserved-tag":       `{"outbounds":[{"type":"direct","tag":"awgm-generation-user"}]}`,
		"reserved-route":     `{"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"awgm-generation-user"}}`,
		"reserved-reference": `{"outbounds":[{"type":"selector","tag":"select","outbounds":["awgm-generation-user"]}]}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := PrepareRuntimeGeneration([]byte(input))
			if err == nil || got.Tag != "" || len(got.Config) != 0 {
				t.Fatal("unsafe runtime input accepted")
			}
		})
	}
}

func TestRuntimeGenerationPreservesNumbersAndInput(t *testing.T) {
	raw := []byte(`{"outbounds":[{"type":"direct","tag":"direct"}],"custom_sequence":9007199254740993}`)
	before := append([]byte(nil), raw...)
	g, err := PrepareRuntimeGeneration(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(g.Config, []byte("9007199254740993")) || !bytes.Equal(raw, before) {
		t.Fatal("number precision or caller input changed")
	}
	if len(g.Config) > 0 {
		g.Config[0] = 'x'
	}
	if !bytes.Equal(raw, before) {
		t.Fatal("prepared bytes alias caller input")
	}
}

func TestRuntimeGenerationRejectsDeepJSON(t *testing.T) {
	raw := strings.TrimSuffix(generationBase, "}") + `,"custom":` + strings.Repeat("[", 260) + "0" + strings.Repeat("]", 260) + "}"
	if _, err := PrepareRuntimeGeneration([]byte(raw)); err == nil {
		t.Fatal("deep runtime JSON accepted without a nesting bound")
	}
}
