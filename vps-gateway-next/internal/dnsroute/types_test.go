package dnsroute

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDomainList_IconURL_RoundTrip(t *testing.T) {
	t.Run("iconUrl survives marshal/unmarshal", func(t *testing.T) {
		orig := DomainList{
			ID:      "list_1",
			Name:    "Telegram",
			IconURL: "https://cdn.jsdelivr.net/gh/Koolson/Qure@master/IconSet/Color/Telegram.png",
			Domains: []string{"t.me"},
			Enabled: true,
		}
		raw, err := json.Marshal(orig)
		if err != nil {
			t.Fatal(err)
		}
		var got DomainList
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got.IconURL != orig.IconURL {
			t.Errorf("IconURL = %q, want %q", got.IconURL, orig.IconURL)
		}
	})

	t.Run("empty iconUrl is omitted in JSON", func(t *testing.T) {
		raw, err := json.Marshal(DomainList{ID: "list_1", Name: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "iconUrl") {
			t.Errorf("expected iconUrl to be omitted, got: %s", raw)
		}
	})

	t.Run("legacy JSON without iconUrl unmarshals fine", func(t *testing.T) {
		legacy := []byte(`{"id":"list_1","name":"x","domains":["a.com"],"manualDomains":[],"routes":[],"enabled":true}`)
		var got DomainList
		if err := json.Unmarshal(legacy, &got); err != nil {
			t.Fatal(err)
		}
		if got.IconURL != "" {
			t.Errorf("IconURL = %q, want empty string", got.IconURL)
		}
	})
}
