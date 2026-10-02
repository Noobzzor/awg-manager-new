//go:build !linux

package icmpprobe

import "net"

// bindToDevice is a stub for non-Linux builds (dev/test on macOS, Windows).
// SO_BINDTODEVICE is Linux-only; interface binding is silently skipped —
// mirrors httpclient's bindDialer stub.
func bindToDevice(_ net.PacketConn, _ string) error {
	return nil
}
