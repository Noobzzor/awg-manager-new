package ingress

import (
	"context"
	"net/netip"
	"testing"
)

func TestCommandApplierPeerCommandDoesNotChangeInterfacePrivateKey(t *testing.T) {
	var got [][]string
	applier := NewCommandApplier("/usr/bin/wg", "awg0", func(_ context.Context, name string, args ...string) (string, error) {
		got = append(got, append([]string{name}, args...))
		return "", nil
	})
	peer := PeerConfig{ClientID: "phone", PublicKey: "client-public", AllowedIP: netip.MustParsePrefix("10.66.0.2/32")}
	if err := applier.ApplyPeer(context.Background(), peer); err != nil {
		t.Fatal(err)
	}
	want := []string{"/usr/bin/wg", "set", "awg0", "peer", "client-public", "allowed-ips", "10.66.0.2/32"}
	if len(got) != 1 || !same(got[0], want) {
		t.Fatalf("peer command=%#v want %#v", got, want)
	}
	for _, arg := range got[0] {
		if arg == "private-key" || arg == "/run/awg-manager/server-private.key" {
			t.Fatalf("peer operation touched server private key: %#v", got[0])
		}
	}
}
