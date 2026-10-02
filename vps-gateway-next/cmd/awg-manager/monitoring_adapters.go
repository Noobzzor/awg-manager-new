package main

import (
	"context"
	"strings"

	"github.com/hoaxisr/awg-manager/internal/monitoring"
	"github.com/hoaxisr/awg-manager/internal/tunnel/systemtunnel"
)

// monitoringSystemTunnelAdapter adapts systemtunnel.Service to
// monitoring.SystemTunnelLister (a small typed view of just the fields the
// monitoring scheduler needs).
type monitoringSystemTunnelAdapter struct {
	svc *systemtunnel.ServiceImpl
}

func (s *monitoringSystemTunnelAdapter) List(ctx context.Context) ([]monitoring.SystemTunnelInfo, error) {
	if s == nil || s.svc == nil {
		return nil, nil
	}
	list, err := s.svc.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]monitoring.SystemTunnelInfo, 0, len(list))
	for _, t := range list {
		// Skip awg-manager's own server interface (tagged with the
		// ManagedServerDescription prefix "AWGM ..."; see
		// internal/managed/types.go::ManagedServerDescription). Server-side
		// WG is not a client tunnel and must not appear in monitoring.
		if strings.HasPrefix(t.Description, "AWGM") {
			continue
		}
		out = append(out, monitoring.SystemTunnelInfo{
			ID:            t.ID,
			InterfaceName: t.InterfaceName,
			Description:   t.Description,
			Connected:     t.Connected,
		})
	}
	return out, nil
}
