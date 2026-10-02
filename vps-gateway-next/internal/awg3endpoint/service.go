package awg3endpoint

import (
	"encoding/json"
	"fmt"

	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

type TagInfo struct {
	Tag  string `json:"tag"`
	Kind string `json:"kind"` // "awg3"
}

// Валидирующая запись: SaveAndValidate прогоняет sing-box check и на невалидном
// конфиге НЕ применяет его (см. orchestrator/draft.go). Sync возвращает ошибку,
// чтобы handler откатил только что добавленную запись.
type Orchestrator interface {
	SaveAndValidate(slot orchestrator.Slot, jsonBytes []byte) (orchestrator.ValidationResult, error)
	SnapshotSlot(slot orchestrator.Slot) ([]byte, bool, error)
	RestoreSlot(slot orchestrator.Slot, data []byte, enabled bool) error
}

type Service struct {
	store  *Store // тот же пакет awg3endpoint (Task 2)
	orch   Orchestrator
	appLog *logging.ScopedLogger
}

func NewService(store *Store, orch Orchestrator, appLogger logging.AppLogger) *Service {
	return &Service{
		store:  store,
		orch:   orch,
		appLog: logging.NewScopedLogger(appLogger, logging.GroupSingbox, logging.SubAwg3),
	}
}

// Sync материализует store → 16-awg3.json как {"endpoints":[...]}, перезаписывая
// поле tag каждого endpoint на человекочитаемый Record.Tag.
func (s *Service) Sync() error {
	list, err := s.store.List()
	if err != nil {
		return err
	}
	return s.Apply(list)
}

// Apply validates and applies an in-memory candidate without reading or
// publishing the durable store. Mutation handlers use it before Store.Replace.
func (s *Service) Apply(list []Record) error {
	eps := make([]json.RawMessage, 0, len(list))
	for _, rec := range list {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(rec.Endpoint, &obj); err != nil {
			return fmt.Errorf("render AWG3 endpoint %q: %w", rec.ID, err)
		}
		tagJSON, _ := json.Marshal(rec.Tag)
		obj["tag"] = tagJSON
		merged, err := json.Marshal(obj)
		if err != nil {
			s.appLog.Warn("sync-skip", rec.Tag, fmt.Sprintf("id=%s: не удалось сериализовать endpoint: %v", rec.ID, err))
			continue
		}
		eps = append(eps, merged)
	}
	data, err := json.MarshalIndent(map[string]any{"endpoints": eps}, "", "  ")
	if err != nil {
		return err
	}
	res, err := s.orch.SaveAndValidate(orchestrator.SlotAwg3, data)
	if err != nil {
		return err
	}
	// ВАЖНО (ревью I-1): при провале валидации SaveAndValidate возвращает (res, nil) —
	// err==nil. ValidationResult имеет Ok() bool и Error() string (НЕ Valid/Message).
	if !res.Ok() {
		return fmt.Errorf("sing-box check: %s", res.Error())
	}
	return nil
}

func (s *Service) SnapshotSlot() ([]byte, bool, error) {
	data, enabled, err := s.orch.SnapshotSlot(orchestrator.SlotAwg3)
	return append([]byte(nil), data...), enabled, err
}

func (s *Service) RestoreSlot(data []byte, enabled bool) error {
	return s.orch.RestoreSlot(orchestrator.SlotAwg3, data, enabled)
}

// HoldReloads keeps candidate slot writes from reaching the running process
// before the enclosing AWG3/DNS durable transaction commits.
func (s *Service) HoldReloads() func() {
	if holder, ok := s.orch.(interface{ HoldReloads() func() }); ok {
		return holder.HoldReloads()
	}
	return func() {}
}

// ListTags возвращает теги всех записей. Сигнатура без error зафиксирована
// адаптерами ([]TagInfo без error): при ошибке чтения store отдаём пустой
// список, но оставляем след в журнале — молчаливое проглатывание скрыло бы
// битый store.
func (s *Service) ListTags() []TagInfo {
	list, err := s.store.List()
	if err != nil {
		s.appLog.Warn("list-tags", "", fmt.Sprintf("не удалось прочитать store: %v", err))
	}
	out := make([]TagInfo, 0, len(list))
	for _, rec := range list {
		out = append(out, TagInfo{Tag: rec.Tag, Kind: "awg3"})
	}
	return out
}
