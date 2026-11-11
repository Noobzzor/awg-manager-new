package api

import (
	"net/http"

	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/singbox/heavyop"
)

// acquireConfigOperation joins the outer local writer transaction before any
// config/store snapshot or mutation. Read request bodies before acquiring it:
// a slow client must not keep unrelated configuration writers locked out.
// nil retains the uncoordinated legacy handler behavior. A configured gate is
// independent of heavyop.Default and must be shared by all local producers.
func acquireConfigOperation(w http.ResponseWriter, r *http.Request, gate *heavyop.Gate) (func(), bool) {
	if gate == nil {
		return func() {}, true
	}
	if err := gate.LockContext(r.Context()); err != nil {
		response.ErrorWithStatus(w, http.StatusRequestTimeout, "Request cancelled while waiting for configuration update", "REQUEST_CANCELLED")
		return nil, false
	}
	return gate.Unlock, true
}
