package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

// ListRules returns all singbox-router routing rules in priority order.
//
//	@Summary		List singbox-router rules
//	@Description	Returns all routing rules in priority (top-first) order.
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterRulesListResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/rules/list [get]
func (h *SingboxRouterHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	rules, err := h.svc.ListRules(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, rules)
}

// AddRule appends a new singbox-router routing rule.
//
//	@Summary		Add singbox-router rule
//	@Description	Appends a new routing rule. Rule conditions reference rulesets/outbounds that must already exist.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleDTO	true	"Routing rule payload"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rules/add [post]
func (h *SingboxRouterHandler) AddRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var rule router.Rule
	if err := decodeBody(r, &rule); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.AddRule(r.Context(), rule); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	h.log.Info("rule-add", rule.Outbound, "routing rule added (outbound: "+rule.Outbound+")")
	response.Success(w, map[string]bool{"ok": true})
}

// UpdateRule replaces a rule at the given index with the provided one.
//
//	@Summary		Update singbox-router rule
//	@Description	Replaces the rule at index with the provided one. Index is the priority slot (0-based).
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleUpdateRequest	true	"Index + replacement rule"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rules/update [post]
func (h *SingboxRouterHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		Index int         `json:"index"`
		Rule  router.Rule `json:"rule"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.UpdateRule(r.Context(), body.Index, body.Rule); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	h.log.Info("rule-update", strconv.Itoa(body.Index),
		"routing rule updated at index "+strconv.Itoa(body.Index)+" (outbound: "+body.Rule.Outbound+")")
	response.Success(w, map[string]bool{"ok": true})
}

// BulkSetRuleOutbound sets Outbound on every rule at the given indices in a
// single config write.
//
//	@Summary		Bulk-set outbound on singbox-router rules
//	@Description	Sets Outbound on every route rule at the given indices in a single config write. Rejects an empty/duplicate index list, an out-of-range index, a non-route rule, or an unknown outbound tag.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleBulkOutboundRequest	true	"Rule indices + new outbound tag"
//	@Success		200		{object}	SingboxRouterBulkUpdatedResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rules/bulk-outbound [post]
func (h *SingboxRouterHandler) BulkSetRuleOutbound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body SingboxRouterRuleBulkOutboundRequest
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.BulkSetRuleOutbound(r.Context(), body.Indices, body.Outbound); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	h.log.Info("rules-bulk-outbound", body.Outbound,
		strconv.Itoa(len(body.Indices))+" routing rule(s) set to outbound "+body.Outbound)
	response.Success(w, map[string]int{"updated": len(body.Indices)})
}

// DeleteRule removes the rule at the given index.
//
//	@Summary		Delete singbox-router rule
//	@Description	Removes the rule at the given index (0-based priority slot).
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleDeleteRequest	true	"Index of the rule to remove"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rules/delete [post]
func (h *SingboxRouterHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		Index int `json:"index"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.DeleteRule(r.Context(), body.Index); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	h.log.Info("rule-delete", strconv.Itoa(body.Index), "routing rule deleted at index "+strconv.Itoa(body.Index))
	response.Success(w, map[string]bool{"ok": true})
}

// MoveRule moves the rule from one priority slot to another.
//
//	@Summary		Move singbox-router rule
//	@Description	Moves the rule from index `from` to index `to` (both 0-based). Adjusts other rules' indices accordingly.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleMoveRequest	true	"From-index and to-index"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rules/move [post]
func (h *SingboxRouterHandler) MoveRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.MoveRule(r.Context(), body.From, body.To); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	h.log.Info("rule-move", strconv.Itoa(body.From),
		"routing rule moved from index "+strconv.Itoa(body.From)+" to "+strconv.Itoa(body.To))
	response.Success(w, map[string]bool{"ok": true})
}

// ListRuleSets returns all configured rulesets.
//
//	@Summary		List singbox-router rulesets
//	@Description	Returns all configured rulesets (downloaded geo files / inline lists), with their tag, type, and freshness metadata.
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterRuleSetsListResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/rulesets/list [get]
func (h *SingboxRouterHandler) ListRuleSets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	rs, err := h.svc.ListRuleSets(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, rs)
}

// AddRuleSet registers a new ruleset (downloads if remote).
//
//	@Summary		Add singbox-router ruleset
//	@Description	Registers a new ruleset. For remote rulesets the file is downloaded synchronously.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleSetDTO	true	"RuleSet payload"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rulesets/add [post]
func (h *SingboxRouterHandler) AddRuleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var rs router.RuleSet
	if err := decodeBody(r, &rs); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.AddRuleSet(r.Context(), rs); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	h.log.Info("ruleset-add", rs.Tag, "ruleset added: "+rs.Tag)
	response.Success(w, map[string]bool{"ok": true})
}

