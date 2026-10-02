package pingcheck

import (
	"context"
	"fmt"
	"time"

	"github.com/hoaxisr/awg-manager/internal/httpprobe"
	"github.com/hoaxisr/awg-manager/internal/icmpprobe"
)

const (
	checkTimeout = 7 * time.Second
)

// checkHTTP performs HTTP 204 connectivity check through the tunnel.
func checkHTTP(ctx context.Context, ifaceName string, checkURL string, dnsServers []string) CheckResult {
	res, err := httpprobe.ByInterface(ctx, ifaceName, checkURL, dnsServers)
	if err != nil {
		return CheckResult{
			Success: false,
			Error:   fmt.Sprintf("HTTP check failed: %v", err),
		}
	}

	if httpprobe.SuccessCode(res.HTTPCode) {
		return CheckResult{
			Success: true,
			Latency: res.LatencyMs,
		}
	}

	return CheckResult{
		Success: false,
		Latency: res.LatencyMs,
		Error:   fmt.Sprintf("unexpected HTTP code: %d", res.HTTPCode),
	}
}

// checkICMP performs a native ICMP ping check through the tunnel interface.
func checkICMP(ctx context.Context, ifaceName string, target string, dnsServers []string) CheckResult {
	checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	start := time.Now()
	res, err := icmpprobe.ByInterface(checkCtx, ifaceName, target, dnsServers)
	if err != nil {
		return CheckResult{
			Success: false,
			Latency: int(time.Since(start).Milliseconds()),
			Error:   fmt.Sprintf("ping failed: %v", err),
		}
	}

	return CheckResult{
		Success: true,
		Latency: res.LatencyMs,
	}
}

// performCheck executes the appropriate check method using the resolved interface name.
func performCheck(ctx context.Context, ifaceName string, method string, target string, checkURL string, dnsServers []string) CheckResult {
	switch method {
	case "icmp":
		return checkICMP(ctx, ifaceName, target, dnsServers)
	default: // "http" is default
		return checkHTTP(ctx, ifaceName, checkURL, dnsServers)
	}
}
