package diagnostics

import (
	"encoding/json"
	"fmt"
)

// collectSingboxConfig fetches the merged sing-box config, sanitizes it, and returns the result.
func (r *Runner) collectSingboxConfig() *SingboxConfigInfo {
	if r.deps.SingboxConfigPreview == nil {
		return nil
	}

	raw, err := r.deps.SingboxConfigPreview()
	if err != nil {
		return &SingboxConfigInfo{
			Available: false,
			Error:     err.Error(),
		}
	}

	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return &SingboxConfigInfo{
			Available: false,
			Error:     fmt.Sprintf("parse merged sing-box config: %v", err),
		}
	}

	sanitizeSingboxConfig(cfg)

	return &SingboxConfigInfo{
		Available: true,
		Config:    cfg,
	}
}

// sanitizeSingboxConfig recursively walks the merged sing-box config and replaces
// sensitive values (UUIDs, credentials, server addresses, REALITY keys, short IDs,
// server names, DNS server hosts, and similar) with deterministic aliases. Same real
// value → same alias within a single config so structural relationships are preserved.
func sanitizeSingboxConfig(cfg map[string]any) {
	if cfg == nil {
		return
	}

	uuidSubs := make(map[string]string)
	uuidCount := 0

	dnsAddrSubs := make(map[string]string)
	dnsAddrCount := 0

	pubkeySubs := make(map[string]string)
	pubkeyCount := 0

	shortIDSubs := make(map[string]string)
	shortIDCount := 0

	serverSubs := make(map[string]string)
	serverCount := 0

	serverNameSubs := make(map[string]string)
	serverNameCount := 0

	sensitiveKeys := map[string]bool{
		"uuid":        true,
		"password":    true,
		"private_key": true,
		"public_key":  true,
		"short_id":    true,
		"server":      true,
		"server_port": true,
		"server_name": true,
		"address":     true,
		"host":        true,
		"domain":      true,
	}

	var knownKey func(inDNSServers bool, k string, v any, val map[string]any)
	knownKey = func(inDNSServers bool, k string, v any, val map[string]any) {
		switch k {
		case "uuid":
			if s, ok := v.(string); ok && s != "" {
				if alias, ok := uuidSubs[s]; ok {
					val[k] = alias
				} else {
					uuidCount++
					alias := fmt.Sprintf("UUID-%d", uuidCount)
					uuidSubs[s] = alias
					val[k] = alias
				}
			}
		case "password", "private_key":
			val[k] = "[REDACTED]"
		case "public_key":
			if s, ok := v.(string); ok && s != "" {
				if alias, ok := pubkeySubs[s]; ok {
					val[k] = alias
				} else {
					pubkeyCount++
					alias := fmt.Sprintf("REALITY-PUBKEY-%d", pubkeyCount)
					pubkeySubs[s] = alias
					val[k] = alias
				}
			}
		case "short_id":
			if s, ok := v.(string); ok && s != "" {
				if alias, ok := shortIDSubs[s]; ok {
					val[k] = alias
				} else {
					shortIDCount++
					alias := fmt.Sprintf("SHORT-ID-%d", shortIDCount)
					shortIDSubs[s] = alias
					val[k] = alias
				}
			}
		case "server":
			if s, ok := v.(string); ok && s != "" {
				if alias, ok := serverSubs[s]; ok {
					val[k] = alias
				} else {
					serverCount++
					alias := fmt.Sprintf("SERVER-%d", serverCount)
					serverSubs[s] = alias
					val[k] = alias
				}
			}
		case "server_port":
			val[k] = 0
		case "server_name":
			if s, ok := v.(string); ok && s != "" {
				if alias, ok := serverNameSubs[s]; ok {
					val[k] = alias
				} else {
					serverNameCount++
					alias := fmt.Sprintf("SNI-%d", serverNameCount)
					serverNameSubs[s] = alias
					val[k] = alias
				}
			}
		case "address", "host", "domain":
			if s, ok := v.(string); ok && s != "" {
				if inDNSServers {
					if alias, ok := dnsAddrSubs[s]; ok {
						val[k] = alias
					} else {
						dnsAddrCount++
						alias := fmt.Sprintf("DNS-SERVER-%d", dnsAddrCount)
						dnsAddrSubs[s] = alias
						val[k] = alias
					}
				}
			}
		}
	}

	// walk recursively sanitizes all sensitive leaf keys in-place.
	var walk func(v any, inDNSServers bool)
	walk = func(v any, inDNSServers bool) {
		switch val := v.(type) {
		case map[string]any:
			for k, child := range val {
				if sensitiveKeys[k] {
					knownKey(inDNSServers, k, child, val)
					continue
				}
				// Set inDNSServers flag when descending into dns.servers array.
				if k == "servers" {
					walk(child, true)
					continue
				}
				// Non-sensitive keys: recurse only if child is a nested structure.
				switch c := child.(type) {
				case map[string]any:
					walk(c, inDNSServers)
				case []any:
					walk(c, inDNSServers)
				}
			}
		case []any:
			for _, item := range val {
				if m, ok := item.(map[string]any); ok {
					walk(m, inDNSServers)
				}
			}
		}
	}

	walk(cfg, false)
}
