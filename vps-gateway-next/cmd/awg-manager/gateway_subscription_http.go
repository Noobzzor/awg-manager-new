package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
	gatewaysubscription "github.com/hoaxisr/awg-manager/internal/gateway/subscription"
	"github.com/hoaxisr/awg-manager/internal/response"
)

const (
	gatewaySubscriptionPathPrefix = "/s/"
	gatewaySubscriptionAPIPrefix  = "/api/gateway/subscriptions"
)

type gatewaySubscriptionHTTP struct {
	manager       *gatewaysubscription.Manager
	clients       *gatewayClientHTTP
	publicBaseURL string
	limiter       *subscriptionRequestLimiter
}

type gatewaySubscriptionCreateRequest struct {
	ClientID      string `json:"clientId"`
	ExpiresInDays int    `json:"expiresInDays"`
}

type gatewaySubscriptionCreateResponse struct {
	Subscription gatewaysubscription.Metadata `json:"subscription"`
	URL          string                       `json:"url"`
}

func newGatewaySubscriptionHTTP(manager *gatewaysubscription.Manager, clients *gatewayClientHTTP) *gatewaySubscriptionHTTP {
	if manager == nil || clients == nil {
		return nil
	}
	return &gatewaySubscriptionHTTP{
		manager: manager,
		clients: clients,
		limiter: newSubscriptionRequestLimiter(60, time.Minute, 4096),
	}
}

func validateGatewaySubscriptionBaseURL(raw string) (string, error) {
	base := strings.TrimSpace(raw)
	if base == "" {
		return "", nil
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", errors.New("AWG_GATEWAY_SUBSCRIPTION_BASE_URL must be an HTTPS origin without credentials, path, query, or fragment")
	}
	return strings.TrimRight(parsed.String(), "/"), nil
}

func (h *gatewaySubscriptionHTTP) ServeAdmin(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == gatewaySubscriptionAPIPrefix:
		if r.Method != http.MethodGet {
			response.MethodNotAllowed(w)
			return
		}
		w.Header().Set("Cache-Control", "no-store, private, max-age=0")
		response.Success(w, h.manager.List())
	case r.URL.Path == gatewaySubscriptionAPIPrefix+"/create":
		h.create(w, r)
	case strings.HasPrefix(r.URL.Path, gatewaySubscriptionAPIPrefix+"/"):
		h.revoke(w, r)
	default:
		response.ErrorWithStatus(w, http.StatusNotFound, "gateway subscription route not found", "NOT_FOUND")
	}
}

func (h *gatewaySubscriptionHTTP) ServePublic(w http.ResponseWriter, r *http.Request) {
	setGatewayProfileHeaders(w)
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		response.MethodNotAllowed(w)
		return
	}
	if !strings.HasPrefix(r.URL.Path, gatewaySubscriptionPathPrefix) {
		if !h.limiter.Allow("invalid") {
			rejectSubscriptionRateLimit(w)
			return
		}
		hideSubscriptionNotFound(w, r)
		return
	}
	token := strings.TrimPrefix(r.URL.Path, gatewaySubscriptionPathPrefix)
	if token == "" || strings.Contains(token, "/") {
		if !h.limiter.Allow("invalid") {
			rejectSubscriptionRateLimit(w)
			return
		}
		hideSubscriptionNotFound(w, r)
		return
	}
	metadata, err := h.manager.Resolve(token)
	limitKey := "invalid"
	if err == nil {
		limitKey = "subscription:" + metadata.ID
	}
	if !h.limiter.Allow(limitKey) {
		rejectSubscriptionRateLimit(w)
		return
	}
	if err != nil {
		hideSubscriptionNotFound(w, r)
		return
	}
	config, err := h.clients.clientConfig(r.Context(), metadata.ClientID)
	if errors.Is(err, gatewayingress.ErrClientNotFound) || errors.Is(err, errGatewayClientConfigUnavailable) {
		hideSubscriptionNotFound(w, r)
		return
	}
	if err != nil {
		response.ErrorWithStatus(w, http.StatusServiceUnavailable, "subscription profile is temporarily unavailable", "GATEWAY_PROFILE_UNAVAILABLE")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="awg-client.conf"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(config))
}

