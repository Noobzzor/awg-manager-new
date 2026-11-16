package main

import (
	"context"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/presets"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

func TestOriginalPresetCatalogPins(t *testing.T) {
	builtins, err := presets.LoadBuiltins()
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, entry := range builtins {
		_, standard := originalGeositePins[entry.ID]
		_, exception := originalGeositeExceptions[entry.ID]
		_, author := originalHoaxisPins[entry.ID]
		if !standard && !exception && !author {
			continue
		}
		checked++
		t.Run(entry.ID, func(t *testing.T) {
			if entry.Engines.Singbox == nil {
				t.Fatal("pinned entry has no sing-box rules")
			}
			preset := router.Preset{ID: entry.ID}
			for _, ref := range entry.Engines.Singbox.RuleSets {
				preset.RuleSets = append(preset.RuleSets, router.RuleRef{Tag: ref.Tag, URL: ref.URL})
			}
			_, err := (&app{}).resolveOriginalPresetRuleSets(context.Background(), preset)
			if err == nil || err.Error() != "original policy lease absent" {
				t.Fatalf("expected catalog validation to pass before owner check, got %v", err)
			}
			preset.RuleSets[0].URL = "https://example.com/" + preset.RuleSets[0].Tag + ".srs"
			if _, err := (&app{}).resolveOriginalPresetRuleSets(context.Background(), preset); err == nil || err.Error() == "original policy lease absent" {
				t.Fatal("accepted a URL outside the original catalog")
			}
		})
	}
	if total := len(originalGeositePins) + len(originalGeositeExceptions) + len(originalHoaxisPins); checked != total || checked != 77 {
		t.Fatalf("checked %d catalog entries for %d pins", checked, total)
	}
}
