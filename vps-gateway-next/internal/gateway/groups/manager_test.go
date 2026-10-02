package groups

import (
	"testing"
	"time"
)

type memoryStore struct {
	groups []Group
	saves  int
}

func (s *memoryStore) Load() ([]Group, error) {
	return cloneGroups(s.groups), nil
}

func (s *memoryStore) Save(groups []Group) error {
	s.groups = cloneGroups(groups)
	s.saves++
	return nil
}

func TestCreatePersistsExistingClientMembership(t *testing.T) {
	store := &memoryStore{}
	manager, err := NewManager(store, func(clientID string) bool { return clientID == "phone" })
	if err != nil {
		t.Fatal(err)
	}

	created, err := manager.Create("Family", []string{"phone"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Name != "Family" || len(created.ClientIDs) != 1 || created.ClientIDs[0] != "phone" {
		t.Fatalf("unexpected group: %+v", created)
	}
	if store.saves != 1 || len(store.groups) != 1 || store.groups[0].ID != created.ID {
		t.Fatalf("group was not persisted: saves=%d groups=%+v", store.saves, store.groups)
	}
}

func TestUpdatePersistsMembershipAndPreservesCreationTime(t *testing.T) {
	store := &memoryStore{}
	manager, err := NewManager(store, func(clientID string) bool {
		return clientID == "phone" || clientID == "tablet"
	})
	if err != nil {
		t.Fatal(err)
	}
	createdAt := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return createdAt }
	created, err := manager.Create("Family", []string{"phone"})
	if err != nil {
		t.Fatal(err)
	}
	updatedAt := createdAt.Add(time.Minute)
	manager.now = func() time.Time { return updatedAt }

	updated, err := manager.Update(created.ID, "Household", []string{"tablet"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Household" || len(updated.ClientIDs) != 1 || updated.ClientIDs[0] != "tablet" {
		t.Fatalf("unexpected updated group: %+v", updated)
	}
	if !updated.CreatedAt.Equal(createdAt) || !updated.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("unexpected update timestamps: created=%s updated=%s", updated.CreatedAt, updated.UpdatedAt)
	}
	if store.saves != 2 || len(store.groups) != 1 || store.groups[0].ClientIDs[0] != "tablet" {
		t.Fatalf("update was not persisted: saves=%d groups=%+v", store.saves, store.groups)
	}
}

func TestGetAndListReturnIndependentMembershipSlices(t *testing.T) {
	manager, err := NewManager(&memoryStore{}, func(clientID string) bool { return clientID == "phone" })
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.Create("Family", []string{"phone"})
	if err != nil {
		t.Fatal(err)
	}
	created.ClientIDs[0] = "changed"

	got, ok := manager.Get(created.ID)
	if !ok || got.ClientIDs[0] != "phone" {
		t.Fatalf("Get returned missing or aliased group: ok=%v group=%+v", ok, got)
	}
	got.ClientIDs[0] = "changed-again"
	listed := manager.List()
	if len(listed) != 1 || listed[0].ClientIDs[0] != "phone" {
		t.Fatalf("List returned missing or aliased groups: %+v", listed)
	}
}

func TestDeletePersistsGroupRemoval(t *testing.T) {
	store := &memoryStore{}
	manager, err := NewManager(store, func(clientID string) bool { return clientID == "phone" })
	if err != nil {
		t.Fatal(err)
	}
	created, err := manager.Create("Family", []string{"phone"})
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Delete(created.ID); err != nil {
		t.Fatal(err)
	}
	if _, exists := manager.Get(created.ID); exists || len(store.groups) != 0 || store.saves != 2 {
		t.Fatalf("delete did not persist removal: exists=%v saves=%d groups=%+v", exists, store.saves, store.groups)
	}
}
