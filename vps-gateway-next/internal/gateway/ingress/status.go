package ingress

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var ErrPeerNotFound = errors.New("gateway peer not found")

type PeerStatusReader interface {
	ReadPeerStatus(ctx context.Context, publicKey string) (PeerStatus, error)
}

type PeerStatus struct {
	PublicKey     string
	Endpoint      string
	LastHandshake time.Time
	Handshaken    bool
	RXBytes       uint64
	TXBytes       uint64
}

func ParsePeerDump(output, publicKey string) (PeerStatus, error) {
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 8 || fields[0] != publicKey {
			continue
		}
		seconds, err := strconv.ParseInt(fields[4], 10, 64)
		if err != nil {
			return PeerStatus{}, fmt.Errorf("parse handshake timestamp: %w", err)
		}
		rx, err := strconv.ParseUint(fields[5], 10, 64)
		if err != nil {
			return PeerStatus{}, fmt.Errorf("parse received bytes: %w", err)
		}
		tx, err := strconv.ParseUint(fields[6], 10, 64)
		if err != nil {
			return PeerStatus{}, fmt.Errorf("parse transmitted bytes: %w", err)
		}
		status := PeerStatus{PublicKey: fields[0], Endpoint: fields[2], RXBytes: rx, TXBytes: tx}
		if seconds > 0 {
			status.LastHandshake = time.Unix(seconds, 0)
			status.Handshaken = true
		}
		return status, nil
	}
	return PeerStatus{}, fmt.Errorf("%w: %s", ErrPeerNotFound, publicKey)
}

func (a *CommandApplier) ReadPeerStatus(ctx context.Context, publicKey string) (PeerStatus, error) {
	if a == nil || a.run == nil || a.binary == "" || a.interfaceName == "" {
		return PeerStatus{}, fmt.Errorf("awg command applier is not configured")
	}
	output, err := a.run(ctx, a.binary, "show", a.interfaceName, "dump")
	if err != nil {
		return PeerStatus{}, fmt.Errorf("read AWG peer status: %w", err)
	}
	return ParsePeerDump(output, publicKey)
}
