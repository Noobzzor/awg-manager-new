package subscription

import (
	"context"
	"testing"
)

func TestService_ReconcileRestoresMissingSubscriptionSlot(t *testing.T) {
	svc, mut := newTestService(t)
	sub := createInlineSubWithTwoMembers(t, svc)

	// Simulate a durable store surviving while 40-subscriptions.json was lost
	// or restored empty. The service must rebuild the selector and members from
	// the persisted inline source before serving requests.
	mut.bodies = map[string][]byte{}
	mut.declaredTags = []string{}
	mut.reloads = 0

	if err := svc.Reconcile(context.Background()); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if !mut.addedOutbound(sub.SelectorTag) {
		t.Fatalf("selector %q was not restored; added=%v", sub.SelectorTag, mut.addedOutbounds)
	}
	if mut.reloads == 0 {
		t.Fatal("Reconcile must commit the rebuilt subscription slot")
	}
	if len(selectorMembers(t, mut, sub.SelectorTag)) != len(sub.MemberTags) {
		t.Fatalf("restored selector members=%v want %v", selectorMembers(t, mut, sub.SelectorTag), sub.MemberTags)
	}
}
