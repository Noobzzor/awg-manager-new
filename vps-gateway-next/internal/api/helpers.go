package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/hoaxisr/awg-manager/internal/response"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// errEmptyBody / errInvalidJSON are the two "the payload is bad JSON" cases
// readJSONBody reports. A body-read failure (oversized, connection reset) is
// returned verbatim so callers can distinguish it (INVALID_BODY) from a
// malformed/absent payload (INVALID_JSON).
var (
	errEmptyBody   = errors.New("empty request body")
	errInvalidJSON = errors.New("invalid JSON")
)

// readJSONBody is the single JSON request-body reader shared by parseJSON and
// decodeBody. It size-caps r.Body, strips a UTF-8 BOM and surrounding
// whitespace, rejects an empty payload and unmarshals into dst.
//
// An empty (or whitespace-only) body is an error, NOT a silent zero-value
// success — that previously let Add/Update requests with no payload through
// as if they carried a valid object.
func readJSONBody(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	raw = bytes.TrimSpace(raw)
	raw = bytes.TrimPrefix(raw, utf8BOM)
	if len(raw) == 0 {
		return errEmptyBody
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("%w: %v", errInvalidJSON, err)
	}
	return nil
}

// parseJSON guards method + reads the request body into T. On any
// error — wrong method, body-read failure, decode failure — it writes
// the canonical error response and returns (zero, false). Callers bail
// out immediately on false.
//
// Body size is capped to maxBodySize via http.MaxBytesReader so an
// oversized payload gets a clean 413 from the decoder rather than
// draining the whole body into memory.
func parseJSON[T any](w http.ResponseWriter, r *http.Request, method string) (T, bool) {
	var dst T
	if r.Method != method {
		response.MethodNotAllowed(w)
		return dst, false
	}
	if err := readJSONBody(w, r, &dst); err != nil {
		if errors.Is(err, errEmptyBody) || errors.Is(err, errInvalidJSON) {
			response.ErrorWithStatus(w, http.StatusBadRequest, "invalid JSON", "INVALID_JSON")
		} else {
			response.ErrorWithStatus(w, http.StatusBadRequest, "invalid body", "INVALID_BODY")
		}
		return dst, false
	}
	return dst, true
}

// requireQueryID reads the "id" query parameter. When it is absent it writes
// the canonical 400 MISSING_ID response and returns ("", false); callers bail
// out on false. Centralising this makes a missing id a consistent 400 across
// every handler that requires one — several previously forgot the guard and
// passed an empty id straight to the service, which surfaced as a 500 (or a
// 200 with the wrong scope) instead.
func requireQueryID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.URL.Query().Get("id")
	if id == "" {
		response.Error(w, "missing id parameter", "MISSING_ID")
		return "", false
	}
	return id, true
}

// decodeBody reads and JSON-decodes the request body into dst for handlers
// that write their own error responses (they typically map the returned
// error to response.BadRequest). It shares readJSONBody with parseJSON, so
// an empty body is rejected here too rather than decoding to a zero value.
func decodeBody(r *http.Request, dst any) error {
	return readJSONBody(nil, r, dst)
}
