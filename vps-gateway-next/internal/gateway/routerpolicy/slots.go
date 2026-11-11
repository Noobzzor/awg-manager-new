package routerpolicy

import (
	"encoding/json"
	"fmt"

	"github.com/hoaxisr/awg-manager/internal/gateway/policy"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

// Slots separates gateway ingress/prefix (18) from scoped original router
// rules/resources (20), leaving existing DNS-route policy (19) in between.
// Canonical editor documents must be stored separately from derived runtime.
type Slots struct {
	Ingress []byte
	Routing []byte
}

func RenderSlots(ingressInterface string, authored *router.RouterConfig) (Slots, error) {
	all, err := Render(ingressInterface, authored)
	if err != nil {
		return Slots{}, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(all, &doc); err != nil {
		return Slots{}, err
	}
	var route map[string]json.RawMessage
	if err := json.Unmarshal(doc["route"], &route); err != nil {
		return Slots{}, err
	}
	var rules []json.RawMessage
	if err := json.Unmarshal(route["rules"], &rules); err != nil {
		return Slots{}, err
	}
	// Derive the boundary from the existing gateway renderer rather than
	// independently duplicating its sniff/DNS prefix or counting fixed rules.
	base, err := policy.RenderSlot(policy.CompiledPolicy{IngressInterface: ingressInterface, Final: authored.Route.Final})
	if err != nil {
		return Slots{}, err
	}
	var ingress struct {
		Route struct {
			Rules []json.RawMessage `json:"rules"`
		} `json:"route"`
	}
	if err := json.Unmarshal(base, &ingress); err != nil {
		return Slots{}, err
	}
	prefixCount := len(ingress.Route.Rules) - 1
	if prefixCount < 0 || len(rules) <= prefixCount {
		return Slots{}, fmt.Errorf("gateway projection has no scoped fallback")
	}
	prefix, err := json.Marshal(struct {
		Inbounds json.RawMessage `json:"inbounds"`
		Route    map[string]any  `json:"route"`
	}{doc["inbounds"], map[string]any{"rules": rules[:prefixCount]}})
	if err != nil {
		return Slots{}, err
	}
	delete(doc, "inbounds")
	route["rules"], err = json.Marshal(rules[prefixCount:])
	if err != nil {
		return Slots{}, err
	}
	doc["route"], err = json.Marshal(route)
	if err != nil {
		return Slots{}, err
	}
	routing, err := json.Marshal(doc)
	if err != nil {
		return Slots{}, err
	}
	return Slots{Ingress: append(prefix, '\n'), Routing: append(routing, '\n')}, nil
}
