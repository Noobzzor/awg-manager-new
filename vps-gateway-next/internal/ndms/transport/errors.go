package transport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// Sentinel errors для Batcher (DataLoader pattern для RCI reads).
// См. internal/ndms/transport/batcher.go.
var (
	// ErrBatcherClosed возвращается из Submit, если Batcher был закрыт
	// до того как submit смог enqueue request.
	ErrBatcherClosed = errors.New("rci batcher: closed")

	// ErrBatchResponseShape возвращается когда NDMS вернул response не в
	// форме JSON-массива (например объект или строка). Сигнал что
	// формат batch'а несовместим — нужно investigate.
	ErrBatchResponseShape = errors.New("rci batcher: unexpected response shape (expected JSON array)")

	// ErrBatchLengthMismatch возвращается когда len(response array) !=
	// len(batch). Может означать NDMS reorder'ил или потерял элементы —
	// сигнал что нужно перейти на id-tagged batch'и.
	ErrBatchLengthMismatch = errors.New("rci batcher: response array length mismatch")
)

// HTTPError is returned by Client.Get / GetRaw / Post when NDMS replies
// with a non-2xx status. Typed so callers can match on Status — e.g.
// PeerStore treats a 404 on /show/interface/<name> as "interface gone,
// no peers" rather than a real error.
type HTTPError struct {
	Method string
	Path   string
	Status int
	Body   []byte
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("rci %s %s: status %d", e.Method, e.Path, e.Status)
}

// NDMSAppError represents an application-level NDMS failure: HTTP 200 but
// the body contains the NDMS error envelope. The wire was fine — the
// command was rejected by NDMS.
type NDMSAppError struct {
	Method  string
	Path    string
	Message string
	Body    []byte
}

func (e *NDMSAppError) Error() string {
	return fmt.Sprintf("rci %s %s: ndms-error: %s", e.Method, e.Path, e.Message)
}

// BatchElementError describes one failed element of a PostBatch.
type BatchElementError struct {
	Index   int
	Message string
}

// BatchError aggregates per-element NDMS application errors from PostBatch.
type BatchError struct {
	Failures []BatchElementError
	Total    int
	Body     json.RawMessage
}

func (e *BatchError) Error() string {
	if len(e.Failures) == 1 {
		return fmt.Sprintf("rci batch: element %d failed: %s",
			e.Failures[0].Index, e.Failures[0].Message)
	}
	return fmt.Sprintf("rci batch: %d/%d elements failed",
		len(e.Failures), e.Total)
}

// ExtractError parses the NDMS error envelope from a response body.
// Returns the message if the body indicates an application error,
// "" otherwise. NDMS shape: {"status":"error","message":"…"}.
func ExtractError(body []byte) string {
	// Fast path: тело без поля "status" конвертом ошибки быть не может —
	// байтовый скан вместо полного Unmarshal больших success-ответов
	// (interface/object-group списки в сотни KB давали ~8% idle-CPU
	// в профиле, стенд 2026-07-16). Семантика не меняется.
	if !bytes.Contains(body, []byte(`"status"`)) {
		return ""
	}
	var envelope struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return ""
	}
	if envelope.Status == "error" {
		return envelope.Message
	}
	return ""
}
