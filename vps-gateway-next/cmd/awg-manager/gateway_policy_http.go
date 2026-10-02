package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"sync"

	gatewaygroups "github.com/hoaxisr/awg-manager/internal/gateway/groups"
	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
	gatewaypolicy "github.com/hoaxisr/awg-manager/internal/gateway/policy"
	"github.com/hoaxisr/awg-manager/internal/response"
)

const gatewayPolicyProfilesAPIPrefix = "/api/gateway/policies/profiles"

// gatewayPolicyHTTP owns the admin API surface for durable policy profiles.
// FileStore remains the source of truth; its atomic Save preserves the previous
// profile set when validation or persistence fails.
type gatewayPolicyHTTP struct {
	mu      sync.Mutex
	store   gatewaypolicy.Store
	runtime gatewaypolicy.Runtime
	clients *gatewayingress.Registry
	groups  *gatewaygroups.Manager
}

func newGatewayPolicyHTTP(store gatewaypolicy.Store, runtime gatewaypolicy.Runtime, clients *gatewayingress.Registry, groups *gatewaygroups.Manager) *gatewayPolicyHTTP {
	if store == nil || clients == nil {
		return nil
	}
	return &gatewayPolicyHTTP{store: store, runtime: runtime, clients: clients, groups: groups}
}

func (h *gatewayPolicyHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == gatewayPolicyProfilesAPIPrefix {
		switch r.Method {
		case http.MethodGet:
			h.list(w)
		default:
			response.MethodNotAllowed(w)
		}
		return
	}
	if r.URL.Path == gatewayPolicyProfilesAPIPrefix+"/create" {
		h.create(w, r)
		return
	}
	if r.URL.Path == gatewayPolicyProfilesAPIPrefix+"/preview" {
		h.preview(w, r)
		return
	}
	escapedPath := r.URL.EscapedPath()
	if !strings.HasPrefix(escapedPath, gatewayPolicyProfilesAPIPrefix+"/") {
		response.ErrorWithStatus(w, http.StatusNotFound, "gateway policy profile route not found", "NOT_FOUND")
		return
	}
	parts := strings.Split(strings.TrimPrefix(escapedPath, gatewayPolicyProfilesAPIPrefix+"/"), "/")
	if len(parts) == 1 && parts[0] != "" {
		id, err := url.PathUnescape(parts[0])
		if err != nil || id == "" {
			response.ErrorWithStatus(w, http.StatusNotFound, "gateway policy profile route not found", "NOT_FOUND")
			return
		}
		h.item(w, r, id)
		return
	}
	if len(parts) == 2 && parts[0] != "" && parts[1] == "apply" {
		id, err := url.PathUnescape(parts[0])
		if err != nil || id == "" {
			response.ErrorWithStatus(w, http.StatusNotFound, "gateway policy profile route not found", "NOT_FOUND")
			return
		}
		h.apply(w, r, id)
		return
	}
	response.ErrorWithStatus(w, http.StatusNotFound, "gateway policy profile route not found", "NOT_FOUND")
}

func (h *gatewayPolicyHTTP) list(w http.ResponseWriter) {
	profiles, err := h.load()
	if err != nil {
		h.writeStoreError(w)
		return
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	w.Header().Set("Cache-Control", "no-store, private, max-age=0")
	response.Success(w, profiles)
}

func (h *gatewayPolicyHTTP) create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var profile gatewaypolicy.Profile
	if err := decodeGatewayPolicyProfile(w, r, &profile); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, "invalid gateway policy profile request", "INVALID_REQUEST")
		return
	}
	if err := profile.Validate(); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, err.Error(), "INVALID_GATEWAY_POLICY")
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	profiles, err := h.store.Load()
	if err != nil {
		h.writeStoreError(w)
		return
	}
	for _, current := range profiles {
		if current.ID == profile.ID {
			response.ErrorWithStatus(w, http.StatusConflict, "gateway policy profile already exists", "GATEWAY_POLICY_PROFILE_EXISTS")
			return
		}
	}
	profiles = append(profiles, profile)
	if err := h.store.Save(profiles); err != nil {
		h.writeStoreError(w)
		return
	}
	response.Success(w, profile)
}

