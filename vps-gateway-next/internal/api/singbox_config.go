package api

import (
	"errors"
	"net/http"

	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/singbox/configmerge"
)

// SingboxConfigPreviewResponse is the typed payload of GET
// /api/singbox/config-preview. The JSON field is a pretty-printed
// merge of every active config.d/*.json — exactly what sing-box loads
// when started with `-C config.d/`.
type SingboxConfigPreviewResponse struct {
	JSON string `json:"json"`
}

// SingboxConfigHandler exposes a read-only preview of the merged
// sing-box configuration. The orchestrator's ConfigDir is injected via
// configDirFn so tests can swap a tmp dir without dragging in the
// orchestrator.
type SingboxConfigHandler struct {
	configDirFn func() string
}

// NewSingboxConfigHandler constructs the handler.
func NewSingboxConfigHandler(configDirFn func() string) *SingboxConfigHandler {
	return &SingboxConfigHandler{configDirFn: configDirFn}
}

// Preview returns the read-only merged sing-box configuration.
//
//	@Summary		Get merged sing-box configuration preview
//	@Description	Returns the read-only pretty-printed JSON that sing-box loads when started with `-C config.d/` — every active slot file concatenated in lexicographic order, with arrays merged and tag collisions surfaced as errors.
//	@Tags			singbox
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	OkResponse{data=SingboxConfigPreviewResponse}
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/config-preview [get]
func (h *SingboxConfigHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	merged, err := configmerge.MergeDir(h.configDirFn())
	if err != nil {
		var ce *configmerge.CollisionError
		if errors.As(err, &ce) {
			response.InternalError(w, ce.Error())
			return
		}
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, SingboxConfigPreviewResponse{JSON: merged})
}