// UpdateRuleSet replaces the ruleset identified by tag with new content.
//
//	@Summary		Update singbox-router ruleset
//	@Description	Replaces the ruleset identified by tag with new content. If the payload tag differs, references are renamed atomically.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleSetUpdateRequest	true	"Tag + new RuleSet payload"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		404		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rulesets/update [post]
func (h *SingboxRouterHandler) UpdateRuleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		Tag     string         `json:"tag"`
		RuleSet router.RuleSet `json:"ruleSet"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if body.Tag == "" {
		response.BadRequest(w, "tag is required")
		return
	}
	if err := h.svc.UpdateRuleSet(r.Context(), body.Tag, body.RuleSet); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	h.log.Info("ruleset-update", body.Tag, "ruleset updated: "+body.Tag)
	response.Success(w, map[string]bool{"ok": true})
}

// BulkSetRuleSetDetour sets download_detour on every rule set with a tag in
// the given list, in a single config write.
//
//	@Summary		Bulk-set download_detour on singbox-router rulesets
//	@Description	Sets download_detour on every rule set with a tag in the given list, in a single config write. Rejects an empty/duplicate tag list, an unknown tag, a rule set whose type isn't "remote", or an unknown outbound tag (empty detour clears the field and skips the known-tag check).
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleSetBulkDetourRequest	true	"Rule set tags + new download_detour"
//	@Success		200		{object}	SingboxRouterBulkUpdatedResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rulesets/bulk-detour [post]
func (h *SingboxRouterHandler) BulkSetRuleSetDetour(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body SingboxRouterRuleSetBulkDetourRequest
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.BulkSetRuleSetDetour(r.Context(), body.Tags, body.DownloadDetour); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	h.log.Info("rulesets-bulk-detour", body.DownloadDetour,
		strconv.Itoa(len(body.Tags))+" rule set(s) detour set to "+body.DownloadDetour)
	response.Success(w, map[string]int{"updated": len(body.Tags)})
}

// DeleteRuleSet removes the ruleset identified by tag.
//
//	@Summary		Delete singbox-router ruleset
//	@Description	Removes the ruleset identified by tag. Refuses if any rule references it; pass force=true to remove this rule_set tag from referencing route and DNS rules.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleSetDeleteRequest	true	"Tag + optional force flag"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		409		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rulesets/delete [post]
func (h *SingboxRouterHandler) DeleteRuleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		Tag   string `json:"tag"`
		Force bool   `json:"force"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.DeleteRuleSet(r.Context(), body.Tag, body.Force); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	h.log.Info("ruleset-delete", body.Tag, "ruleset deleted: "+body.Tag)
	response.Success(w, map[string]bool{"ok": true})
}

// DatRuleSetURL returns the local tokenized URL that sing-box can fetch directly.
//
//	@Summary		Build dat→SRS rule-set URL
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Param			kind	query	string	true	"geosite or geoip"
//	@Param			tag		query	[]string	true	"Geo tag(s)"
//	@Success		200	{object}	SingboxRouterDatRuleSetURLResponse
//	@Failure		400	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/rulesets/dat-url [get]
func (h *SingboxRouterHandler) DatRuleSetURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	kind := r.URL.Query().Get("kind")
	tags := nonEmptyQueryValues(r.URL.Query()["tag"])
	if (kind != "geosite" && kind != "geoip") || len(tags) == 0 {
		response.BadRequest(w, "kind must be geosite or geoip, tag is required")
		return
	}
	u, err := h.svc.DatRuleSetURL(r.Context(), kind, tags)
	if err != nil {
		h.handleErr(w, "dat-url", err)
		return
	}
	response.Success(w, SingboxRouterDatRuleSetURLData{URL: u})
}

// DatRuleSetSRS serves a compiled .srs artifact for sing-box. It is intentionally
// not protected by session cookies because sing-box fetches it as a plain remote
// rule_set URL; access is controlled by the token in the URL.
func (h *SingboxRouterHandler) DatRuleSetSRS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	kind := r.URL.Query().Get("kind")
	tags := nonEmptyQueryValues(r.URL.Query()["tag"])
	if (kind != "geosite" && kind != "geoip") || len(tags) == 0 {
		response.BadRequest(w, "kind must be geosite or geoip, tag is required")
		return
	}
	p, err := h.svc.DatRuleSetFile(
		r.Context(),
		kind,
		tags,
		r.URL.Query().Get("token"),
	)
	if err != nil {
		if errors.Is(err, router.ErrDatRuleSetForbidden) {
			response.ErrorWithStatus(w, http.StatusForbidden, "invalid dat rule-set token", "FORBIDDEN")
			return
		}
		h.handleErr(w, "dat-srs", err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="rule-set.srs"`)
	http.ServeFile(w, r, p)
}

func nonEmptyQueryValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

// SetRouteFinal updates route.final.
//
//	@Summary		Set route.final outbound
//	@Description	Updates the route.final fallback outbound. Use "direct" for default sing-box direct, or the tag of any existing outbound (composite, AWG, sing-box tunnel).
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRouteFinalRequest	true	"New final outbound tag"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		405		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/route/final [post]
func (h *SingboxRouterHandler) SetRouteFinal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req SingboxRouterRouteFinalRequest
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.SetRouteFinal(r.Context(), req.Final); err != nil {
		h.handleErr(w, "route-final", err)
		return
	}
	h.log.Info("route-final", req.Final, "route.final set to "+req.Final)
	response.Success(w, map[string]bool{"ok": true})
}