func (h *gatewaySubscriptionHTTP) create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var input gatewaySubscriptionCreateRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, "invalid subscription request", "INVALID_REQUEST")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		response.ErrorWithStatus(w, http.StatusBadRequest, "invalid subscription request", "INVALID_REQUEST")
		return
	}
	input.ClientID = strings.TrimSpace(input.ClientID)
	if input.ClientID == "" || input.ExpiresInDays < 1 || input.ExpiresInDays > 365 {
		response.ErrorWithStatus(w, http.StatusBadRequest, "clientId and expiresInDays (1-365) are required", "INVALID_REQUEST")
		return
	}
	if h.publicBaseURL == "" {
		response.ErrorWithStatus(w, http.StatusServiceUnavailable, "public HTTPS subscription URL is not configured", "GATEWAY_SUBSCRIPTION_URL_UNCONFIGURED")
		return
	}
	client, ok := h.clients.registry.Get(input.ClientID)
	if !ok {
		response.ErrorWithStatus(w, http.StatusNotFound, "client not found", "GATEWAY_CLIENT_NOT_FOUND")
		return
	}
	if client.Status != gatewayingress.StatusActive {
		response.ErrorWithStatus(w, http.StatusConflict, "subscriptions require an active client", "GATEWAY_CLIENT_CONFIG_UNAVAILABLE")
		return
	}
	if _, err := h.clients.clientConfig(r.Context(), client.ID); err != nil {
		switch {
		case errors.Is(err, errGatewayEndpointUnconfigured):
			response.ErrorWithStatus(w, http.StatusServiceUnavailable, "gateway public endpoint is not configured", "GATEWAY_ENDPOINT_UNCONFIGURED")
		case errors.Is(err, errGatewayPeerStatusUnavailable):
			response.ErrorWithStatus(w, http.StatusServiceUnavailable, "gateway peer status is unavailable", "GATEWAY_PEER_STATUS_UNAVAILABLE")
		case errors.Is(err, errGatewayClientConfigUnavailable):
			response.ErrorWithStatus(w, http.StatusConflict, "client profile is unavailable", "GATEWAY_CLIENT_CONFIG_UNAVAILABLE")
		default:
			response.ErrorWithStatus(w, http.StatusServiceUnavailable, "client profile is temporarily unavailable", "GATEWAY_PROFILE_UNAVAILABLE")
		}
		return
	}
	issued, err := h.manager.Create(client.ID, time.Now().UTC().Add(time.Duration(input.ExpiresInDays)*24*time.Hour))
	if err != nil {
		response.ErrorWithStatus(w, http.StatusInternalServerError, "subscription could not be persisted", "GATEWAY_SUBSCRIPTION_PERSIST_ERROR")
		return
	}
	w.Header().Set("Cache-Control", "no-store, private, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	response.Success(w, gatewaySubscriptionCreateResponse{
		Subscription: issued.Subscription,
		URL:          h.publicBaseURL + gatewaySubscriptionPathPrefix + issued.Token,
	})
}

func (h *gatewaySubscriptionHTTP) revoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	relative := strings.TrimPrefix(r.URL.Path, gatewaySubscriptionAPIPrefix+"/")
	parts := strings.Split(relative, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] != "revoke" {
		response.ErrorWithStatus(w, http.StatusNotFound, "gateway subscription route not found", "NOT_FOUND")
		return
	}
	if err := h.manager.Revoke(parts[0]); errors.Is(err, gatewaysubscription.ErrNotFound) {
		response.ErrorWithStatus(w, http.StatusNotFound, "gateway subscription not found", "GATEWAY_SUBSCRIPTION_NOT_FOUND")
		return
	} else if err != nil {
		response.ErrorWithStatus(w, http.StatusInternalServerError, "subscription revocation could not be persisted", "GATEWAY_SUBSCRIPTION_PERSIST_ERROR")
		return
	}
	response.Success(w, map[string]bool{"revoked": true})
}

func hideSubscriptionNotFound(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store, private, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.NotFound(w, r)
}

func setGatewayProfileHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store, private, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func rejectSubscriptionRateLimit(w http.ResponseWriter) {
	w.Header().Set("Retry-After", "60")
	response.ErrorWithStatus(w, http.StatusTooManyRequests, "subscription request rate limit exceeded", "RATE_LIMITED")
}

type subscriptionRequestBucket struct {
	windowStart time.Time
	lastSeen    time.Time
	count       int
}

type subscriptionRequestLimiter struct {
	mu       sync.Mutex
	now      func() time.Time
	limit    int
	window   time.Duration
	capacity int
	buckets  map[string]subscriptionRequestBucket
}

func newSubscriptionRequestLimiter(limit int, window time.Duration, capacity int) *subscriptionRequestLimiter {
	return &subscriptionRequestLimiter{
		now: time.Now, limit: limit, window: window, capacity: capacity,
		buckets: make(map[string]subscriptionRequestBucket),
	}
}

func (l *subscriptionRequestLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for candidate, bucket := range l.buckets {
		if now.Sub(bucket.lastSeen) >= l.window {
			delete(l.buckets, candidate)
		}
	}
	bucket, exists := l.buckets[key]
	if !exists {
		if l.capacity > 0 && len(l.buckets) >= l.capacity {
			oldestKey := ""
			var oldest time.Time
			for candidate, current := range l.buckets {
				if oldestKey == "" || current.lastSeen.Before(oldest) {
					oldestKey, oldest = candidate, current.lastSeen
				}
			}
			delete(l.buckets, oldestKey)
		}
		bucket = subscriptionRequestBucket{windowStart: now}
	} else if now.Sub(bucket.windowStart) >= l.window {
		bucket = subscriptionRequestBucket{windowStart: now}
	}
	bucket.lastSeen = now
	bucket.count++
	l.buckets[key] = bucket
	return bucket.count <= l.limit
}
