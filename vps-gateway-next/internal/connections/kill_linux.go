//go:build linux

package connections

import (
	"encoding/binary"
	"fmt"
	"net"
	"syscall"
)

// KillParams identifies a conntrack entry by its original-direction 5-tuple.
type KillParams struct {
	Src      string
	Dst      string
	SrcPort  int
	DstPort  int
	Protocol string // "tcp" | "udp"
}

// ctnetlink-константы из linux/netfilter/nfnetlink_conntrack.h (в syscall их нет).
const (
	nfnlSubsysCtnetlink = 1
	ipctnlMsgCtDelete   = 2

	ctaTupleOrig  = 1
	ctaTupleIP    = 1
	ctaTupleProto = 2

	ctaIPv4Src = 1
	ctaIPv4Dst = 2
	ctaIPv6Src = 3
	ctaIPv6Dst = 4

	ctaProtoNum     = 1
	ctaProtoSrcPort = 2
	ctaProtoDstPort = 3

	nlaFNested = 0x8000
)

// Kill deletes the conntrack entry matching the original-direction tuple via
// ctnetlink (IPCTNL_MSG_CT_DELETE). На Keenetic нет бинаря `conntrack` и
// записи в /proc — netlink единственный путь. ENOENT — не ошибка: запись
// могла истечь между кликом в UI и удалением (идемпотентность).
func Kill(p KillParams) error {
	var protoNum byte
	switch p.Protocol {
	case "tcp":
		protoNum = syscall.IPPROTO_TCP
	case "udp":
		protoNum = syscall.IPPROTO_UDP
	default:
		return fmt.Errorf("unsupported protocol %q", p.Protocol)
	}
	srcIP := net.ParseIP(p.Src)
	dstIP := net.ParseIP(p.Dst)
	if srcIP == nil || dstIP == nil {
		return fmt.Errorf("invalid src/dst address")
	}
	src4, dst4 := srcIP.To4(), dstIP.To4()
	if (src4 != nil) != (dst4 != nil) {
		return fmt.Errorf("src/dst address family mismatch")
	}

	var ipAttrs []byte
	family := byte(syscall.AF_INET)
	if src4 != nil {
		ipAttrs = append(ipAttrs, nlAttr(ctaIPv4Src, src4)...)
		ipAttrs = append(ipAttrs, nlAttr(ctaIPv4Dst, dst4)...)
	} else {
		family = syscall.AF_INET6
		ipAttrs = append(ipAttrs, nlAttr(ctaIPv6Src, srcIP.To16())...)
		ipAttrs = append(ipAttrs, nlAttr(ctaIPv6Dst, dstIP.To16())...)
	}

	sport := make([]byte, 2)
	dport := make([]byte, 2)
	binary.BigEndian.PutUint16(sport, uint16(p.SrcPort)) // порты — network order
	binary.BigEndian.PutUint16(dport, uint16(p.DstPort))
	var protoAttrs []byte
	protoAttrs = append(protoAttrs, nlAttr(ctaProtoNum, []byte{protoNum})...)
	protoAttrs = append(protoAttrs, nlAttr(ctaProtoSrcPort, sport)...)
	protoAttrs = append(protoAttrs, nlAttr(ctaProtoDstPort, dport)...)

	tuple := append(nlAttr(nlaFNested|ctaTupleIP, ipAttrs),
		nlAttr(nlaFNested|ctaTupleProto, protoAttrs)...)
	// nfgenmsg{family, version=NFNETLINK_V0(0), res_id(be16)=0} + CTA_TUPLE_ORIG
	body := append([]byte{family, 0, 0, 0}, nlAttr(nlaFNested|ctaTupleOrig, tuple)...)

	return nlTransact(nfnlSubsysCtnetlink<<8|ipctnlMsgCtDelete, body)
}

// nlTransact sends one netfilter-netlink request with NLM_F_ACK and reads
// the ack. ENOENT is swallowed (see Kill).
func nlTransact(msgType int, body []byte) error {
	fd, err := syscall.Socket(syscall.AF_NETLINK, syscall.SOCK_RAW, syscall.NETLINK_NETFILTER)
	if err != nil {
		return fmt.Errorf("netlink socket: %w", err)
	}
	defer syscall.Close(fd)
	if err := syscall.Bind(fd, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return fmt.Errorf("netlink bind: %w", err)
	}

	msg := make([]byte, syscall.NLMSG_HDRLEN+len(body))
	// nlmsghdr — host byte order (на mips это big-endian, поэтому NativeEndian).
	binary.NativeEndian.PutUint32(msg[0:4], uint32(len(msg)))
	binary.NativeEndian.PutUint16(msg[4:6], uint16(msgType))
	binary.NativeEndian.PutUint16(msg[6:8], syscall.NLM_F_REQUEST|syscall.NLM_F_ACK)
	binary.NativeEndian.PutUint32(msg[8:12], 1) // seq
	copy(msg[syscall.NLMSG_HDRLEN:], body)

	if err := syscall.Sendto(fd, msg, 0, &syscall.SockaddrNetlink{Family: syscall.AF_NETLINK}); err != nil {
		return fmt.Errorf("netlink send: %w", err)
	}

	buf := make([]byte, 4096)
	n, _, err := syscall.Recvfrom(fd, buf, 0)
	if err != nil {
		return fmt.Errorf("netlink recv: %w", err)
	}
	msgs, err := syscall.ParseNetlinkMessage(buf[:n])
	if err != nil {
		return fmt.Errorf("netlink parse: %w", err)
	}
	for _, m := range msgs {
		if m.Header.Type == syscall.NLMSG_ERROR && len(m.Data) >= 4 {
			errno := int32(binary.NativeEndian.Uint32(m.Data[:4]))
			switch {
			case errno == 0:
				return nil // ack
			case -errno == int32(syscall.ENOENT):
				return nil // запись уже истекла — идемпотентно
			default:
				return fmt.Errorf("conntrack delete: errno %d", -errno)
			}
		}
	}
	return nil
}

// nlAttr encodes one netlink attribute with 4-byte padding.
func nlAttr(typ int, data []byte) []byte {
	l := 4 + len(data)
	buf := make([]byte, (l+3) & ^3)
	binary.NativeEndian.PutUint16(buf[0:2], uint16(l))
	binary.NativeEndian.PutUint16(buf[2:4], uint16(typ))
	copy(buf[4:], data)
	return buf
}
