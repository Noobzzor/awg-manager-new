package policy

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWARPConfigValidationAndEncryptedCredentials(t *testing.T) {
	cfg := WARPConfig{Tag: "warp", Endpoint: "engage.cloudflareclient.com:2408", MTU: 1280, IPv4: "100.64.0.2", IPv6: "2606:4700:110:8765::2"}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	key := []byte("01234567890123456789012345678901")
	ciphertext, err := EncryptCredentials(key, []byte("client-secret"))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := DecryptCredentials(key, ciphertext)
	if err != nil || string(plaintext) != "client-secret" {
		t.Fatalf("decrypt = %q, err=%v", plaintext, err)
	}
	if string(ciphertext) == "client-secret" {
		t.Fatal("credentials were not encrypted")
	}
}

func TestHealthRegistryURLTestChoosesHealthyLowestScore(t *testing.T) {
	r := NewHealthRegistry()
	ctx := context.Background()
	for _, item := range []struct {
		u   Upstream
		err error
	}{
		{Upstream{Tag: "slow", Kind: UpstreamVPN, Score: 200 * time.Millisecond}, nil},
		{Upstream{Tag: "fast", Kind: UpstreamWARP, Score: 50 * time.Millisecond}, nil},
		{Upstream{Tag: "broken", Kind: UpstreamVPN, Score: 1 * time.Millisecond}, errors.New("probe failed")},
	} {
		got := r.Check(ctx, item.u, func(context.Context, Upstream) error { return item.err })
		if item.err == nil && got.State != HealthHealthy {
			t.Fatalf("healthy probe state = %q", got.State)
		}
	}
	got, err := r.SelectUpstream(SelectorModeURLTest, "", []Upstream{
		{Tag: "slow", Score: 200 * time.Millisecond},
		{Tag: "fast", Score: 50 * time.Millisecond},
		{Tag: "broken", Score: 1 * time.Millisecond},
	})
	if err != nil || got.Tag != "fast" {
		t.Fatalf("selected = %+v, err=%v", got, err)
	}
}

func TestHealthRegistryNeverFallsBackToDirect(t *testing.T) {
	r := NewHealthRegistry()
	u := Upstream{Tag: "vpn", Kind: UpstreamVPN}
	r.Check(context.Background(), u, func(context.Context, Upstream) error { return errors.New("offline") })
	if _, err := r.SelectUpstream(SelectorModeExplicit, "vpn", []Upstream{u}); err == nil {
		t.Fatal("failed upstream was selected")
	}
	if _, err := r.SelectUpstream(SelectorModeURLTest, "", nil); err == nil {
		t.Fatal("empty set silently fell back")
	}
}

func TestHealthRegistryExplicitSelectionRejectsWrongTag(t *testing.T) {
	r := NewHealthRegistry()
	u := Upstream{Tag: "vpn-a", Kind: UpstreamVPN}
	r.Check(context.Background(), u, func(context.Context, Upstream) error { return nil })
	if _, err := r.SelectUpstream(SelectorModeExplicit, "vpn-b", []Upstream{u}); err == nil {
		t.Fatal("wrong explicit tag accepted")
	}
}
