package main

import (
	"strings"
	"testing"
)

func TestParseDeploymentMode(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		set     bool
		want    deploymentMode
		wantErr bool
	}{
		{name: "unset value defaults to keenetic", want: deploymentModeKeenetic},
		{name: "explicit empty fails", set: true, wantErr: true},
		{name: "keenetic", value: "keenetic", set: true, want: deploymentModeKeenetic},
		{name: "singbox", value: "singbox", set: true, want: deploymentModeSingbox},
		{name: "unknown", value: "docker", set: true, wantErr: true},
		{name: "blank explicit value", value: "   ", set: true, wantErr: true},
		{name: "case is strict", value: "SINGBOX", set: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDeploymentMode(func(string) (string, bool) { return tt.value, tt.set })
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "AWG_MODE") {
					t.Fatalf("parseDeploymentMode(%q, set=%v) error = %v, want clear AWG_MODE error", tt.value, tt.set, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseDeploymentMode(%q, set=%v): %v", tt.value, tt.set, err)
			}
			if got != tt.want {
				t.Fatalf("parseDeploymentMode(%q, set=%v) = %q, want %q", tt.value, tt.set, got, tt.want)
			}
		})
	}
}

func TestLocalRuntimeSettingsDefaultsContainNoOpt(t *testing.T) {
	got := localRuntimeSettings(func(string) string { return "" })
	if got.dataDir != "/data" {
		t.Errorf("dataDir = %q, want /data", got.dataDir)
	}
	if got.httpAddr != "0.0.0.0:2222" {
		t.Errorf("httpAddr = %q, want 0.0.0.0:2222", got.httpAddr)
	}
	if got.singboxBinary != "/usr/local/bin/sing-box" {
		t.Errorf("singboxBinary = %q, want /usr/local/bin/sing-box", got.singboxBinary)
	}
	if got.singboxConfigDir != "/data/sing-box/config.d" {
		t.Errorf("singboxConfigDir = %q, want /data/sing-box/config.d", got.singboxConfigDir)
	}
	if got.proxyAddr != "127.0.0.1:1080" {
		t.Errorf("proxyAddr = %q, want 127.0.0.1:1080", got.proxyAddr)
	}
	for name, value := range map[string]string{
		"dataDir": got.dataDir, "httpAddr": got.httpAddr,
		"singboxBinary": got.singboxBinary, "singboxConfigDir": got.singboxConfigDir,
	} {
		if strings.Contains(value, "/opt") {
			t.Errorf("%s local default contains /opt: %q", name, value)
		}
	}
}

func TestLocalRuntimeSettingsPreservesOverridesLiterally(t *testing.T) {
	env := map[string]string{
		"AWG_DATA_DIR":       "relative data",
		"AWG_HTTP_ADDR":      "127.0.0.1:9999 ",
		"SINGBOX_BIN":        "./custom sing-box",
		"SINGBOX_CONFIG_DIR": "./custom fragments",
		"AWG_PROXY_ADDR":     "0.0.0.0:2080",
	}
	got := localRuntimeSettings(func(key string) string { return env[key] })
	if got.dataDir != env["AWG_DATA_DIR"] {
		t.Errorf("dataDir = %q, want literal %q", got.dataDir, env["AWG_DATA_DIR"])
	}
	if got.httpAddr != env["AWG_HTTP_ADDR"] {
		t.Errorf("httpAddr = %q, want literal %q", got.httpAddr, env["AWG_HTTP_ADDR"])
	}
	if got.singboxBinary != env["SINGBOX_BIN"] {
		t.Errorf("singboxBinary = %q, want literal %q", got.singboxBinary, env["SINGBOX_BIN"])
	}
	if got.singboxConfigDir != env["SINGBOX_CONFIG_DIR"] {
		t.Errorf("singboxConfigDir = %q, want literal %q", got.singboxConfigDir, env["SINGBOX_CONFIG_DIR"])
	}
	if got.proxyAddr != env["AWG_PROXY_ADDR"] {
		t.Errorf("proxyAddr = %q, want literal %q", got.proxyAddr, env["AWG_PROXY_ADDR"])
	}
}

func TestLocalRuntimeSettingsDerivesConfigDirFromDataDir(t *testing.T) {
	env := map[string]string{"AWG_DATA_DIR": "/volume/custom"}
	got := localRuntimeSettings(func(key string) string { return env[key] })
	if got.singboxConfigDir != "/volume/custom/sing-box/config.d" {
		t.Fatalf("singboxConfigDir = %q, want derived path", got.singboxConfigDir)
	}
}
