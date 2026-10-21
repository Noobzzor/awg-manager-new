package main

import (
	"encoding/json"
	"fmt"
	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
	"io"
	"strings"
)

type gatewayOwnedAWGInterface interface {
	gatewayingress.ManagedInterface
	gatewayingress.PeerApplier
	gatewayingress.PeerStatusReader
	SetPrivateKey(string) error
}

func parseGatewayAWGParameters(raw string) (gatewayingress.AWGParameters, error) {
	var p gatewayingress.AWGParameters
	decoder := json.NewDecoder(strings.NewReader(raw))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return p, fmt.Errorf("invalid AWG_GATEWAY_PARAMETERS")
	}
	allowed := map[string]bool{"jc": true, "jmin": true, "jmax": true, "s1": true, "s2": true, "s3": true, "s4": true, "h1": true, "h2": true, "h3": true, "h4": true}
	seen := make(map[string]bool)
	for decoder.More() {
		key, err := decoder.Token()
		name, ok := key.(string)
		if err != nil || !ok || !allowed[name] || seen[name] {
			return p, fmt.Errorf("invalid AWG_GATEWAY_PARAMETERS: unknown or repeated field")
		}
		seen[name] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return p, fmt.Errorf("invalid AWG_GATEWAY_PARAMETERS")
		}
	}
	if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
		return p, fmt.Errorf("invalid AWG_GATEWAY_PARAMETERS")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("invalid AWG_GATEWAY_PARAMETERS: trailing data")
	}
	if len(seen) != len(allowed) {
		return p, fmt.Errorf("invalid AWG_GATEWAY_PARAMETERS: all eleven fields required")
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return p, fmt.Errorf("invalid AWG_GATEWAY_PARAMETERS")
	}
	if err := p.Validate(); err != nil {
		return p, fmt.Errorf("invalid AWG_GATEWAY_PARAMETERS: %w", err)
	}
	return p, nil
}
