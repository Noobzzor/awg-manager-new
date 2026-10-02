package testing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hoaxisr/awg-manager/internal/httpprobe"
	"github.com/hoaxisr/awg-manager/internal/icmpprobe"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/storage"
	"github.com/hoaxisr/awg-manager/internal/sys/exec"
)

// CheckConnectivity performs quick connectivity test through tunnel.
func (s *Service) CheckConnectivity(ctx context.Context, tunnelID string) (*ConnectivityResult, error) {
	if err := s.CheckTunnelRunning(tunnelID); err != nil {
		s.appLog.Debug("connectivity-check", tunnelID, "Tunnel not running")
		// Сброс серии: после перезапуска туннеля первый отказ снова
		// логируется как Warn, а не как «повтор».
		s.connTracker.Forget(tunnelID)
		return &ConnectivityResult{Connected: false, Reason: ReasonTunnelNotRunning}, nil
	}

	stored := s.GetAWG(tunnelID)
	method := "http"
	if stored != nil && stored.ConnectivityCheck != nil && stored.ConnectivityCheck.Method != "" {
		method = stored.ConnectivityCheck.Method
	}

	s.appLog.Full("connectivity-check", tunnelID, fmt.Sprintf("Starting connectivity check with method: %s", method))

	var result *ConnectivityResult
	var err error
	switch method {
	case "ping":
		result, err = s.checkPing(ctx, tunnelID, stored)
	case "handshake":
		result, err = s.checkHandshake(tunnelID)
	case "disabled":
		s.appLog.Debug("connectivity-check", tunnelID, "Check disabled, returning OK")
		// Выключенная проверка не наблюдается трекером — сбрасываем серию,
		// чтобы после включения первый отказ снова дал Warn, а не «повтор».
		s.connTracker.Forget(tunnelID)
		return &ConnectivityResult{Connected: true, Reason: "check disabled"}, nil
	default:
		result, err = s.checkHTTP(ctx, tunnelID)
	}
	if err == nil && result != nil {
		s.logConnectivityOutcome(method, tunnelID, result)
	}
	return result, err
}

// logConnectivityOutcome логирует исход проверки по переходной модели:
// повторяющиеся периодические проверки (авто-чеки фронта) не спамят журнал —
// Warn пишется на переходе в отказ, Info на восстановлении с длиной серии,
// повторы того же исхода видны только на debug.
func (s *Service) logConnectivityOutcome(method, tunnelID string, res *ConnectivityResult) {
	// Единая метка: method приходит из хранимого конфига и не whitelisted —
	// производная метка вида "<произвольное>-check" дробила бы фильтрацию.
	const action = "connectivity-check"
	latency := ""
	if res.Latency != nil {
		latency = fmt.Sprintf(", latency=%dms", *res.Latency)
	}
	switch obs := s.connTracker.Observe(tunnelID, res.Connected); obs.Kind {
	case logging.TransitionNowFailing:
		s.appLog.Warn(action, tunnelID, fmt.Sprintf("Connectivity check (%s) failed: %s", method, res.Reason))
	case logging.TransitionStillFailing:
		s.appLog.Debug(action, tunnelID, fmt.Sprintf("Connectivity check (%s) still failing (%d in a row): %s", method, obs.Failures, res.Reason))
	case logging.TransitionRecovered:
		s.appLog.Info(action, tunnelID, fmt.Sprintf("Connectivity restored after %d failed checks%s", obs.Failures, latency))
	default: // FirstOK / StillOK
		s.appLog.Debug(action, tunnelID, fmt.Sprintf("Connectivity check (%s) ok%s", method, latency))
	}
}

// checkHTTP performs connectivity check using HTTP through the tunnel.
func (s *Service) checkHTTP(ctx context.Context, tunnelID string) (*ConnectivityResult, error) {
	iface, err := s.GetInterfaceName(tunnelID)
	if err != nil {
		s.appLog.Debug("http-check", tunnelID, "Tunnel not running, cannot get interface name")
		return &ConnectivityResult{Connected: false, Reason: ReasonTunnelNotRunning}, nil
	}

	checkURL := s.connectivityCheckURL()
	s.appLog.Full("http-check", tunnelID, fmt.Sprintf("Executing HTTP check: %s", checkURL))

	res, err := httpprobe.ByInterface(ctx, iface, checkURL, nil)
	if err != nil {
		errDetail := err.Error()
		return &ConnectivityResult{Connected: false, Reason: ReasonConnectionFailed + ": " + errDetail}, nil
	}

	s.appLog.Debug("http-check", tunnelID, fmt.Sprintf("HTTP check result: code=%d, latency=%dms", res.HTTPCode, res.LatencyMs))

	latencyMs := res.LatencyMs
	if httpprobe.SuccessCode(res.HTTPCode) {
		return &ConnectivityResult{Connected: true, Latency: &latencyMs}, nil
	}

	return &ConnectivityResult{Connected: false, Reason: fmt.Sprintf("%s: code=%d", ReasonUnexpectedResponse, res.HTTPCode), HTTPCode: &res.HTTPCode}, nil
}

