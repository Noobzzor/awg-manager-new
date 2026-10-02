package ingress

import (
	"context"
	"net/netip"
	"testing"
)

func TestCommandApplierBuildsAWGPeerCommands(t *testing.T) {
	var got [][]string
	runner := func(_ context.Context, name string, args ...string) (string, error) {
		got = append(got, append([]string{name}, args...))
		return "", nil
	}
	applier := NewCommandApplier("/opt/sbin/awg", "awg0", runner)
	peer := PeerConfig{ClientID: "client-a", PublicKey: "public", AllowedIP: netip.MustParsePrefix("10.66.0.1/32"), presharedKey: "psk"}
	if err := applier.ApplyPeer(context.Background(), peer); err != nil {
		t.Fatal(err)
	}
	if err := applier.RemovePeer(context.Background(), "public"); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0][0] != "/opt/sbin/awg" || got[0][1] != "set" || got[0][2] != "awg0" || got[0][3] != "peer" || got[0][4] != "public" || got[0][5] != "preshared-key" || got[0][6] != "psk" || got[0][7] != "allowed-ips" || got[0][8] != "10.66.0.1/32" {
		t.Fatalf("apply command = %#v", got)
	}
	if got[1][1] != "set" || got[1][3] != "peer" || got[1][5] != "remove" {
		t.Fatalf("remove command = %#v", got[1])
	}
}
