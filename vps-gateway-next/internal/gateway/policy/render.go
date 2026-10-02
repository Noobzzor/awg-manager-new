package policy

import (
	"encoding/json"
)

type slotConfig struct {
	Inbounds []gatewayTUNInbound `json:"inbounds"`
	Route    slotRoute           `json:"route"`
}

type slotRoute struct {
	Rules []CompiledRule `json:"rules,omitempty"`
}

type gatewayTUNInbound struct {
	Type             string   `json:"type"`
	Tag              string   `json:"tag"`
	InterfaceName    string   `json:"interface_name"`
	Address          []string `json:"address"`
	DNSMode          string   `json:"dns_mode"`
	AutoRoute        bool     `json:"auto_route"`
	AutoRedirect     bool     `json:"auto_redirect"`
	IncludeInterface []string `json:"include_interface"`
}

// RenderSlot renders a standalone route slot for the portable gateway policy.
// It does not write files or reload sing-box; callers hand the bytes to the
// existing atomic slot/orchestrator layer.
func RenderSlot(compiled CompiledPolicy) ([]byte, error) {
	ingressInterface := compiled.IngressInterface
	if ingressInterface == "" {
		ingressInterface = DefaultGatewayIngressInterface
	}
	rules := []CompiledRule{
		// This slot precedes the Router slot and terminates Gateway routing.
		// Populate protocol/domain metadata here, before any terminal rule.
		{Inbound: []string{GatewayInboundTag}, Action: "sniff"},
		{Inbound: []string{GatewayInboundTag}, Protocol: "dns", Action: "hijack-dns"},
		{Inbound: []string{GatewayInboundTag}, Port: []int{53}, Action: "hijack-dns"},
	}
	rules = append(rules, compiled.Rules...)
	fallback := CompiledRule{
		Inbound: []string{GatewayInboundTag},
	}
	if compiled.Final == "block" {
		fallback.Action = "reject"
	} else {
		fallback.Action = "route"
		fallback.Outbound = compiled.Final
	}
	rules = append(rules, fallback)
	payload := slotConfig{
		Inbounds: []gatewayTUNInbound{{
			Type:             "tun",
			Tag:              GatewayInboundTag,
			InterfaceName:    GatewayTUNInterface,
			Address:          []string{GatewayTUNAddress},
			DNSMode:          "disabled",
			AutoRoute:        true,
			AutoRedirect:     true,
			IncludeInterface: []string{ingressInterface},
		}},
		Route: slotRoute{
			Rules: rules,
		},
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
