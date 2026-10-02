package awg3endpoint

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

func TestStore_CRUD(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "awg3.json"))
	rec := Record{ID: "id1", Tag: "AMS", Endpoint: json.RawMessage(`{"type":"awg"}`)}
	if err := s.Add(rec); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(); len(list) != 1 || list[0].Tag != "AMS" {
		t.Fatalf("list = %v", list)
	}
	if !s.Tags()["AMS"] {
		t.Fatal("Tags missing AMS")
	}
	if err := s.Rename("id1", "AMS2"); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get("id1"); got.Tag != "AMS2" {
		t.Fatalf("rename failed: %v", got)
	}
	if err := s.Delete("id1"); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.List(); len(list) != 0 {
		t.Fatalf("delete failed: %v", list)
	}
}

// awg3.json хранит приватные ключи — файл должен писаться с правами 0600.
func TestStore_FilePerm0600(t *testing.T) {
	p := filepath.Join(t.TempDir(), "awg3.json")
	s := NewStore(p)
	if err := s.Add(Record{ID: "id1", Tag: "T", Endpoint: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0600 {
		t.Fatalf("perm = %o, want 600", perm)
	}
}

func TestStore_Persists(t *testing.T) {
	p := filepath.Join(t.TempDir(), "awg3.json")
	s := NewStore(p)
	_ = s.Add(Record{ID: "id1", Tag: "T", Endpoint: json.RawMessage(`{}`)})
	if list, _ := NewStore(p).List(); len(list) != 1 {
		t.Fatal("not persisted")
	}
}

func TestStore_WriteFailureLeavesCacheAndDiskUnchanged(t *testing.T) {
	initial := []Record{
		{ID: "id1", Tag: "AMS", Endpoint: json.RawMessage(`{"type":"awg"}`)},
		{ID: "id2", Tag: "BER", Endpoint: json.RawMessage(`{"type":"awg"}`)},
	}

	tests := []struct {
		name   string
		mutate func(*Store) error
	}{
		{name: "add", mutate: func(s *Store) error {
			return s.Add(Record{ID: "id3", Tag: "LON", Endpoint: json.RawMessage(`{"type":"awg"}`)})
		}},
		{name: "delete", mutate: func(s *Store) error { return s.Delete("id1") }},
		{name: "rename", mutate: func(s *Store) error { return s.Rename("id1", "PAR") }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "awg3.json")
			s := NewStore(path)
			for _, rec := range initial {
				if err := s.Add(rec); err != nil {
					t.Fatal(err)
				}
			}
			beforeDisk, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			s.writeAtomic = func(string, []byte, os.FileMode) error {
				return errors.New("injected atomic write failure")
			}

			if err := tc.mutate(s); err == nil {
				t.Fatal("mutation succeeded with blocked durable path")
			}
			got, err := s.List()
			if err != nil {
				t.Fatalf("List after rejected mutation: %v", err)
			}
			if !reflect.DeepEqual(got, initial) {
				t.Fatalf("cache after rejected mutation = %+v, want %+v", got, initial)
			}
			for _, want := range initial {
				if rec, ok := s.Get(want.ID); !ok || !reflect.DeepEqual(rec, want) {
					t.Fatalf("Get(%q) after rejected mutation = %+v, %v; want %+v, true", want.ID, rec, ok, want)
				}
			}
			afterDisk, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(afterDisk, beforeDisk) {
				t.Fatalf("disk changed after rejected mutation:\nbefore=%s\nafter=%s", beforeDisk, afterDisk)
			}
		})
	}
}

func TestStoreReplacePublishesExactCandidateOnlyAfterDurableWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "awg3.json")
	s := NewStore(path)
	before := []Record{
		{ID: "one", Tag: "one", Endpoint: json.RawMessage(`{"opaque": { "spacing": true }}`)},
		{ID: "two", Tag: "two", Endpoint: json.RawMessage(`{"unknown":[3,2,1]}`)},
	}
	if err := s.Replace(before); err != nil {
		t.Fatal(err)
	}
	candidate := []Record{
		before[1],
		{ID: "three", Tag: "three", Endpoint: json.RawMessage(`{"future_field":"kept"}`)},
		before[0],
	}

	s.writeAtomic = func(string, []byte, os.FileMode) error { return errors.New("disk full") }
	if err := s.Replace(candidate); err == nil {
		t.Fatal("Replace succeeded despite durable write failure")
	}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, before) {
		t.Fatalf("failed Replace published cache=%+v, want %+v", got, before)
	}

	s.writeAtomic = storage.AtomicWritePerm
	if err := s.Replace(candidate); err != nil {
		t.Fatal(err)
	}
	got, err = s.List()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, candidate) {
		t.Fatalf("Replace changed order or opaque endpoint JSON: got=%+v want=%+v", got, candidate)
	}
}
