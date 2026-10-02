package httpclient

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"testing"
)

func TestEncodeDNSQuery(t *testing.T) {
	q, err := encodeDNSQuery("raw.githubusercontent.com")
	if err != nil {
		t.Fatal(err)
	}
	if len(q) < 20 {
		t.Fatalf("query too short: %d", len(q))
	}
	// QDCOUNT must be 1 — the packet carries one question. A query that omits
	// it (QDCOUNT=0 with a question present) is malformed and strict resolvers
	// reject it with FORMERR/rcode 1 (#239: download-via-AWG-tunnel DNS).
	if qd := binary.BigEndian.Uint16(q[4:6]); qd != 1 {
		t.Errorf("QDCOUNT = %d, want 1", qd)
	}
	// Sanity: ANCOUNT/NSCOUNT/ARCOUNT are 0 in a query.
	for name, off := range map[string]int{"ANCOUNT": 6, "NSCOUNT": 8, "ARCOUNT": 10} {
		if v := binary.BigEndian.Uint16(q[off : off+2]); v != 0 {
			t.Errorf("%s = %d, want 0", name, v)
		}
	}
}

func buildDNSResponse(rrType uint16, rdata []byte) []byte {
	q := []byte{7, 'e', 'x', 'a', 'm', 'p', 'l', 'e', 3, 'c', 'o', 'm', 0}
	ansName := []byte{0xc0, 0x0c}
	ansTail := make([]byte, 10+len(rdata))
	binary.BigEndian.PutUint16(ansTail[0:2], rrType)
	binary.BigEndian.PutUint16(ansTail[2:4], 1)
	binary.BigEndian.PutUint32(ansTail[4:8], 60)
	binary.BigEndian.PutUint16(ansTail[8:10], uint16(len(rdata)))
	copy(ansTail[10:], rdata)

	out := make([]byte, 12+len(q)+4+len(ansName)+len(ansTail))
	out[2] = 0x81
	binary.BigEndian.PutUint16(out[6:8], 1)
	copy(out[12:], q)
	off := 12 + len(q)
	binary.BigEndian.PutUint16(out[off:], 1)
	binary.BigEndian.PutUint16(out[off+2:], 1)
	off += 4
	copy(out[off:], ansName)
	off += len(ansName)
	copy(out[off:], ansTail)
	return out
}

func TestParseDNSARecord_directA(t *testing.T) {
	pkt := buildDNSResponse(1, net.IPv4(1, 2, 3, 4).To4())
	ip, err := parseDNSARecord(context.Background(), pkt, "1.1.1.1", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if ip != "1.2.3.4" {
		t.Fatalf("ip = %q, want 1.2.3.4", ip)
	}
}

func buildDNSCNAMEResponse(target string) []byte {
	return buildDNSResponse(5, encodeDNSNameBytes(target))
}

func encodeDNSNameBytes(name string) []byte {
	var out []byte
	for _, label := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		out = append(out, byte(len(label)))
		out = append(out, label...)
	}
	return append(out, 0)
}

func TestParseDNSCNAMETargets(t *testing.T) {
	pkt := buildDNSCNAMEResponse("d111111.cloudfront.net")
	got, err := parseDNSCNAMETargets(pkt)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "d111111.cloudfront.net" {
		t.Fatalf("got %v", got)
	}
}
