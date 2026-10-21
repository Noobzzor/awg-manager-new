package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatewayAWGRejectsBeforeIdentity(t *testing.T) {
	for _, tc := range []struct{ name, backend, params, want string }{
		{"unknown", "typo", "", "invalid AWG_GATEWAY_BACKEND"},
		{"missing-profile", "userspace-awg", "", "AWG_GATEWAY_PARAMETERS"},
		{"unknown-field", "userspace-awg", `{"jc":2,"typo":1}`, "AWG_GATEWAY_PARAMETERS"},
		{"missing-runtime", "userspace-awg", `{"jc":2,"jmin":40,"jmax":80,"s1":15,"s2":20,"s3":10,"s4":25,"h1":100001,"h2":100002,"h3":100003,"h4":100004}`, "required executable unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AWG_GATEWAY_ENABLE", "true")
			t.Setenv("AWG_GATEWAY_WAN_INTERFACE", "eth0")
			t.Setenv("AWG_GATEWAY_SUBSCRIPTION_BASE_URL", "")
			t.Setenv("AWG_GATEWAY_BACKEND", tc.backend)
			t.Setenv("AWG_GATEWAY_PARAMETERS", tc.params)
			t.Setenv("AWG_GATEWAY_TOOL", "/missing-awg-tool")
			t.Setenv("AWG_GATEWAY_RUNTIME_BINARY", "/missing-awg-runtime")
			a := &app{dataDir: t.TempDir()}
			defer a.runOnExit()
			err := a.setupGatewayRuntime()
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wrong early refusal: %v; want %s", err, tc.want)
			}
			if _, err := os.Stat(filepath.Join(a.dataDir, "gateway-server-identity.json")); !os.IsNotExist(err) {
				t.Fatal("identity created before semantic preflight")
			}
			if a.gatewayClients != nil || a.gatewayRuntime != nil {
				t.Fatal("failed startup published gateway")
			}
		})
	}
}

func TestGatewayWGModeKeepsLegacyAdmission(t *testing.T) {
	for _, backend := range []string{"", "wg"} {
		t.Run("backend="+backend, func(t *testing.T) {
			t.Setenv("AWG_GATEWAY_ENABLE", "true")
			t.Setenv("AWG_GATEWAY_WAN_INTERFACE", "eth0")
			t.Setenv("AWG_GATEWAY_SUBSCRIPTION_BASE_URL", "")
			t.Setenv("AWG_GATEWAY_BACKEND", backend)
			t.Setenv("AWG_GATEWAY_PARAMETERS", "invalid-awg-profile")
			t.Setenv("AWG_GATEWAY_TOOL", "/missing-wg-tool")
			a := &app{dataDir: t.TempDir()}
			defer a.runOnExit()
			if err := a.setupGatewayRuntime(); err == nil || !strings.Contains(err.Error(), "generate server identity") {
				t.Fatalf("legacy path changed: %v", err)
			}
		})
	}
}

func TestGatewayAWGProfileStrictJSON(t *testing.T) {
	valid := `{"jc":2,"jmin":40,"jmax":80,"s1":15,"s2":20,"s3":10,"s4":25,"h1":100001,"h2":100002,"h3":100003,"h4":100004}`
	p, err := parseGatewayAWGParameters(valid)
	if err != nil || p.S4 != 25 || p.H4 != 100004 {
		t.Fatalf("valid profile rejected: %v", err)
	}
	for _, raw := range []string{"", "null", "{}", valid + "{}", valid + " null", strings.Replace(valid, `"jc":2`, `"jc":2,"unknown":3`, 1), strings.Replace(valid, `"h4":100004`, `"h4":1`, 1), strings.Replace(valid, `"jc":2`, `"jc":2,"jc":3`, 1), strings.Replace(valid, `"jc":2`, `"jc":2,"JC":3`, 1)} {
		if _, err := parseGatewayAWGParameters(raw); err == nil {
			t.Fatal("invalid profile accepted")
		}
	}
}
