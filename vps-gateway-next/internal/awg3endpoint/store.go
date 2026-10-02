package awg3endpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/hoaxisr/awg-manager/internal/storage"
)

// Store хранит AWG3-endpoint записи в одном JSON-файле ([]Record).
type Store struct {
	mu            sync.Mutex
	transactionMu sync.RWMutex
	writeAtomic   func(string, []byte, os.FileMode) error
	path          string
	loaded        bool
	records       []Record
}

// TransactionLocker coordinates the complete logical AWG3 transaction
// (store -> AWG slot -> dependent DNS routes) with external readers. Store
// methods intentionally do not acquire it: mutation handlers hold the write
// side while calling Add/Delete/Rename/List during commit and rollback.
type TransactionLocker interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

// NewStore создаёт store поверх файла path.
func NewStore(path string) *Store {
	return &Store{path: path, writeAtomic: storage.AtomicWritePerm}
}

// TransactionLock returns the process-local coordinator owned by this store.
// Every externally visible AWG3 reader must share it with mutation handlers.
func (s *Store) TransactionLock() TransactionLocker { return &s.transactionMu }

// load лениво читает файл. Пустой/отсутствующий → пустой список.
// Вызывать под удержанием s.mu.
func (s *Store) load() error {
	if s.loaded {
		return nil
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.records = nil
			s.loaded = true
			return nil
		}
		return fmt.Errorf("read awg3 store: %w", err)
	}
	if len(data) == 0 {
		s.records = nil
		s.loaded = true
		return nil
	}
	var recs []Record
	if err := json.Unmarshal(data, &recs); err != nil {
		return fmt.Errorf("parse awg3 store: %w", err)
	}
	s.records = recs
	s.loaded = true
	return nil
}

// save пишет candidate атомарно. Вызывать под удержанием s.mu. Публикация в
// s.records выполняется вызывающим только после успешной durable-записи.
func (s *Store) save(candidate []Record) error {
	data, err := json.MarshalIndent(candidate, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal awg3 store: %w", err)
	}
	// 0600: awg3.json содержит приватные ключи endpoint'ов — не отдаём их
	// в мир (остальные store'ы пишутся 0644, их права не трогаем).
	if err := s.writeAtomic(s.path, data, 0600); err != nil {
		return fmt.Errorf("write awg3 store: %w", err)
	}
	return nil
}

// List возвращает копию всех записей в порядке добавления.
func (s *Store) List() ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return nil, err
	}
	return cloneRecords(s.records), nil
}

// Replace durably replaces the complete ordered snapshot. The in-memory cache
// is published only after the atomic write succeeds, so a failed commit leaves
// both disk and readers on the previous snapshot.
func (s *Store) Replace(candidate []Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	next := cloneRecords(candidate)
	if err := s.save(next); err != nil {
		return err
	}
	s.records = next
	return nil
}

func cloneRecords(records []Record) []Record {
	if records == nil {
		return nil
	}
	out := make([]Record, len(records))
	for i := range records {
		out[i] = records[i]
		out[i].Endpoint = append(json.RawMessage(nil), records[i].Endpoint...)
	}
	return out
}

// Add добавляет запись в конец и сохраняет.
func (s *Store) Add(rec Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	candidate := make([]Record, len(s.records), len(s.records)+1)
	copy(candidate, s.records)
	candidate = append(candidate, rec)
	if err := s.save(candidate); err != nil {
		return err
	}
	s.records = candidate
	return nil
}

// Delete удаляет запись по id и сохраняет.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	for i, r := range s.records {
		if r.ID == id {
			candidate := make([]Record, 0, len(s.records)-1)
			candidate = append(candidate, s.records[:i]...)
			candidate = append(candidate, s.records[i+1:]...)
			if err := s.save(candidate); err != nil {
				return err
			}
			s.records = candidate
			return nil
		}
	}
	return fmt.Errorf("awg3 endpoint not found: %s", id)
}

// Rename меняет тег записи id на newTag. Ошибка, если newTag занят ДРУГОЙ записью.
func (s *Store) Rename(id, newTag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	idx := -1
	for i, r := range s.records {
		if r.ID == id {
			idx = i
			continue
		}
		if r.Tag == newTag {
			return fmt.Errorf("%w: %q", ErrTag, newTag)
		}
	}
	if idx == -1 {
		return fmt.Errorf("awg3 endpoint not found: %s", id)
	}
	candidate := make([]Record, len(s.records))
	copy(candidate, s.records)
	candidate[idx].Tag = newTag
	if err := s.save(candidate); err != nil {
		return err
	}
	s.records = candidate
	return nil
}

// Get возвращает запись по id.
func (s *Store) Get(id string) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return Record{}, false
	}
	for _, r := range s.records {
		if r.ID == id {
			return r, true
		}
	}
	return Record{}, false
}

// Tags возвращает множество занятых тегов. Сигнатура без error намеренная
// (Parse и HasContent-замыкание зовут её как чистую функцию): битый файл →
// пустой результат. Fail-closed обеспечивают Add/List — они читают тот же
// файл и вернут ошибку раньше, чем пустой Tags() приведёт к дублю.
func (s *Store) Tags() map[string]bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return map[string]bool{}
	}
	tags := make(map[string]bool, len(s.records))
	for _, r := range s.records {
		tags[r.Tag] = true
	}
	return tags
}

// Len возвращает число записей (для HasContent). Сигнатура без error
// намеренная (используется в HasContent-замыкании): битый файл → 0.
// Fail-closed обеспечивают Add/List, читающие тот же файл (см. Tags).
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return 0
	}
	return len(s.records)
}
