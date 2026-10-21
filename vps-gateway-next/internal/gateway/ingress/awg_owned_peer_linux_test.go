//go:build linux

package ingress

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const ownedPeerFixtureKey = "AQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func ownedPeerContracts(t *testing.T, m *UserspaceAWGInterface) (PeerApplier, PeerStatusReader) {
	t.Helper()
	applier, ok := any(m).(PeerApplier)
	if !ok {
		t.Fatal("owned AWG owner does not control peer lifecycle")
	}
	reader, ok := any(m).(PeerStatusReader)
	if !ok {
		t.Fatal("owned AWG owner does not read peer state")
	}
	return applier, reader
}

// Real child/socket ownership with injected configured/protocol observations;
// this fixture is NOT a real AWG process or traffic proof.
func TestOwnedAWGPeerControlAndPublicStatus(t *testing.T) {
	m, _ := newAWGControlFixture(t)
	if err := m.Preflight(context.Background()); err != nil {
		t.Fatal(err)
	}
	applier, reader := ownedPeerContracts(t, m)
	m.configured = true
	calls := []string{}
	m.run = func(_ context.Context, _ string, args ...string) (string, error) {
		calls = append(calls, strings.Join(args, " "))
		if args[0] == "set" {
			return "", nil
		}
		switch args[2] {
		case "endpoints":
			return ownedPeerFixtureKey + " 127.0.0.1:50000\n", nil
		case "latest-handshakes":
			return ownedPeerFixtureKey + " 1234\n", nil
		case "transfer":
			return ownedPeerFixtureKey + " 42 99\n", nil
		default:
			t.Fatal("status requested a secret/dump or unsupported field")
			return "", nil
		}
	}
	peer := PeerConfig{PublicKey: ownedPeerFixtureKey, AllowedIP: netip.MustParsePrefix("10.66.0.2/32")}
	if err := applier.ApplyPeer(context.Background(), peer); err != nil {
		t.Fatal(err)
	}
	status, err := reader.ReadPeerStatus(context.Background(), peer.PublicKey)
	if err != nil || status.Endpoint != "127.0.0.1:50000" || !status.Handshaken || status.LastHandshake.Unix() != 1234 || status.RXBytes != 42 || status.TXBytes != 99 {
		t.Fatalf("public peer state lost: %+v %v", status, err)
	}
	if err := applier.RemovePeer(context.Background(), peer.PublicKey); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 5 || calls[0] != "set awg0 peer "+peer.PublicKey+" allowed-ips 10.66.0.2/32" || calls[4] != "set awg0 peer "+peer.PublicKey+" remove" {
		t.Fatal("wrong peer command contract")
	}
}

func TestOwnedAWGPeerControlRefusesLostOwnership(t *testing.T) {
	for _, kind := range []string{"unconfigured", "closed", "exited", "replaced"} {
		t.Run(kind, func(t *testing.T) {
			m, _ := newAWGControlFixture(t, kind == "replaced")
			if err := m.Preflight(context.Background()); err != nil {
				t.Fatal(err)
			}
			applier, reader := ownedPeerContracts(t, m)
			m.configured = kind != "unconfigured"
			switch kind {
			case "closed":
				if err := m.Close(context.Background()); err != nil {
					t.Fatal(err)
				}
			case "exited":
				if err := m.child.cmd.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				<-m.child.done
			case "replaced":
				if err := os.Rename(m.socketPath, m.socketPath+".owned"); err != nil {
					t.Fatal(err)
				}
				foreign, err := net.ListenUnix("unix", &net.UnixAddr{Name: m.socketPath, Net: "unix"})
				if err != nil {
					t.Fatal(err)
				}
				foreign.SetUnlinkOnClose(false)
				defer foreign.Close()
			}
			m.run = func(context.Context, string, ...string) (string, error) {
				t.Fatal("peer operation touched unowned/unconfigured runtime")
				return "", nil
			}
			peer := PeerConfig{PublicKey: ownedPeerFixtureKey, AllowedIP: netip.MustParsePrefix("10.66.0.2/32")}
			if err := applier.ApplyPeer(context.Background(), peer); err == nil {
				t.Fatal("apply accepted")
			}
			if err := applier.RemovePeer(context.Background(), peer.PublicKey); err == nil {
				t.Fatal("remove accepted")
			}
			if _, err := reader.ReadPeerStatus(context.Background(), peer.PublicKey); err == nil {
				t.Fatal("status accepted")
			}
		})
	}
}

