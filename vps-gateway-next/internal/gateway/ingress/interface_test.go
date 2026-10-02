package ingress

import (
	"context"
	"errors"
	"testing"
)

func TestInterfaceManagerEnsureBuildsServerCommands(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, name string, args ...string) (string, error) {
		calls = append(calls, append([]string{name}, args...))
		return "", nil
	}
	m := NewInterfaceManager("/sbin/ip", "/opt/sbin/awg", "awg0", 51820, "server-private", run)
	m.SetAddress("10.66.0.1/24")
	if err := m.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 {
		t.Fatalf("calls=%#v", calls)
	}
	want := [][]string{
		{"/sbin/ip", "link", "add", "dev", "awg0", "type", "wireguard"},
		{"/sbin/ip", "address", "add", "10.66.0.1/24", "dev", "awg0"},
		{"/opt/sbin/awg", "set", "awg0", "private-key", "server-private", "listen-port", "51820"},
		{"/sbin/ip", "link", "set", "up", "dev", "awg0"},
	}
	for i := range want {
		if !same(calls[i], want[i]) {
			t.Fatalf("call %d=%#v want %#v", i, calls[i], want[i])
		}
	}
}

func TestInterfaceManagerEnsureAcceptsAddressAlreadyAssignedOnSameInterface(t *testing.T) {
	var calls [][]string
	run := func(_ context.Context, name string, args ...string) (string, error) {
		command := append([]string{name}, args...)
		calls = append(calls, command)
		if same(command, []string{"/sbin/ip", "link", "add", "dev", "awg0", "type", "wireguard"}) {
			return "RTNETLINK answers: File exists", errors.New("exit status 2")
		}
		if same(command, []string{"/sbin/ip", "address", "add", "10.66.0.1/24", "dev", "awg0"}) {
			return "Error: ipv4: Address already assigned.", errors.New("exit status 2")
		}
		if same(command, []string{"/sbin/ip", "-o", "address", "show", "dev", "awg0"}) {
			return "3: awg0 inet 10.66.0.1/24 scope global awg0", nil
		}
		return "", nil
	}
	m := NewInterfaceManager("/sbin/ip", "/opt/sbin/awg", "awg0", 51820, "server-private", run)
	m.SetAddress("10.66.0.1/24")
	if err := m.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"/sbin/ip", "link", "add", "dev", "awg0", "type", "wireguard"},
		{"/sbin/ip", "address", "add", "10.66.0.1/24", "dev", "awg0"},
		{"/sbin/ip", "-o", "address", "show", "dev", "awg0"},
		{"/opt/sbin/awg", "set", "awg0", "private-key", "server-private", "listen-port", "51820"},
		{"/sbin/ip", "link", "set", "up", "dev", "awg0"},
	}
	if len(calls) != len(want) {
		t.Fatalf("command count=%d want %d", len(calls), len(want))
	}
	for i := range want {
		if !same(calls[i], want[i]) {
			t.Fatalf("command %d=%#v want %#v", i, calls[i], want[i])
		}
	}
}

func TestInterfaceManagerEnsureDoesNotIgnoreAddressAssignedElsewhere(t *testing.T) {
	run := func(_ context.Context, name string, args ...string) (string, error) {
		command := append([]string{name}, args...)
		if same(command, []string{"/sbin/ip", "link", "add", "dev", "awg0", "type", "wireguard"}) {
			return "RTNETLINK answers: File exists", errors.New("exit status 2")
		}
		if same(command, []string{"/sbin/ip", "address", "add", "10.66.0.1/24", "dev", "awg0"}) {
			return "Error: ipv4: Address already assigned.", errors.New("exit status 2")
		}
		if same(command, []string{"/sbin/ip", "-o", "address", "show", "dev", "awg0"}) {
			return "3: awg0 inet 10.66.0.1/32 scope global awg0", nil
		}
		return "", nil
	}
	m := NewInterfaceManager("/sbin/ip", "/opt/sbin/awg", "awg0", 51820, "server-private", run)
	m.SetAddress("10.66.0.1/24")
	if err := m.Ensure(context.Background()); err == nil {
		t.Fatal("expected address conflict to fail when the exact configured CIDR is absent from awg0")
	}
}

func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
