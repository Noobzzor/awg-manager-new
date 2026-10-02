package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Issue #428: a feed listing the same endpoint twice (identical identity,
// different labels) produced duplicate preview keys. The frontend renders the
// preview with a keyed each over member.key, so a duplicate key crashed the
// render (each_key_duplicate) and froze the add-subscription wizard on
// «Загрузка...». PreviewURL must dedupe by key exactly like ApplyDiff does
// on refresh (SkippedDuplicate) — the refresh creates one member anyway.
func TestPreviewURL_DedupesByExclusionKey(t *testing.T) {
	const link = "vless://11111111-2222-3333-4444-555555555555@a.example.com:443?security=tls&sni=a.example.com&type=tcp"
	body := link + "#Server%20A\n" + link + "#Server%20A%20(backup)\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	svc := NewService(nil, nil)
	members, err := svc.PreviewURL(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("PreviewURL: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 deduped member, got %d: %+v", len(members), members)
	}
	seen := map[string]struct{}{}
	for _, m := range members {
		if m.Key == "" {
			t.Errorf("member has empty key: %+v", m)
		}
		if _, dup := seen[m.Key]; dup {
			t.Errorf("duplicate preview key %q", m.Key)
		}
		seen[m.Key] = struct{}{}
	}
}

// Distinct endpoints must NOT be collapsed by the dedupe.
func TestPreviewURL_KeepsDistinctMembers(t *testing.T) {
	body := "vless://11111111-2222-3333-4444-555555555555@a.example.com:443?security=tls&type=tcp#A\n" +
		"vless://11111111-2222-3333-4444-555555555555@b.example.com:443?security=tls&type=tcp#B\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	svc := NewService(nil, nil)
	members, err := svc.PreviewURL(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatalf("PreviewURL: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d: %+v", len(members), members)
	}
	if members[0].Key == members[1].Key {
		t.Fatalf("distinct servers must have distinct keys, both %q", members[0].Key)
	}
}
