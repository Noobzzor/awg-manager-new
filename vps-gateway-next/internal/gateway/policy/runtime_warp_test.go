package policy

import (
	"errors"
	"github.com/hoaxisr/awg-manager/internal/awg3endpoint"
	"testing"
)

type warpProviderFixture struct {
	cfg *WARPConfig
	err error
}

func (p warpProviderFixture) WARPConfig() (*WARPConfig, error) { return p.cfg, p.err }
func TestRuntimeWARPUsesExplicitProvider(t *testing.T) {
	saver := new(recordingSlotSaver)
	runtime := Runtime{Catalog: fakeAWG3Catalog{}, Saver: saver, WARP: warpProviderFixture{cfg: &WARPConfig{Tag: "warp", Endpoint: "162.159.193.10:2408", MTU: 1280, IPv4: "172.16.0.2"}}}
	p := Profile{ID: "p", Name: "WARP", DefaultAction: ActionWARP}
	compiled, err := runtime.PreviewWithSources(p, "", nil, nil)
	if err != nil || compiled.Final != "warp" {
		t.Fatal("WARP default unavailable")
	}
	if err := runtime.Apply(p, ""); err != nil || !containsBytes(saver.data, []byte(`"outbound": "warp"`)) {
		t.Fatal("WARP apply unavailable")
	}
	p.DefaultAction = ActionDirect
	p.Rules = []Rule{{ID: "warp-sites", Action: ActionWARP, Outbound: "warp", DomainSuffixes: []string{"example.com"}, Enabled: true}}
	compiled, err = runtime.PreviewWithSources(p, "", nil, nil)
	if err != nil || len(compiled.Rules) != 1 || compiled.Rules[0].Outbound != "warp" || compiled.Final != "direct" {
		t.Fatal("WARP rule wrong")
	}
}
func TestRuntimeWARPRejectsAbsentBrokenAndConflictingProvider(t *testing.T) {
	for _, mode := range []string{"absent", "missing profile", "broken", "conflict"} {
		t.Run(mode, func(t *testing.T) {
			saver := new(recordingSlotSaver)
			runtime := Runtime{Catalog: fakeAWG3Catalog{}, Saver: saver}
			switch mode {
			case "missing profile":
				runtime.WARP = warpProviderFixture{}
			case "broken":
				runtime.WARP = warpProviderFixture{err: errors.New("unreadable")}
			case "conflict":
				runtime.Catalog = fakeAWG3Catalog{tags: []awg3endpoint.TagInfo{{Tag: "warp", Kind: "awg3"}}}
				runtime.WARP = warpProviderFixture{cfg: &WARPConfig{Tag: "warp", Endpoint: "162.159.193.10:2408", MTU: 1280}}
			}
			p := Profile{ID: "p", Name: "WARP", DefaultAction: ActionWARP}
			if err := runtime.Apply(p, ""); err == nil || len(saver.data) != 0 {
				t.Fatal("unavailable WARP applied or slot touched")
			}
		})
	}
}
