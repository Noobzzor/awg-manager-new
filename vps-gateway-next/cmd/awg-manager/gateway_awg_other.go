//go:build !linux

package main

import (
	"fmt"
	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
)

func newGatewayUserspaceAWG(string, int, string, string, string, gatewayingress.AWGParameters) (gatewayOwnedAWGInterface, error) {
	return nil, fmt.Errorf("userspace AWG gateway requires Linux")
}
