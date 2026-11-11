package routerpolicy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/gateway/policy"
	"github.com/hoaxisr/awg-manager/internal/singbox/configmerge"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

func TestSplitProjectionPreservesDNSRoutePriority(t *testing.T) {
	cfg := router.NewEmptyConfig()
	cfg.Route.Rules = []router.Rule{{DomainSuffix: []string{"fixture.example"}, Action: "reject"}}
	cfg.DNS.Servers = []router.DNSServer{{Tag: "editor-dns", Type: "udp", Server: "192.0.2.53"}}
	cfg.DNS.Final = "editor-dns"
	cfg.Route.RuleSet = []router.RuleSet{{Tag: "editor-set", Type: "inline", Rules: []map[string]any{{"domain_suffix": []string{"fixture.example"}}}}}
	cfg.Outbounds = []router.Outbound{{Tag: "editor-direct", Type: "direct"}}
	want, _ := json.Marshal(cfg)
	slots, err := RenderSlots("awg0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	var ingress, routing struct {
		Inbounds  json.RawMessage   `json:"inbounds"`
		Route     router.Route      `json:"route"`
		DNS       router.DNS        `json:"dns"`
		Outbounds []router.Outbound `json:"outbounds"`
	}
	if err := json.Unmarshal(slots.Ingress, &ingress); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(slots.Routing, &routing); err != nil {
		t.Fatal(err)
	}
	if len(ingress.Route.Rules) != 3 {
		t.Fatal("early ingress slot still contains editor rules or terminal fallback")
	}
	if len(ingress.Outbounds) != 0 || len(ingress.DNS.Servers) != 0 || len(ingress.Route.RuleSet) != 0 {
		t.Fatal("early ingress owns editor resources")
	}
	if len(ingress.Inbounds) == 0 || len(routing.Inbounds) != 0 {
		t.Fatal("gateway ingress lost or duplicated")
	}
	if len(routing.Route.Rules) != 2 || routing.Route.Final != "" {
		t.Fatal("scoped editor rules and fallback missing")
	}
	wantSets, _ := json.Marshal(cfg.Route.RuleSet)
	gotSets, _ := json.Marshal(routing.Route.RuleSet)
	if !reflect.DeepEqual(routing.Outbounds, cfg.Outbounds) || string(gotSets) != string(wantSets) || routing.DNS.Final != cfg.DNS.Final {
		t.Fatalf("original resources lost: got sets=%s want=%s", gotSets, wantSets)
	}
	dir := t.TempDir()
	for file, data := range map[string][]byte{
		"18-z-gateway-policy.json": slots.Ingress,
		"19-dns-routes.json":       []byte(`{"route":{"rules":[{"domain_suffix":["fixture.example"],"outbound":"dns-route-exit"}]}}`),
		"20-router.json":           slots.Routing,
	} {
		if err := os.WriteFile(filepath.Join(dir, file), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	merged, err := configmerge.MergeDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got router.RouterConfig
	if err := json.Unmarshal([]byte(merged), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Route.Rules) != 6 || got.Route.Rules[3].Outbound != "dns-route-exit" || got.Route.Rules[4].Action != "reject" {
		t.Fatal("editor projection changed DNS-route precedence")
	}
	last := got.Route.Rules[5]
	if !reflect.DeepEqual(last.Inbound, []string{policy.GatewayInboundTag}) || last.Outbound != "direct" {
		t.Fatal("fallback can capture unrelated ingress")
	}
	after, _ := json.Marshal(cfg)
	if string(after) != string(want) {
		t.Fatal("projection mutated canonical editor")
	}
}
