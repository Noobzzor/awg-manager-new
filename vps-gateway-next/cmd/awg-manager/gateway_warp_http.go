package main

import (
	gatewaywarp "github.com/hoaxisr/awg-manager/internal/gateway/warp"
	"github.com/hoaxisr/awg-manager/internal/response"
	"net/http"
)

type gatewayWARPBackend interface {
	Import(gatewaywarp.Profile) error
	Describe() (gatewaywarp.Description, error)
}

// SetGatewayWARP is startup-only, before the handler begins serving requests.
func (h *gatewayClientHTTP) SetGatewayWARP(b gatewayWARPBackend) { h.warp = b }

// serveWARP is mounted under the existing gateway admin authentication wrapper.
// Credentials and backend diagnostics are never echoed or logged here.
func (h *gatewayClientHTTP) serveWARP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/gateway/warp":
		if r.Method != http.MethodGet {
			response.MethodNotAllowed(w)
			return
		}
		if h.warp == nil {
			response.ErrorWithStatus(w, 503, "WARP configuration unavailable", "WARP_UNAVAILABLE")
			return
		}
		d, err := h.warp.Describe()
		if err != nil {
			response.ErrorWithStatus(w, 500, "Cannot read WARP configuration", "WARP_READ_FAILED")
			return
		}
		response.Success(w, d)
	case "/api/gateway/warp/import":
		if r.Method != http.MethodPost {
			response.MethodNotAllowed(w)
			return
		}
		if h.warp == nil {
			response.ErrorWithStatus(w, 503, "WARP configuration unavailable", "WARP_UNAVAILABLE")
			return
		}
		p, err := gatewaywarp.DecodeProfile(r.Body)
		if err != nil {
			response.ErrorWithStatus(w, 400, "Invalid WARP profile", "INVALID_WARP_PROFILE")
			return
		}
		if err = h.warp.Import(p); err != nil {
			response.ErrorWithStatus(w, 500, "WARP import failed; verify configuration before retry", "WARP_IMPORT_FAILED")
			return
		}
		response.Success(w, p.Description())
	default:
		response.ErrorWithStatus(w, 404, "WARP route not found", "NOT_FOUND")
	}
}