func TestOwnedAWGPeerControlRejectsSecretAndInvalidInput(t *testing.T) {
	m, _ := newAWGControlFixture(t)
	if err := m.Preflight(context.Background()); err != nil {
		t.Fatal(err)
	}
	applier, reader := ownedPeerContracts(t, m)
	m.configured = true
	m.run = func(context.Context, string, ...string) (string, error) {
		t.Fatal("invalid peer reached tool")
		return "", nil
	}
	for _, peer := range []PeerConfig{
		{PublicKey: "not-a-key", AllowedIP: netip.MustParsePrefix("10.66.0.2/32")},
		{PublicKey: ownedPeerFixtureKey, AllowedIP: netip.MustParsePrefix("10.66.0.2/24")},
		{PublicKey: ownedPeerFixtureKey, AllowedIP: netip.MustParsePrefix("::1/128")},
		{PublicKey: ownedPeerFixtureKey, AllowedIP: netip.MustParsePrefix("10.66.0.2/32"), presharedKey: "synthetic-secret"},
	} {
		if err := applier.ApplyPeer(context.Background(), peer); err == nil || strings.Contains(err.Error(), "synthetic-secret") {
			t.Fatal("invalid/secret input accepted or leaked")
		}
	}
	if err := applier.RemovePeer(context.Background(), "bad-key"); err == nil {
		t.Fatal("bad remove key accepted")
	}
	if _, err := reader.ReadPeerStatus(context.Background(), "bad-key"); err == nil {
		t.Fatal("bad status key accepted")
	}
}

func TestOwnedAWGPublicStatusMissingAndMalformed(t *testing.T) {
	for _, kind := range []string{"absent", "bad-number", "duplicate", "partial"} {
		t.Run(kind, func(t *testing.T) {
			m, _ := newAWGControlFixture(t)
			if err := m.Preflight(context.Background()); err != nil {
				t.Fatal(err)
			}
			_, reader := ownedPeerContracts(t, m)
			m.configured = true
			m.run = func(_ context.Context, _ string, args ...string) (string, error) {
				if kind == "absent" || kind == "partial" && args[2] == "transfer" {
					return "", nil
				}
				row := ownedPeerFixtureKey + " 0\n"
				if args[2] == "transfer" {
					row = ownedPeerFixtureKey + " 0 0\n"
				}
				if kind == "bad-number" && args[2] == "latest-handshakes" {
					row = ownedPeerFixtureKey + " secret-invalid-number\n"
				}
				if kind == "duplicate" {
					row += row
				}
				return row, nil
			}
			_, err := reader.ReadPeerStatus(context.Background(), ownedPeerFixtureKey)
			if err == nil || strings.Contains(err.Error(), "secret-invalid-number") {
				t.Fatal("malformed status accepted or raw output leaked")
			}
			if kind == "absent" && !errors.Is(err, ErrPeerNotFound) {
				t.Fatal("missing peer classification lost")
			}
		})
	}
}

// Exercise the actual subprocess stdout path, with valid aggregate public rows
// exceeding4KiB and the requested peer last; no real AWG traffic claim.
func TestOwnedAWGPublicStatusLargeRegistry(t *testing.T) {
	m, _ := newAWGControlFixture(t)
	if err := m.Preflight(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, reader := ownedPeerContracts(t, m)
	m.configured = true
	dir := t.TempDir()
	tool := filepath.Join(dir, "public-tool")
	script := "#!/bin/sh\ncase \"$3\" in endpoints|latest-handshakes|transfer) cat \"$0.$3\" ;; *) exit 19 ;; esac\n"
	if err := os.WriteFile(tool, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"endpoints", "latest-handshakes", "transfer"} {
		var rows strings.Builder
		for i := 2; i < 302; i++ {
			raw := make([]byte, 32)
			raw[0] = byte(i)
			raw[1] = byte(i >> 8)
			key := base64.StdEncoding.EncodeToString(raw)
			value := "0"
			if field == "endpoints" {
				value = "127.0.0.1:50001"
			}
			if field == "transfer" {
				value = "0 0"
			}
			fmt.Fprintf(&rows, "%s %s\n", key, value)
		}
		value := "1234"
		if field == "endpoints" {
			value = "127.0.0.1:50000"
		}
		if field == "transfer" {
			value = "42 99"
		}
		fmt.Fprintf(&rows, "%s %s\n", ownedPeerFixtureKey, value)
		if rows.Len() <= 4096 {
			t.Fatal("large output setup invalid")
		}
		if err := os.WriteFile(tool+"."+field, []byte(rows.String()), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m.config.ToolBinary = tool
	m.run = runUserspaceAWGCommand
	m.statusRun = runUserspaceAWGSelectedPublicField
	status, err := reader.ReadPeerStatus(context.Background(), ownedPeerFixtureKey)
	if err != nil || status.RXBytes != 42 || status.TXBytes != 99 || !status.Handshaken {
		t.Fatalf("large valid registry lost chosen peer: %+v %v", status, err)
	}
}
