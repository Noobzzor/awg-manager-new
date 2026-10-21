//go:build linux

package main

import (
	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
	"os"
	"path/filepath"
	"strings"
)

func newGatewayUserspaceAWG(name string, port int, address, dataDir, tool string, params gatewayingress.AWGParameters) (gatewayOwnedAWGInterface, error) {
	runtime := strings.TrimSpace(os.Getenv("AWG_GATEWAY_RUNTIME_BINARY"))
	if runtime == "" {
		runtime = "/usr/local/bin/amneziawg-go"
	}
	ip := strings.TrimSpace(os.Getenv("AWG_GATEWAY_IP_BINARY"))
	if ip == "" {
		ip = "/sbin/ip"
	}
	return gatewayingress.NewUserspaceAWGInterface(gatewayingress.UserspaceAWGConfig{
		RuntimeBinary: runtime, ToolBinary: tool, IPBinary: ip, InterfaceName: name, ListenPort: port,
		Address: address, KeyDirectory: filepath.Join(dataDir, "gateway-runtime"), Parameters: params,
	}), nil
}
