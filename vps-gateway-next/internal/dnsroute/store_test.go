package dnsroute

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreLoadBackfillsRawEditorText(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dns-routes.json")

	raw := []byte(`{
  "lists": [
    {
      "id": "list_1",
      "name": "legacy",
      "domains": ["youtube.com"],
      "manualDomains": ["youtube.com", "googlevideo.com"],
      "excludes": ["local"],
      "excludeSubnets": ["10.0.0.0/8"],
      "routes": []
    }
  ]
}`)
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}

	store := NewStore(dir)
	data, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Lists) != 1 {
		t.Fatalf("lists len = %d, want 1", len(data.Lists))
	}

	list := data.Lists[0]

	if list.ManualText == nil {
		t.Fatal("ManualText was not backfilled")
	}
	if *list.ManualText != "youtube.com\ngooglevideo.com" {
		t.Fatalf("ManualText = %q", *list.ManualText)
	}

	if list.ExcludesText == nil {
		t.Fatal("ExcludesText was not backfilled")
	}
	if *list.ExcludesText != "local\n10.0.0.0/8" {
		t.Fatalf("ExcludesText = %q", *list.ExcludesText)
	}
}

func TestMigrateRuleSetSubscriptionURLs(t *testing.T) {
	data := &StoreData{Lists: []DomainList{{
		Subscriptions: []Subscription{
			{URL: "https://github.com/vernette/rulesets/raw/master/raw/unavailable-in-russia.txt"},
			{URL: "https://example.com/other.txt"},
		},
	}}}

	if !migrateRuleSetSubscriptionURLs(data) {
		t.Fatal("ожидался changed=true")
	}
	if got := data.Lists[0].Subscriptions[0].URL; got != "https://repo.hoaxisr.ru/rulesets/raw/unavailable-in-russia.txt" {
		t.Errorf("vernette не переписан: %s", got)
	}
	if got := data.Lists[0].Subscriptions[1].URL; got != "https://example.com/other.txt" {
		t.Errorf("чужой URL изменён: %s", got)
	}

	// Идемпотентность / no-op.
	if migrateRuleSetSubscriptionURLs(data) {
		t.Error("2-й прогон должен вернуть false")
	}
}

func TestMigrateDiscordDNSSubnet(t *testing.T) {
	mt := "discord.com\ndiscordapp.com\n162.158.0.0/15"
	data := &StoreData{Lists: []DomainList{
		{ // applied Discord DNS list — должен мигрировать
			Name:          "Discord",
			Domains:       []string{"discord.com", "discordapp.com", "discord.gg"},
			ManualDomains: []string{"discord.com", "discordapp.com", "162.158.0.0/15"},
			ManualText:    &mt,
			Subnets:       []string{"162.158.0.0/15"},
		},
		{ // чужой список — НЕ трогать
			Name:    "My stuff",
			Domains: []string{"example.com"},
			Subnets: []string{"1.2.3.0/24"},
		},
	}}

	if !migrateDiscordDNSSubnet(data) {
		t.Fatal("ожидался changed=true")
	}
	d := data.Lists[0]
	if !slicesHas(d.Subnets, "104.29.0.0/16") {
		t.Errorf("Subnets без CIDR: %v", d.Subnets)
	}
	if !slicesHas(d.ManualDomains, "104.29.0.0/16") {
		t.Errorf("ManualDomains без CIDR: %v", d.ManualDomains)
	}
	if d.ManualText == nil || !strings.Contains(*d.ManualText, "104.29.0.0/16") {
		t.Errorf("ManualText без CIDR: %v", d.ManualText)
	}
	if other := data.Lists[1]; slicesHas(other.Subnets, "104.29.0.0/16") {
		t.Errorf("чужой список затронут: %v", other.Subnets)
	}

	// Идемпотентность.
	if migrateDiscordDNSSubnet(data) {
		t.Error("2-й прогон должен вернуть false")
	}
}

func TestMigrateDiscordDNSSubnet_Negatives(t *testing.T) {
	// Один сигнатурный домен — НЕ Discord, не матчим.
	one := &StoreData{Lists: []DomainList{{Domains: []string{"discord.com", "example.com"}}}}
	if migrateDiscordDNSSubnet(one) {
		t.Error("один сигнатурный домен не должен матчиться")
	}

	// ManualText == nil, обе сигнатуры в ManualDomains — матч; ManualText остаётся nil.
	noText := &StoreData{Lists: []DomainList{{
		ManualDomains: []string{"discord.com", "discordapp.com"},
	}}}
	if !migrateDiscordDNSSubnet(noText) {
		t.Fatal("ManualDomains-сигнатура должна матчиться")
	}
	if noText.Lists[0].ManualText != nil {
		t.Error("ManualText не должен создаваться из nil")
	}
	if !slicesHas(noText.Lists[0].Subnets, "104.29.0.0/16") {
		t.Error("Subnets должны получить CIDR")
	}

	// HydraRoute-список не трогаем даже при сигнатуре.
	hr := &StoreData{Lists: []DomainList{{
		Backend: "hydraroute",
		Domains: []string{"discord.com", "discordapp.com"},
	}}}
	if migrateDiscordDNSSubnet(hr) {
		t.Error("HydraRoute-список не должен мигрировать")
	}

	// Уже есть CIDR во всех полях — no-op.
	mt := "discord.com\ndiscordapp.com\n104.29.0.0/16"
	full := &StoreData{Lists: []DomainList{{
		Domains:       []string{"discord.com", "discordapp.com"},
		ManualDomains: []string{"discord.com", "discordapp.com", "104.29.0.0/16"},
		ManualText:    &mt,
		Subnets:       []string{"104.29.0.0/16"},
	}}}
	if migrateDiscordDNSSubnet(full) {
		t.Error("полностью мигрированный список — no-op")
	}
}

func slicesHas(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

// Load-уровневый тест: миграции C2/C3 не только правят кэш, но и ПЕРСИСТЯТСЯ
// на диск (ветка forkMigrated/discordMigrated → writeLocked).
func TestStoreLoadPersistsForkAndDiscordMigrations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dns-routes.json")

	raw := []byte(`{
  "lists": [
    {
      "id": "list_1",
      "name": "Discord",
      "domains": ["discord.com", "discordapp.com"],
      "manualDomains": ["discord.com", "discordapp.com"],
      "subnets": ["162.158.0.0/15"],
      "subscriptions": [
        {"url": "https://github.com/vernette/rulesets/raw/master/raw/unavailable-in-russia.txt"}
      ],
      "routes": []
    }
  ]
}`)
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewStore(dir).Load(); err != nil {
		t.Fatal(err)
	}

	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(onDisk)
	if !strings.Contains(s, "https://repo.hoaxisr.ru/rulesets/raw/unavailable-in-russia.txt") {
		t.Error("C2 не персистнута: URL зеркала нет на диске")
	}
	if strings.Contains(s, "vernette") {
		t.Error("C2 не персистнута: vernette остался на диске")
	}
	if !strings.Contains(s, "104.29.0.0/16") {
		t.Error("C3 не персистнута: CIDR нет на диске")
	}
}
