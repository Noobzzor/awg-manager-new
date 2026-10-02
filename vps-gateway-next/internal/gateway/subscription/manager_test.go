package subscription

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

type memoryStore struct {
	records []StoredSubscription
	saveErr error
}

func (s *memoryStore) Load() ([]StoredSubscription, error) {
	return append([]StoredSubscription(nil), s.records...), nil
}

func (s *memoryStore) Save(records []StoredSubscription) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.records = append([]StoredSubscription(nil), records...)
	return nil
}

func TestCreateIssuesOpaqueTokenAndStoresOnlyHash(t *testing.T) {
	store := &memoryStore{}
	manager, err := NewManager(store)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := manager.Create("phone-1", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(issued.Token) != 43 || strings.ContainsAny(issued.Token, "+/= ") {
		t.Fatalf("token is not URL-safe 256-bit base64: %q", issued.Token)
	}
	resolved, err := manager.Resolve(issued.Token)
	if err != nil || resolved.ClientID != "phone-1" || resolved.ID != issued.Subscription.ID {
		t.Fatalf("Resolve() = %+v, %v", resolved, err)
	}
	metadataJSON, err := json.Marshal(issued.Subscription)
	if err != nil {
		t.Fatal(err)
	}
	storedJSON, err := json.Marshal(store.records)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(metadataJSON), issued.Token) || strings.Contains(string(storedJSON), issued.Token) {
		t.Fatal("raw token was exposed in metadata or persistence")
	}
	if len(store.records) != 1 || store.records[0].TokenHash == "" {
		t.Fatalf("expected a hashed token record, got %+v", store.records)
	}
}

func TestResolveRejectsRevokedAndExpiredTokens(t *testing.T) {
	manager, err := NewManager(&memoryStore{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }
	issued, err := manager.Create("phone-1", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Revoke(issued.Subscription.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Resolve(issued.Token); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("revoked token error = %v, want ErrUnavailable", err)
	}

	second, err := manager.Create("tablet-1", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Hour)
	if _, err := manager.Resolve(second.Token); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("expired token error = %v, want ErrUnavailable", err)
	}
}

func TestPersistenceFailureDoesNotCommitCreateOrRevoke(t *testing.T) {
	store := &memoryStore{}
	manager, err := NewManager(store)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	manager.now = func() time.Time { return now }
	issued, err := manager.Create("phone-1", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	store.saveErr = errors.New("disk unavailable")
	if _, err := manager.Create("tablet-1", now.Add(time.Hour)); err == nil {
		t.Fatal("Create() succeeded despite failed persistence")
	}
	if len(manager.List()) != 1 {
		t.Fatalf("failed Create() changed memory state: %+v", manager.List())
	}
	if err := manager.Revoke(issued.Subscription.ID); err == nil {
		t.Fatal("Revoke() succeeded despite failed persistence")
	}
	if _, err := manager.Resolve(issued.Token); err != nil {
		t.Fatalf("failed Revoke() changed memory state: %v", err)
	}
}

func TestManagerRestoresTokenHashesFromStore(t *testing.T) {
	store := &memoryStore{}
	first, err := NewManager(store)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := first.Create("phone-1", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewManager(store)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := second.Resolve(issued.Token)
	if err != nil || resolved.ClientID != "phone-1" {
		t.Fatalf("restored Resolve() = %+v, %v", resolved, err)
	}
}
