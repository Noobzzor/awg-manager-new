package subscription

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// breakStorePath подменяет путь store на путь под обычным ФАЙЛОМ: AtomicWrite
// падает на MkdirAll (ENOTDIR) даже под root — надёжная симуляция ошибки
// записи на диск. Возвращает функцию восстановления пути.
func breakStorePath(t *testing.T, path *string) func() {
	t.Helper()
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	orig := *path
	*path = filepath.Join(blocker, "store.json")
	return func() { *path = orig }
}

func newCOWTestSub(t *testing.T, s *Store) *Subscription {
	t.Helper()
	sub, err := s.Create(CreateInput{
		Label:   "cow",
		URL:     "https://example.com/sub.txt",
		Headers: []Header{{Name: "User-Agent", Value: "test"}},
		Enabled: true,
		Mode:    ModeURLTest,
	})
	if err != nil {
		t.Fatal(err)
	}
	members := []MemberInfo{
		{Tag: "sub-x-1", Label: "A", Protocol: "vless", Server: "a.example", Port: 443},
		{Tag: "sub-x-2", Label: "B", Protocol: "vless", Server: "b.example", Port: 443},
	}
	if err := s.SetMembers(sub.ID, members, []string{"sub-x-orphan"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetRejectedAndInfo(sub.ID,
		[]RejectedMember{{Tag: "sub-x-bad", Reason: "invalid"}},
		[]SubscriptionInfoItem{{ID: "i1", Label: "info"}}); err != nil {
		t.Fatal(err)
	}
	return sub
}

// TestStore_MutatorsSaveFailure_MemoryUnchanged: ошибка AtomicWrite не должна
// оставлять в памяти незаписанное состояние — иначе память и диск расходятся
// до перезапуска, а следующая успешная запись чего угодно молча уносит
// несохранённое изменение (copy-on-write в Store.mutate).
func TestStore_MutatorsSaveFailure_MemoryUnchanged(t *testing.T) {
	s, err := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	if err != nil {
		t.Fatal(err)
	}
	sub := newCOWTestSub(t, s)

	before, err := s.Get(sub.ID)
	if err != nil {
		t.Fatal(err)
	}

	restore := breakStorePath(t, &s.path)

	label := "new-label"
	mutations := map[string]func() error{
		"Update": func() error {
			_, err := s.Update(sub.ID, UpdatePatch{Label: &label})
			return err
		},
		"UpdateState": func() error {
			return s.UpdateState(sub.ID, RefreshResult{})
		},
		"SetMembers": func() error {
			return s.SetMembers(sub.ID, nil, nil)
		},
		"MoveToExcluded": func() error {
			return s.MoveToExcluded(sub.ID, before.Members[:1], []string{"sub-x-2"}, before.Members[1:])
		},
		"SetExcludedTags": func() error {
			return s.SetExcludedTags(sub.ID, []string{"sub-x-1"}, nil)
		},
		"SetMembersExtras": func() error {
			return s.SetMembersExtras(sub.ID, nil, nil, nil, nil, nil, nil)
		},
		"RemoveInfoItem": func() error {
			return s.RemoveInfoItem(sub.ID, "i1")
		},
		"SetRejectedAndInfo": func() error {
			return s.SetRejectedAndInfo(sub.ID, nil, nil)
		},
		"SetProxyIndex": func() error {
			return s.SetProxyIndex(sub.ID, 7)
		},
		"SetMembership": func() error {
			return s.SetMembership(sub.ID, []string{"sub-x-2"}, nil)
		},
		"SetActiveMember": func() error {
			return s.SetActiveMember(sub.ID, "sub-x-2")
		},
		"SetListenPort": func() error {
			return s.SetListenPort(sub.ID, 11999)
		},
	}
	for name, fn := range mutations {
		if err := fn(); err == nil {
			t.Errorf("%s: expected save error, got nil", name)
		}
		after, err := s.Get(sub.ID)
		if err != nil {
			t.Fatalf("%s: Get: %v", name, err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Errorf("%s: memory diverged from pre-call state after failed save:\nbefore: %+v\nafter:  %+v", name, before, after)
		}
	}

	// Delete при упавшей записи не должен терять строку из памяти.
	if err := s.Delete(sub.ID); err == nil {
		t.Error("Delete: expected save error, got nil")
	}
	if _, err := s.Get(sub.ID); err != nil {
		t.Errorf("Delete: subscription vanished from memory after failed save: %v", err)
	}

	// После восстановления пути мутация проходит и переживает перезагрузку.
	restore()
	if err := s.SetActiveMember(sub.ID, "sub-x-2"); err != nil {
		t.Fatalf("SetActiveMember after restore: %v", err)
	}
	reloaded, err := NewStore(s.path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reloaded.Get(sub.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ActiveMember != "sub-x-2" {
		t.Errorf("ActiveMember=%q want %q after reload", got.ActiveMember, "sub-x-2")
	}
}

// TestSubscription_CloneIndependence: правки slice-полей клона не должны быть
// видны в оригинале сквозь общие backing-массивы.
func TestSubscription_CloneIndependence(t *testing.T) {
	orig := &Subscription{
		ID:               "id1",
		Headers:          []Header{{Name: "H", Value: "V"}},
		MemberTags:       []string{"t1", "t2"},
		Members:          []MemberInfo{{Tag: "t1"}, {Tag: "t2"}},
		OrphanTags:       []string{"o1"},
		RejectedMembers:  []RejectedMember{{Tag: "r1", Reason: "x"}},
		InfoItems:        []SubscriptionInfoItem{{ID: "i1"}},
		DismissedInfoIDs: []string{"d1"},
		ExcludedTags:     []string{"e1"},
		ExcludedMembers:  []MemberInfo{{Tag: "e1"}},
		FilteredMembers:  []MemberInfo{{Tag: "f1"}},
		URLTest:          &URLTestConfig{URL: "u", IntervalSec: 1, ToleranceMs: 2},
	}
	cp := orig.clone()
	cp.Headers[0].Value = "mutated"
	cp.MemberTags[0] = "mutated"
	cp.Members[0].Tag = "mutated"
	cp.OrphanTags[0] = "mutated"
	cp.RejectedMembers[0].Reason = "mutated"
	cp.InfoItems[0].ID = "mutated"
	cp.DismissedInfoIDs[0] = "mutated"
	cp.ExcludedTags[0] = "mutated"
	cp.ExcludedMembers[0].Tag = "mutated"
	cp.FilteredMembers[0].Tag = "mutated"
	cp.URLTest.URL = "mutated"

	if orig.Headers[0].Value != "V" || orig.MemberTags[0] != "t1" || orig.Members[0].Tag != "t1" ||
		orig.OrphanTags[0] != "o1" || orig.RejectedMembers[0].Reason != "x" || orig.InfoItems[0].ID != "i1" ||
		orig.DismissedInfoIDs[0] != "d1" || orig.ExcludedTags[0] != "e1" || orig.ExcludedMembers[0].Tag != "e1" ||
		orig.FilteredMembers[0].Tag != "f1" || orig.URLTest.URL != "u" {
		t.Errorf("clone shares backing storage with original: %+v", orig)
	}

	// nil-срезы остаются nil (JSON-форма omitempty не меняется).
	empty := &Subscription{ID: "id2"}
	if got := empty.clone(); got.Members != nil || got.Headers != nil || got.URLTest != nil {
		t.Errorf("clone of zero subscription materialized nil fields: %+v", got)
	}
}

// TestGroupStore_MutatorsSaveFailure_MemoryUnchanged — зеркало copy-on-write
// теста для GroupStore.
func TestGroupStore_MutatorsSaveFailure_MemoryUnchanged(t *testing.T) {
	s, err := NewGroupStore(filepath.Join(t.TempDir(), "groups.json"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := s.Create(GroupCreateInput{
		Label:              "cow",
		UseSubscriptionIDs: []string{"sub-a", "sub-b"},
		Enabled:            true,
	})
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.Get(g.ID)
	if err != nil {
		t.Fatal(err)
	}

	restore := breakStorePath(t, &s.path)

	label := "new-label"
	mutations := map[string]func() error{
		"Update": func() error {
			_, err := s.Update(g.ID, GroupUpdatePatch{Label: &label})
			return err
		},
		"SetListenPort": func() error {
			return s.SetListenPort(g.ID, 11999)
		},
		"SetProxyIndex": func() error {
			return s.SetProxyIndex(g.ID, 7)
		},
		"RemoveSubscriptionRef": func() error {
			return s.RemoveSubscriptionRef("sub-a")
		},
	}
	for name, fn := range mutations {
		if err := fn(); err == nil {
			t.Errorf("%s: expected save error, got nil", name)
		}
		after, err := s.Get(g.ID)
		if err != nil {
			t.Fatalf("%s: Get: %v", name, err)
		}
		if !reflect.DeepEqual(before, after) {
			t.Errorf("%s: memory diverged from pre-call state after failed save:\nbefore: %+v\nafter:  %+v", name, before, after)
		}
	}

	if err := s.Delete(g.ID); err == nil {
		t.Error("Delete: expected save error, got nil")
	}
	if _, err := s.Get(g.ID); err != nil {
		t.Errorf("Delete: group vanished from memory after failed save: %v", err)
	}

	// Create при упавшей записи не должен оставлять строку в памяти.
	if _, err := s.Create(GroupCreateInput{Label: "fail"}); err == nil {
		t.Error("Create: expected save error, got nil")
	}
	if got := s.List(); len(got) != 1 {
		t.Errorf("List after failed Create: %d groups, want 1", len(got))
	}

	restore()
	if err := s.RemoveSubscriptionRef("sub-a"); err != nil {
		t.Fatalf("RemoveSubscriptionRef after restore: %v", err)
	}
	got, err := s.Get(g.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.UseSubscriptionIDs, []string{"sub-b"}) {
		t.Errorf("UseSubscriptionIDs=%v want [sub-b]", got.UseSubscriptionIDs)
	}
}
