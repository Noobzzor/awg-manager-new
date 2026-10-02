// Package icmpprobe sends a single ICMP echo through a tunnel interface.
// Native replacement for `exec /opt/bin/ping -I <iface>`: fork/exec is
// prohibitively expensive on mipsel routers, and parsing ping stdout is
// less precise than timing the reply ourselves.
package icmpprobe

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/hoaxisr/awg-manager/internal/sys/httpclient"
)

const (
	replyTimeout = 5 * time.Second // parity with the old `ping -W 5`
	protoICMPv4  = 1               // iana.ProtocolICMP
)

var echoPayload = []byte("awg-manager-ping")

// echoID hands out per-probe identifiers: a raw ICMP socket receives every
// echo reply on the host, so concurrent probes for different tunnels must
// be distinguishable.
var echoID atomic.Uint32

type Result struct {
	LatencyMs int
}

// ByInterface sends one ICMP echo to target out of ifaceName and waits for
// the matching reply. dnsServers (tunnel DNS, may be nil) are used to
// resolve hostname targets via httpclient.ResolveIPv4Resilient.
func ByInterface(ctx context.Context, ifaceName, target string, dnsServers []string) (Result, error) {
	ipStr, err := httpclient.ResolveIPv4Resilient(ctx, target, dnsServers, ifaceName)
	if err != nil {
		return Result{}, err
	}
	dst := &net.IPAddr{IP: net.ParseIP(ipStr)}

	conn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return Result{}, fmt.Errorf("icmpprobe: raw socket: %w", err)
	}
	defer conn.Close()
	if err := bindToDevice(conn, ifaceName); err != nil {
		return Result{}, fmt.Errorf("icmpprobe: bind %s: %w", ifaceName, err)
	}

	id := int(echoID.Add(1) & 0xffff)
	wb, err := (&icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Body: &icmp.Echo{ID: id, Seq: 1, Data: echoPayload},
	}).Marshal(nil)
	if err != nil {
		return Result{}, fmt.Errorf("icmpprobe: marshal: %w", err)
	}

	deadline := time.Now().Add(replyTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)

	start := time.Now()
	if _, err := conn.WriteTo(wb, dst); err != nil {
		return Result{}, fmt.Errorf("icmpprobe: send: %w", err)
	}

	buf := make([]byte, 1500)
	for {
		n, peer, err := conn.ReadFrom(buf)
		if err != nil {
			return Result{}, fmt.Errorf("icmpprobe: no reply from %s: %w", ipStr, err)
		}
		peerAddr, ok := peer.(*net.IPAddr)
		if !ok || !matchEchoReply(buf[:n], id, 1, peerAddr.IP, dst.IP) {
			continue
		}
		latency := int(time.Since(start).Milliseconds())
		if latency < 1 {
			latency = 1
		}
		return Result{LatencyMs: latency}, nil
	}
}

// matchEchoReply reports whether b is the echo reply for (id, seq) from
// target. Filtering is mandatory: the raw socket sees replies belonging to
// concurrent probes of other tunnels and unrelated ICMP traffic.
func matchEchoReply(b []byte, id, seq int, src, target net.IP) bool {
	if src == nil || !src.Equal(target) {
		return false
	}
	m, err := icmp.ParseMessage(protoICMPv4, b)
	if err != nil {
		return false
	}
	if m.Type != ipv4.ICMPTypeEchoReply {
		return false
	}
	echo, ok := m.Body.(*icmp.Echo)
	return ok && echo.ID == id && echo.Seq == seq
}
