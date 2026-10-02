package icmpprobe

import (
	"net"
	"testing"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func marshalEcho(t *testing.T, typ icmp.Type, id, seq int) []byte {
	t.Helper()
	b, err := (&icmp.Message{
		Type: typ,
		Body: &icmp.Echo{ID: id, Seq: seq, Data: echoPayload},
	}).Marshal(nil)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestMatchEchoReply(t *testing.T) {
	target := net.ParseIP("8.8.8.8")
	reply := marshalEcho(t, ipv4.ICMPTypeEchoReply, 42, 1)

	cases := []struct {
		name string
		buf  []byte
		id   int
		seq  int
		src  net.IP
		want bool
	}{
		{"valid reply", reply, 42, 1, target, true},
		{"wrong id", reply, 43, 1, target, false},
		{"wrong seq", reply, 42, 2, target, false},
		{"wrong source", reply, 42, 1, net.ParseIP("1.1.1.1"), false},
		{"nil source", reply, 42, 1, nil, false},
		{"echo request not reply", marshalEcho(t, ipv4.ICMPTypeEcho, 42, 1), 42, 1, target, false},
		{"garbage", []byte{0xde, 0xad}, 42, 1, target, false},
		{"empty", nil, 42, 1, target, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchEchoReply(tc.buf, tc.id, tc.seq, tc.src, target); got != tc.want {
				t.Fatalf("matchEchoReply = %v, want %v", got, tc.want)
			}
		})
	}
}
