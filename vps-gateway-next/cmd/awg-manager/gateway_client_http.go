package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	gatewaygroups "github.com/hoaxisr/awg-manager/internal/gateway/groups"
	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
	gatewaypolicy "github.com/hoaxisr/awg-manager/internal/gateway/policy"
	gatewaysubscription "github.com/hoaxisr/awg-manager/internal/gateway/subscription"
	"github.com/hoaxisr/awg-manager/internal/response"
)

var (
	errGatewayClientConfigUnavailable = errors.New("gateway client profile unavailable")
	errGatewayPeerStatusUnavailable   = errors.New("gateway peer status unavailable")
	errGatewayEndpointUnconfigured    = errors.New("gateway endpoint is not configured")
)

type gatewayClientHTTP struct {
	registry        *gatewayingress.Registry
	path            string
	applier         gatewayingress.PeerApplier
	statusReader    gatewayingress.PeerStatusReader
	serverPublicKey string
	endpoint        string
	subscriptions   *gatewaySubscriptionHTTP
	groups          *gatewayGroupHTTP
	policies        *gatewayPolicyHTTP
}

type gatewayClientRequest struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func newGatewayClientHTTP(registry *gatewayingress.Registry, path string, appliers ...gatewayingress.PeerApplier) *gatewayClientHTTP {
	if registry == nil || strings.TrimSpace(path) == "" {
		return nil
	}
	h := &gatewayClientHTTP{registry: registry, path: path}
	if len(appliers) > 0 {
		h.applier = appliers[0]
		if reader, ok := appliers[0].(gatewayingress.PeerStatusReader); ok {
			h.statusReader = reader
		}
	}
	return h
}

func (h *gatewayClientHTTP) SetClientConfig(serverPublicKey, endpoint string) {
	h.serverPublicKey = strings.TrimSpace(serverPublicKey)
	h.endpoint = strings.TrimSpace(endpoint)
}

func (h *gatewayClientHTTP) SetClientSubscriptions(manager *gatewaysubscription.Manager) {
	h.subscriptions = newGatewaySubscriptionHTTP(manager, h)
}

func (h *gatewayClientHTTP) SetGatewayGroups(manager *gatewaygroups.Manager) {
	h.groups = newGatewayGroupHTTP(manager)
}

func (h *gatewayClientHTTP) SetGatewayPolicy(store gatewaypolicy.Store, runtime gatewaypolicy.Runtime) {
	h.policies = newGatewayPolicyHTTP(store, runtime, h.registry, h.groupsManager())
}

func (h *gatewayClientHTTP) groupsManager() *gatewaygroups.Manager {
	if h.groups == nil {
		return nil
	}
	return h.groups.manager
}

func (h *gatewayClientHTTP) SetSubscriptionBaseURL(raw string) error {
	if h.subscriptions == nil {
		return errors.New("gateway subscriptions are not configured")
	}
	base, err := validateGatewaySubscriptionBaseURL(raw)
	if err != nil {
		return err
	}
	h.subscriptions.publicBaseURL = base
	return nil
}

