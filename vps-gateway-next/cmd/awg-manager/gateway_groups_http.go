package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	gatewaygroups "github.com/hoaxisr/awg-manager/internal/gateway/groups"
	"github.com/hoaxisr/awg-manager/internal/response"
)

const gatewayGroupsAPIPrefix = "/api/gateway/groups"

type gatewayGroupHTTP struct {
	manager *gatewaygroups.Manager
}

type gatewayGroupRequest struct {
	Name      string   `json:"name"`
	ClientIDs []string `json:"clientIds"`
}

func newGatewayGroupHTTP(manager *gatewaygroups.Manager) *gatewayGroupHTTP {
	if manager == nil {
		return nil
	}
	return &gatewayGroupHTTP{manager: manager}
}

func (h *gatewayGroupHTTP) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == gatewayGroupsAPIPrefix:
		if r.Method != http.MethodGet {
			response.MethodNotAllowed(w)
			return
		}
		w.Header().Set("Cache-Control", "no-store, private, max-age=0")
		response.Success(w, h.manager.List())
	case r.URL.Path == gatewayGroupsAPIPrefix+"/create":
		h.create(w, r)
	case strings.HasPrefix(r.URL.Path, gatewayGroupsAPIPrefix+"/"):
		h.item(w, r)
	default:
		response.ErrorWithStatus(w, http.StatusNotFound, "gateway group route not found", "NOT_FOUND")
	}
}

func (h *gatewayGroupHTTP) create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var input gatewayGroupRequest
	if err := decodeGatewayGroupRequest(w, r, &input); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, "invalid gateway group request", "INVALID_REQUEST")
		return
	}
	created, err := h.manager.Create(input.Name, input.ClientIDs)
	if err != nil {
		h.writeManagerError(w, err)
		return
	}
	response.Success(w, created)
}

func (h *gatewayGroupHTTP) item(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, gatewayGroupsAPIPrefix+"/")
	if id == "" || strings.Contains(id, "/") || id == "create" {
		response.ErrorWithStatus(w, http.StatusNotFound, "gateway group not found", "GATEWAY_GROUP_NOT_FOUND")
		return
	}
	switch r.Method {
	case http.MethodGet:
		group, exists := h.manager.Get(id)
		if !exists {
			response.ErrorWithStatus(w, http.StatusNotFound, "gateway group not found", "GATEWAY_GROUP_NOT_FOUND")
			return
		}
		response.Success(w, group)
	case http.MethodPut:
		var input gatewayGroupRequest
		if err := decodeGatewayGroupRequest(w, r, &input); err != nil {
			response.ErrorWithStatus(w, http.StatusBadRequest, "invalid gateway group request", "INVALID_REQUEST")
			return
		}
		updated, err := h.manager.Update(id, input.Name, input.ClientIDs)
		if err != nil {
			h.writeManagerError(w, err)
			return
		}
		response.Success(w, updated)
	case http.MethodDelete:
		if err := h.manager.Delete(id); err != nil {
			h.writeManagerError(w, err)
			return
		}
		response.Success(w, map[string]bool{"deleted": true})
	default:
		response.MethodNotAllowed(w)
	}
}

func decodeGatewayGroupRequest(w http.ResponseWriter, r *http.Request, input *gatewayGroupRequest) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(input); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func (h *gatewayGroupHTTP) writeManagerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gatewaygroups.ErrGroupNotFound):
		response.ErrorWithStatus(w, http.StatusNotFound, "gateway group not found", "GATEWAY_GROUP_NOT_FOUND")
	case errors.Is(err, gatewaygroups.ErrInvalidGroup), errors.Is(err, gatewaygroups.ErrClientNotFound):
		response.ErrorWithStatus(w, http.StatusBadRequest, "invalid gateway group membership", "INVALID_GATEWAY_GROUP")
	default:
		response.ErrorWithStatus(w, http.StatusInternalServerError, "gateway group could not be persisted", "GATEWAY_GROUP_PERSIST_ERROR")
	}
}