// checkPing performs connectivity check using ICMP ping through the tunnel interface.
func (s *Service) checkPing(ctx context.Context, tunnelID string, stored *storage.AWGTunnel) (*ConnectivityResult, error) {
	iface := s.resolveIfaceName(tunnelID)

	target := ""
	if stored != nil && stored.ConnectivityCheck != nil {
		target = stored.ConnectivityCheck.PingTarget
	}
	if target == "" {
		target = autoDetectGateway(stored)
	}
	if target == "" {
		return &ConnectivityResult{Connected: false, Reason: "no ping target configured"}, nil
	}

	s.appLog.Full("ping-check", tunnelID, fmt.Sprintf("Starting ping check: iface=%s, target=%s", iface, target))

	// 3s wait — parity with the old `ping -W 3`.
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	res, err := icmpprobe.ByInterface(pingCtx, iface, target, nil)
	if err != nil {
		return &ConnectivityResult{Connected: false, Reason: "ping failed: " + target + " - " + err.Error()}, nil
	}

	return &ConnectivityResult{Connected: true, Latency: intPtr(res.LatencyMs)}, nil
}

// intPtr returns a pointer to an int.
func intPtr(i int) *int {
	return &i
}

func (s *Service) connectivityCheckURL() string {
	if s == nil || s.settings == nil {
		return storage.DefaultConnectivityCheckURL
	}
	settings, err := s.settings.Get()
	if err != nil || settings == nil || strings.TrimSpace(settings.ConnectivityCheckURL) == "" {
		return storage.DefaultConnectivityCheckURL
	}
	return strings.TrimSpace(settings.ConnectivityCheckURL)
}

// autoDetectGateway derives a likely gateway IP from the tunnel address (e.g. 10.0.0.2/32 → 10.0.0.1).
func autoDetectGateway(stored *storage.AWGTunnel) string {
	if stored == nil || stored.Interface.Address == "" {
		return ""
	}
	addr := stored.Interface.Address
	if idx := strings.Index(addr, "/"); idx > 0 {
		addr = addr[:idx]
	}
	if idx := strings.Index(addr, ","); idx > 0 {
		addr = strings.TrimSpace(addr[:idx])
	}
	parts := strings.Split(addr, ".")
	if len(parts) != 4 {
		return ""
	}
	parts[3] = "1"
	return strings.Join(parts, ".")
}

// checkHandshake checks if WireGuard has a recent handshake (< 3 minutes).
func (s *Service) checkHandshake(tunnelID string) (*ConnectivityResult, error) {
	iface := s.resolveIfaceName(tunnelID)

	s.appLog.Full("handshake-check", tunnelID, fmt.Sprintf("Checking WireGuard handshake on interface %s", iface))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := exec.Run(ctx, "/opt/sbin/awg", "show", iface)
	if err != nil {
		s.appLog.Debug("handshake-check", tunnelID, fmt.Sprintf("Cannot read WG state: %v, stdout=%s, stderr=%s", err, result.Stdout, result.Stderr))
		return &ConnectivityResult{Connected: false, Reason: "cannot read WG state"}, nil
	}

	s.appLog.Debug("handshake-check", tunnelID, fmt.Sprintf("awg show output: %s", result.Stdout))

	for _, line := range strings.Split(result.Stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "latest handshake:") {
			continue
		}
		hs := strings.TrimSpace(strings.TrimPrefix(line, "latest handshake:"))
		if hs == "(none)" || hs == "" {
			return &ConnectivityResult{Connected: false, Reason: "no handshake"}, nil
		}
		if strings.Contains(hs, "hour") || strings.Contains(hs, "day") {
			return &ConnectivityResult{Connected: false, Reason: "handshake stale: " + hs}, nil
		}
		if strings.Contains(hs, "minute") {
			var mins int
			fmt.Sscanf(hs, "%d minute", &mins)
			if mins >= 3 {
				return &ConnectivityResult{Connected: false, Reason: "handshake stale: " + hs}, nil
			}
		}
		s.appLog.Debug("handshake-check", tunnelID, fmt.Sprintf("Handshake recent: %s", hs))
		return &ConnectivityResult{Connected: true}, nil
	}

	return &ConnectivityResult{Connected: false, Reason: "no handshake info"}, nil
}

// CheckConnectivityByInterfaceURL performs connectivity test using a kernel
// interface name directly and the supplied HTTP check URL.
func CheckConnectivityByInterfaceURL(ctx context.Context, ifaceName string, checkURL string) *ConnectivityResult {
	res, err := httpprobe.ByInterface(ctx, ifaceName, checkURL, nil)
	if err != nil {
		return &ConnectivityResult{
			Connected: false,
			Reason:    ReasonConnectionFailed,
		}
	}

	latencyMs := res.LatencyMs
	if httpprobe.SuccessCode(res.HTTPCode) {
		return &ConnectivityResult{
			Connected: true,
			Latency:   &latencyMs,
		}
	}

	return &ConnectivityResult{
		Connected: false,
		Reason:    ReasonUnexpectedResponse,
		HTTPCode:  &res.HTTPCode,
	}
}