func (h *gatewayClientHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == gatewayPolicyProfilesAPIPrefix || strings.HasPrefix(r.URL.Path, gatewayPolicyProfilesAPIPrefix+"/") {
		if h.policies == nil {
			h.notFound(w)
			return
		}
		h.policies.ServeHTTP(w, r)
		return
	}
	if r.URL.Path == gatewayGroupsAPIPrefix || strings.HasPrefix(r.URL.Path, gatewayGroupsAPIPrefix+"/") {
		if h.groups == nil {
			http.NotFound(w, r)
			return
		}
		h.groups.ServeHTTP(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, gatewaySubscriptionPathPrefix) {
		if h.subscriptions == nil {
			http.NotFound(w, r)
			return
		}
		h.subscriptions.ServePublic(w, r)
		return
	}
	if r.URL.Path == gatewaySubscriptionAPIPrefix || strings.HasPrefix(r.URL.Path, gatewaySubscriptionAPIPrefix+"/") {
		if h.subscriptions == nil {
			h.notFound(w)
			return
		}
		h.subscriptions.ServeAdmin(w, r)
		return
	}
	if r.URL.Path == "/api/gateway/clients" {
		if r.Method != http.MethodGet {
			response.MethodNotAllowed(w)
			return
		}
		response.Success(w, h.registry.List())
		return
	}
	if r.URL.Path == "/api/gateway/clients/create" {
		h.create(w, r)
		return
	}
	const prefix = "/api/gateway/clients/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		h.notFound(w)
		return
	}
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/"), "/")
	if len(parts) == 1 && r.Method == http.MethodGet {
		h.get(w, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "config" {
		h.downloadConfig(w, r, parts[0])
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPost {
		h.transition(w, r, parts[0], parts[1])
		return
	}
	h.notFound(w)
}

func (h *gatewayClientHTTP) downloadConfig(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	config, err := h.clientConfig(r.Context(), id)
	switch {
	case errors.Is(err, errGatewayEndpointUnconfigured):
		response.ErrorWithStatus(w, http.StatusServiceUnavailable, "gateway public endpoint is not configured", "GATEWAY_ENDPOINT_UNCONFIGURED")
		return
	case errors.Is(err, gatewayingress.ErrClientNotFound):
		response.ErrorWithStatus(w, http.StatusNotFound, "client not found", "GATEWAY_CLIENT_NOT_FOUND")
		return
	case errors.Is(err, errGatewayClientConfigUnavailable):
		response.ErrorWithStatus(w, http.StatusConflict, "client profile is unavailable", "GATEWAY_CLIENT_CONFIG_UNAVAILABLE")
		return
	case errors.Is(err, errGatewayPeerStatusUnavailable):
		response.ErrorWithStatus(w, http.StatusServiceUnavailable, "gateway peer status is unavailable", "GATEWAY_PEER_STATUS_UNAVAILABLE")
		return
	case err != nil:
		response.ErrorWithStatus(w, http.StatusConflict, "client profile is unavailable", "GATEWAY_CLIENT_CONFIG_UNAVAILABLE")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="awg-client.conf"`)
	w.Header().Set("Cache-Control", "no-store, private, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(config))
}

func (h *gatewayClientHTTP) clientConfig(ctx context.Context, id string) (string, error) {
	if h.serverPublicKey == "" || h.endpoint == "" {
		return "", errGatewayEndpointUnconfigured
	}
	client, ok := h.registry.Get(id)
	if !ok {
		return "", gatewayingress.ErrClientNotFound
	}
	if client.Status != gatewayingress.StatusActive {
		return "", errGatewayClientConfigUnavailable
	}
	if h.applier != nil {
		if h.statusReader == nil {
			return "", errGatewayPeerStatusUnavailable
		}
		status, err := h.statusReader.ReadPeerStatus(ctx, client.PublicKey)
		if errors.Is(err, gatewayingress.ErrPeerNotFound) {
			return "", errGatewayClientConfigUnavailable
		}
		if err != nil || status.PublicKey != client.PublicKey {
			return "", errGatewayPeerStatusUnavailable
		}
	}
	config, err := h.registry.RenderClientConfig(id, h.serverPublicKey, h.endpoint)
	if err != nil {
		if errors.Is(err, gatewayingress.ErrClientNotFound) {
			return "", err
		}
		return "", errGatewayClientConfigUnavailable
	}
	return config, nil
}

func (h *gatewayClientHTTP) create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var input gatewayClientRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, "invalid client request", "INVALID_REQUEST")
		return
	}
	client, err := h.registry.Create(input.ID, input.Label)
	if err != nil {
		response.ErrorWithStatus(w, http.StatusConflict, err.Error(), "GATEWAY_CLIENT_CREATE_ERROR")
		return
	}
	if h.applier != nil {
		peer, err := gatewayingress.BuildPeerConfig(client, "")
		if err != nil {
			_ = h.registry.Delete(client.ID)
			response.ErrorWithStatus(w, http.StatusInternalServerError, "client peer config failed", "GATEWAY_PEER_APPLY_ERROR")
			return
		}
		if err := h.applier.ApplyPeer(r.Context(), peer); err != nil {
			_ = h.registry.Delete(client.ID)
			response.ErrorWithStatus(w, http.StatusBadGateway, "client peer could not be applied", "GATEWAY_PEER_APPLY_ERROR")
			return
		}
	}
	if err := gatewayingress.Save(h.registry, h.path); err != nil {
		if h.applier != nil {
			_ = h.applier.RemovePeer(r.Context(), client.PublicKey)
		}
		_ = h.registry.Delete(client.ID)
		response.ErrorWithStatus(w, http.StatusInternalServerError, "client persistence failed", "GATEWAY_CLIENT_PERSIST_ERROR")
		return
	}
	response.Success(w, client.Public())
}

func (h *gatewayClientHTTP) get(w http.ResponseWriter, id string) {
	client, ok := h.registry.Get(id)
	if !ok {
		response.ErrorWithStatus(w, http.StatusNotFound, "client not found", "GATEWAY_CLIENT_NOT_FOUND")
		return
	}
	response.Success(w, client.Public())
}

func (h *gatewayClientHTTP) transition(w http.ResponseWriter, r *http.Request, id, action string) {
	before, ok := h.registry.Get(id)
	if !ok {
		response.ErrorWithStatus(w, http.StatusNotFound, "client not found", "GATEWAY_CLIENT_NOT_FOUND")
		return
	}
	var err error
	switch action {
	case "disable", "revoke":
		if h.applier != nil {
			if err := h.applier.RemovePeer(r.Context(), before.PublicKey); err != nil {
				response.ErrorWithStatus(w, http.StatusBadGateway, "client peer could not be removed", "GATEWAY_PEER_REMOVE_ERROR")
				return
			}
		}
		if action == "disable" {
			err = h.registry.Disable(id)
		} else {
			err = h.registry.Revoke(id)
		}
	case "enable":
		err = h.registry.Enable(id)
		if err == nil && h.applier != nil {
			client, _ := h.registry.Get(id)
			peer, buildErr := gatewayingress.BuildPeerConfig(client, "")
			if buildErr != nil || h.applier.ApplyPeer(r.Context(), peer) != nil {
				_ = h.registry.Disable(id)
				response.ErrorWithStatus(w, http.StatusBadGateway, "client peer could not be enabled", "GATEWAY_PEER_APPLY_ERROR")
				return
			}
		}
	default:
		h.notFound(w)
		return
	}
	if errors.Is(err, gatewayingress.ErrClientNotFound) {
		response.ErrorWithStatus(w, http.StatusNotFound, "client not found", "GATEWAY_CLIENT_NOT_FOUND")
		return
	}
	if err != nil {
		response.ErrorWithStatus(w, http.StatusConflict, err.Error(), "GATEWAY_CLIENT_TRANSITION_ERROR")
		return
	}
	if err := gatewayingress.Save(h.registry, h.path); err != nil {
		response.ErrorWithStatus(w, http.StatusInternalServerError, "client persistence failed", "GATEWAY_CLIENT_PERSIST_ERROR")
		return
	}
	client, _ := h.registry.Get(id)
	response.Success(w, client.Public())
}

func (h *gatewayClientHTTP) notFound(w http.ResponseWriter) {
	response.ErrorWithStatus(w, http.StatusNotFound, "gateway client route not found", "NOT_FOUND")
}
