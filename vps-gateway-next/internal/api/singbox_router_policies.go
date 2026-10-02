package api

import (
	"net/http"

	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

// PoliciesCollection routes by HTTP method:
//
//	GET  → ListPolicies (returns []router.PolicyInfo)
//	POST → CreatePolicy (body: {description}, returns router.PolicyInfo)
func (h *SingboxRouterHandler) PoliciesCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listPolicies(w, r)
	case http.MethodPost:
		h.createPolicy(w, r)
	default:
		response.MethodNotAllowed(w)
	}
}

// listPolicies returns all NDMS policies known to the singbox-router engine.
//
//	@Summary		List singbox-router policies
//	@Description	Returns all NDMS policies known to the singbox-router engine. Always a JSON array, never null.
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterPoliciesListResponse
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/policies [get]
func (h *SingboxRouterHandler) listPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := h.svc.ListPolicies(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if policies == nil {
		policies = []router.PolicyInfo{}
	}
	response.Success(w, policies)
}

// createPolicy creates a new NDMS policy with the given description.
//
//	@Summary		Create singbox-router policy
//	@Description	Creates a new NDMS policy with the given description. Returns the created policy.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterCreatePolicyRequest	true	"Policy description"
//	@Success		200		{object}	SingboxRouterPolicyResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/policies [post]
func (h *SingboxRouterHandler) createPolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description string `json:"description"`
	}
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, "invalid body")
		return
	}
	policy, err := h.svc.CreatePolicy(r.Context(), req.Description)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	h.log.Info("policy-create", policy.Name, "NDMS policy created: "+policy.Name)
	response.Success(w, policy)
}

// ListPolicyDevices handles GET /api/singbox/router/policy-devices?name=X
//
//	@Summary		List singbox-router policy devices
//	@Description	Returns the LAN devices currently bound to the named policy. Always a JSON array, never null.
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Param			name	query		string	true	"Policy name"
//	@Success		200		{object}	SingboxRouterPolicyDevicesListResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/policy-devices [get]
func (h *SingboxRouterHandler) ListPolicyDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	policyName := r.URL.Query().Get("name")
	if policyName == "" {
		response.Error(w, "missing name parameter", "MISSING_NAME")
		return
	}
	devices, err := h.svc.ListPolicyDevices(r.Context(), policyName)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if devices == nil {
		devices = []router.PolicyDevice{}
	}
	response.Success(w, devices)
}

// BindDevice handles POST /api/singbox/router/policy-devices/bind
//
//	@Summary		Bind device to singbox-router policy
//	@Description	Binds the LAN device (MAC) to the named policy. Replaces any existing binding.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterBindDeviceRequest	true	"Device MAC + target policy name"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/policy-devices/bind [post]
func (h *SingboxRouterHandler) BindDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req struct {
		MAC        string `json:"mac"`
		PolicyName string `json:"policyName"`
	}
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, "invalid body")
		return
	}
	if err := h.svc.BindDevice(r.Context(), req.MAC, req.PolicyName); err != nil {
		response.InternalError(w, err.Error())
		return
	}
	h.log.Info("device-bind", req.MAC, "device bound to policy "+req.PolicyName)
	response.Success(w, map[string]bool{"ok": true})
}

// UnbindDevice handles POST /api/singbox/router/policy-devices/unbind
//
//	@Summary		Unbind device from singbox-router policy
//	@Description	Removes any policy binding for the LAN device identified by MAC.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterUnbindDeviceRequest	true	"Device MAC"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/policy-devices/unbind [post]
func (h *SingboxRouterHandler) UnbindDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req struct {
		MAC string `json:"mac"`
	}
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, "invalid body")
		return
	}
	if err := h.svc.UnbindDevice(r.Context(), req.MAC); err != nil {
		response.InternalError(w, err.Error())
		return
	}
	h.log.Info("device-unbind", req.MAC, "device policy binding removed")
	response.Success(w, map[string]bool{"ok": true})
}