func (h *gatewayPolicyHTTP) item(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		profile, exists, err := h.get(id)
		if err != nil {
			h.writeStoreError(w)
			return
		}
		if !exists {
			response.ErrorWithStatus(w, http.StatusNotFound, "gateway policy profile not found", "GATEWAY_POLICY_PROFILE_NOT_FOUND")
			return
		}
		response.Success(w, profile)
	case http.MethodPut:
		var profile gatewaypolicy.Profile
		if err := decodeGatewayPolicyProfile(w, r, &profile); err != nil {
			response.ErrorWithStatus(w, http.StatusBadRequest, "invalid gateway policy profile request", "INVALID_REQUEST")
			return
		}
		if profile.ID != id {
			response.ErrorWithStatus(w, http.StatusBadRequest, "profile id must match the route id", "INVALID_GATEWAY_POLICY")
			return
		}
		if err := profile.Validate(); err != nil {
			response.ErrorWithStatus(w, http.StatusBadRequest, err.Error(), "INVALID_GATEWAY_POLICY")
			return
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		profiles, err := h.store.Load()
		if err != nil {
			h.writeStoreError(w)
			return
		}
		found := false
		for i := range profiles {
			if profiles[i].ID == id {
				profiles[i] = profile
				found = true
				break
			}
		}
		if !found {
			response.ErrorWithStatus(w, http.StatusNotFound, "gateway policy profile not found", "GATEWAY_POLICY_PROFILE_NOT_FOUND")
			return
		}
		if err := h.store.Save(profiles); err != nil {
			h.writeStoreError(w)
			return
		}
		response.Success(w, profile)
	case http.MethodDelete:
		h.mu.Lock()
		defer h.mu.Unlock()
		profiles, err := h.store.Load()
		if err != nil {
			h.writeStoreError(w)
			return
		}
		filtered := make([]gatewaypolicy.Profile, 0, len(profiles))
		found := false
		for _, profile := range profiles {
			if profile.ID == id {
				found = true
				continue
			}
			filtered = append(filtered, profile)
		}
		if !found {
			response.ErrorWithStatus(w, http.StatusNotFound, "gateway policy profile not found", "GATEWAY_POLICY_PROFILE_NOT_FOUND")
			return
		}
		if err := h.store.Save(filtered); err != nil {
			h.writeStoreError(w)
			return
		}
		response.Success(w, map[string]bool{"deleted": true})
	default:
		response.MethodNotAllowed(w)
	}
}

func (h *gatewayPolicyHTTP) apply(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	profile, exists, err := h.get(id)
	if err != nil {
		h.writeStoreError(w)
		return
	}
	if !exists {
		response.ErrorWithStatus(w, http.StatusNotFound, "gateway policy profile not found", "GATEWAY_POLICY_PROFILE_NOT_FOUND")
		return
	}
	clients, groups := h.sourceAddresses()
	if err := h.runtime.ApplyWithSources(profile, "", clients, groups); err != nil {
		if strings.Contains(err.Error(), "unknown client") || strings.Contains(err.Error(), "unknown group") {
			response.ErrorWithStatus(w, http.StatusConflict, err.Error(), "GATEWAY_POLICY_SOURCE_UNRESOLVED")
			return
		}
		response.ErrorWithStatus(w, http.StatusConflict, err.Error(), "GATEWAY_POLICY_APPLY_INVALID")
		return
	}
	response.Success(w, map[string]any{"applied": true, "profileId": profile.ID})
}

func (h *gatewayPolicyHTTP) preview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var profile gatewaypolicy.Profile
	if err := decodeGatewayPolicyProfile(w, r, &profile); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, "invalid gateway policy profile request", "INVALID_REQUEST")
		return
	}
	if err := profile.Validate(); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, err.Error(), "INVALID_GATEWAY_POLICY")
		return
	}
	clients, groups := h.sourceAddresses()
	compiled, err := h.runtime.PreviewWithSources(profile, "", clients, groups)
	if err != nil {
		if strings.Contains(err.Error(), "unknown client") || strings.Contains(err.Error(), "unknown group") {
			response.ErrorWithStatus(w, http.StatusConflict, err.Error(), "GATEWAY_POLICY_SOURCE_UNRESOLVED")
			return
		}
		response.ErrorWithStatus(w, http.StatusUnprocessableEntity, err.Error(), "GATEWAY_POLICY_PREVIEW_INVALID")
		return
	}
	response.Success(w, compiled)
}

func (h *gatewayPolicyHTTP) sourceAddresses() (map[string][]string, map[string][]string) {
	clients := make(map[string][]string)
	for _, client := range h.clients.List() {
		if client.Address.IsValid() {
			clients[client.ID] = []string{netip.PrefixFrom(client.Address, client.Address.BitLen()).String()}
		}
	}
	groups := make(map[string][]string)
	if h.groups == nil {
		return clients, groups
	}
	for _, group := range h.groups.List() {
		var addresses []string
		for _, clientID := range group.ClientIDs {
			client, exists := clients[clientID]
			if !exists {
				addresses = nil
				break
			}
			addresses = append(addresses, client...)
		}
		if len(addresses) > 0 {
			groups[group.ID] = addresses
		}
	}
	return clients, groups
}

func (h *gatewayPolicyHTTP) get(id string) (gatewaypolicy.Profile, bool, error) {
	profiles, err := h.load()
	if err != nil {
		return gatewaypolicy.Profile{}, false, err
	}
	for _, profile := range profiles {
		if profile.ID == id {
			return profile, true, nil
		}
	}
	return gatewaypolicy.Profile{}, false, nil
}

func (h *gatewayPolicyHTTP) load() ([]gatewaypolicy.Profile, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.store.Load()
}

func (h *gatewayPolicyHTTP) writeStoreError(w http.ResponseWriter) {
	response.ErrorWithStatus(w, http.StatusInternalServerError, "gateway policy profiles could not be persisted", "GATEWAY_POLICY_PERSIST_ERROR")
}

func decodeGatewayPolicyProfile(w http.ResponseWriter, r *http.Request, profile *gatewaypolicy.Profile) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(profile); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}
