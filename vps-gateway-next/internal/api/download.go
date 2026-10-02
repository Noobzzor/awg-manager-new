package api

import (
	"net/http"

	"github.com/hoaxisr/awg-manager/internal/downloader"
	"github.com/hoaxisr/awg-manager/internal/response"
)

type DownloadRouteDTO struct {
	Tag  string `json:"tag" example:"direct"`
	Kind string `json:"kind,omitempty" example:"direct"`
}

type DownloadOutboundDTO struct {
	Tag       string `json:"tag" example:"direct"`
	Kind      string `json:"kind" example:"direct"`
	Label     string `json:"label" example:"Direct (WAN)"`
	Detail    string `json:"detail,omitempty" example:"без туннеля"`
	Available bool   `json:"available" example:"true"`
}

type DownloadOutboundsResponse struct {
	Success bool                  `json:"success" example:"true"`
	Data    []DownloadOutboundDTO `json:"data"`
}

type DownloadHandler struct {
	svc *downloader.Service
}

func NewDownloadHandler(svc *downloader.Service) *DownloadHandler {
	if svc == nil {
		svc = downloader.NewService(downloader.Deps{})
	}
	return &DownloadHandler{svc: svc}
}

// ListOutbounds returns existing route options for service downloads.
//
//	@Summary		List service download outbounds
//	@Tags			download
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	DownloadOutboundsResponse
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/download/outbounds [get]
func (h *DownloadHandler) ListOutbounds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}

	out := h.svc.ListOutbounds(r.Context())
	resp := make([]DownloadOutboundDTO, 0, len(out))
	for _, ob := range out {
		resp = append(resp, DownloadOutboundDTO{
			Tag:       ob.Tag,
			Kind:      ob.Kind,
			Label:     ob.Label,
			Detail:    ob.Detail,
			Available: ob.Available,
		})
	}
	response.Success(w, resp)
}
