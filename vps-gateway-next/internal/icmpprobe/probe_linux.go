//go:build linux

package icmpprobe

import (
	"fmt"
	"net"
	"strings"
	"syscall"
)

// bindToDevice pins the raw socket egress to iface via SO_BINDTODEVICE
// (privileged; awg-manager runs as root on the router — same assumption as
// httpclient's bindDialer).
func bindToDevice(conn net.PacketConn, iface string) error {
	iface = strings.TrimSpace(iface)
	if iface == "" {
		return nil
	}
	ipConn, ok := conn.(*net.IPConn)
	if !ok {
		return fmt.Errorf("unexpected conn type %T", conn)
	}
	sc, err := ipConn.SyscallConn()
	if err != nil {
		return err
	}
	var setErr error
	if err := sc.Control(func(fd uintptr) {
		setErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface)
	}); err != nil {
		return err
	}
	return setErr
}
