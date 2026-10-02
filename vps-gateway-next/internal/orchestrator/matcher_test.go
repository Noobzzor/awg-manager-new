package orchestrator

import (
	"testing"
	"time"
)

func TestConsumeExpectedHook_ExpiredNotConsumed(t *testing.T) {
	base := time.Unix(1000, 0)
	cur := base
	o := &Orchestrator{clock: func() time.Time { return cur }}

	o.ExpectHook("Wireguard2", "disabled") // expires at base+TTL

	cur = base.Add(expectedHookTTL + time.Second) // advance past TTL

	if o.consumeExpectedHook("Wireguard2", "disabled") {
		t.Fatal("expired expected-hook must not be consumed (would absorb a real later edge)")
	}
}

func TestConsumeExpectedHook_FreshConsumedOnce(t *testing.T) {
	base := time.Unix(1000, 0)
	cur := base
	o := &Orchestrator{clock: func() time.Time { return cur }}

	o.ExpectHook("Wireguard2", "running")
	cur = base.Add(5 * time.Second)

	if !o.consumeExpectedHook("Wireguard2", "running") {
		t.Fatal("fresh expected-hook must be consumed")
	}
	if o.consumeExpectedHook("Wireguard2", "running") {
		t.Fatal("expected-hook must be consumed only once")
	}
}

func TestConsumeExpectedHook_PrunesExpiredButKeepsFresh(t *testing.T) {
	base := time.Unix(1000, 0)
	cur := base
	o := &Orchestrator{clock: func() time.Time { return cur }}

	o.ExpectHook("Wireguard2", "disabled") // stale soon
	cur = base.Add(expectedHookTTL + time.Second)
	o.ExpectHook("Wireguard2", "disabled") // fresh, registered after advance

	if !o.consumeExpectedHook("Wireguard2", "disabled") {
		t.Fatal("fresh expected-hook must still be consumable after pruning")
	}
	if o.consumeExpectedHook("Wireguard2", "disabled") {
		t.Fatal("only one fresh expectation existed")
	}
}
