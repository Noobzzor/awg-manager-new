package api

import (
	"context"
	"net/http"

	"github.com/hoaxisr/awg-manager/internal/accesspolicy"
	"github.com/hoaxisr/awg-manager/internal/diagnostics"
	"github.com/hoaxisr/awg-manager/internal/response"
)

// dnsProxyStatusReader is the narrow read surface over the NDMS query store.
type dnsProxyStatusReader interface {
	List(ctx context.Context) ([]byte, error)
}

// accessPolicyLister is the narrow surface used to resolve PolicyN -> name.
type accessPolicyLister interface {
	List(ctx context.Context) ([]accesspolicy.Policy, error)
}

// DnsProxyInfoData is the response payload.
type DnsProxyInfoData struct {
	Proxies []diagnostics.DNSProxy `json:"proxies"`
}

type DnsProxyInfoHandler struct {
	store    dnsProxyStatusReader
	policies accessPolicyLister
}

func NewDnsProxyInfoHandler(store dnsProxyStatusReader, policies accessPolicyLister) *DnsProxyInfoHandler {
	return &DnsProxyInfoHandler{store: store, policies: policies}
}

// Get returns the parsed /show/dns-proxy state.
//
//	@Summary		DNS proxy info
//	@Description	Running ndnproxy state: upstreams, per-policy stats, static records, rebind.
//	@Tags			diagnostics
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	DnsProxyInfoEnvelope
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/diagnostics/dns-proxy [get]
func (h *DnsProxyInfoHandler) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	raw, err := h.store.List(r.Context())
	if err != nil {
		response.InternalError(w, "не удалось прочитать dns-proxy роутера")
		return
	}
	proxies, err := diagnostics.ParseDNSProxy(raw)
	if err != nil {
		response.InternalError(w, "не удалось разобрать ответ dns-proxy")
		return
	}

	nameByPolicy := map[string]string{}
	if pols, perr := h.policies.List(r.Context()); perr == nil {
		for _, p := range pols {
			nameByPolicy[p.Name] = p.Description
		}
	}
	for i := range proxies {
		proxies[i].DisplayName = dnsProxyDisplayName(proxies[i].Name, nameByPolicy)
	}

	response.Success(w, DnsProxyInfoData{Proxies: proxies})
}

func dnsProxyDisplayName(name string, byPolicy map[string]string) string {
	if name == "System" {
		return "Системный"
	}
	if d := byPolicy[name]; d != "" {
		return d
	}
	return name
}

// DnsProxyInfoEnvelope documents the success envelope for swagger.
type DnsProxyInfoEnvelope struct {
	Success bool             `json:"success" example:"true"`
	Data    DnsProxyInfoData `json:"data"`
}
