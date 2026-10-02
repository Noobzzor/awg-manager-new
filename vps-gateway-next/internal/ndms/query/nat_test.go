package query

import (
	"context"
	"testing"
)

func TestNATStore_HasInterface(t *testing.T) {
	fg := newFakeGetter()
	fg.SetJSON("/show/rc/ip/nat", `[
		{"interface":"Wireguard0"},
		{"interface":"Home"}
	]`)
	s := NewNATStore(fg, nopLogger{})

	ok, err := s.HasInterface(context.Background(), "Wireguard0")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected NAT on Wireguard0")
	}

	ok, err = s.HasInterface(context.Background(), "Wireguard9")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("expected no NAT on Wireguard9")
	}
}
