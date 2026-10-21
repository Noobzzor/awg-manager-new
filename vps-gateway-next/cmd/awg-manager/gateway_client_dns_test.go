package main

import (
	"strings"
	"testing"
)

func TestGatewayClientDNSValidatedBeforeRuntimeEffects(t *testing.T) {
	t.Setenv("AWG_GATEWAY_ENABLE", "true")
	t.Setenv("AWG_GATEWAY_BACKEND", "wg")
	t.Setenv("AWG_GATEWAY_CLIENT_DNS", "invalid-client-dns")
	a := &app{}
	if err := a.setupGatewayRuntime(); err == nil || !strings.Contains(err.Error(), "AWG_GATEWAY_CLIENT_DNS") {
		t.Fatalf("expected early DNS validation, got %v", err)
	}
}
