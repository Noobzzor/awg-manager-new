// Package routerpolicy projects the original editor model into the owned VPS
// ingress slot. It does not persist, activate, or replace a gateway policy.
package routerpolicy

import (
	"encoding/json"
	"fmt"

	"github.com/hoaxisr/awg-manager/internal/gateway/policy"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

// Render produces a combined projection for standalone engine controls. Product
// integration must use RenderSlots to preserve other producers' slot ordering.
// The caller must select the original editor as the single rule owner first.
func Render(ingressInterface string, authored *router.RouterConfig) ([]byte, error) {
	if authored == nil {
		return nil, fmt.Errorf("original router config is required")
	}
	if len(authored.Inbounds) != 0 {
		return nil, fmt.Errorf("original router platform inbounds cannot replace gateway ingress")
	}
	if authored.Route.Final == "" {
		return nil, fmt.Errorf("original router final outbound is required")
	}
	base, err := policy.RenderSlot(policy.CompiledPolicy{IngressInterface: ingressInterface, Final: authored.Route.Final})
	if err != nil {
		return nil, err
	}
	// Keep the proven ingress bytes intact: router.Inbound intentionally does
	// not model all gateway fields (dns_mode/include_interface in particular).
	var ingress struct {
		Inbounds json.RawMessage `json:"inbounds"`
		Route    router.Route    `json:"route"`
	}
	if err := json.Unmarshal(base, &ingress); err != nil {
		return nil, err
	}
	prefix := ingress.Route.Rules[:len(ingress.Route.Rules)-1]
	fallback := ingress.Route.Rules[len(ingress.Route.Rules)-1]
	route := authored.Route
	route.Final = ""
	route.Rules = make([]router.Rule, 0, len(prefix)+len(authored.Route.Rules)+1)
	route.Rules = append(route.Rules, prefix...)
	for _, rule := range authored.Route.Rules {
		// Scope outside the original matcher, not inside its OR/invert. Existing
		// inbound conditions remain an additional restriction, never overwritten.
		matcher := rule
		matcher.Action, matcher.Outbound, matcher.UDPTimeout = "", "", ""
		route.Rules = append(route.Rules, router.Rule{
			Type: "logical", Mode: "and",
			Rules:  []router.Rule{{Inbound: []string{policy.GatewayInboundTag}}, matcher},
			Action: rule.Action, Outbound: rule.Outbound, UDPTimeout: rule.UDPTimeout,
		})
	}
	route.Rules = append(route.Rules, fallback)
	// DNS remains the original engine-wide DNS model, as in the original
	// editor. Global DNS/base/other-slot ownership must be validated by apply.
	payload := struct {
		Inbounds     json.RawMessage      `json:"inbounds"`
		Outbounds    []router.Outbound    `json:"outbounds"`
		DNS          router.DNS           `json:"dns"`
		Route        router.Route         `json:"route"`
		HTTPClients  []router.HTTPClient  `json:"http_clients,omitempty"`
		Experimental *router.Experimental `json:"experimental,omitempty"`
	}{ingress.Inbounds, authored.Outbounds, authored.DNS, route, authored.HTTPClients, authored.Experimental}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
