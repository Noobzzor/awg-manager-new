package ingress

import (
	"testing"
	"time"
)

func TestParsePeerDumpReadsHandshakeAndTraffic(t *testing.T) {
	output := "interface-private interface-public 51820 0\n" +
		"peer-public peer-psk 203.0.113.4:51820 10.66.0.1/32 1700000000 1234 5678 25\n"
	status, err := ParsePeerDump(output, "peer-public")
	if err != nil {
		t.Fatal(err)
	}
	if !status.Handshaken || !status.LastHandshake.Equal(time.Unix(1700000000, 0)) || status.RXBytes != 1234 || status.TXBytes != 5678 {
		t.Fatalf("status = %+v", status)
	}
}

func TestParsePeerDumpReturnsUnknownForMissingPeer(t *testing.T) {
	if _, err := ParsePeerDump("interface-private interface-public 51820 0\n", "missing"); err == nil {
		t.Fatal("expected missing peer error")
	}
}
