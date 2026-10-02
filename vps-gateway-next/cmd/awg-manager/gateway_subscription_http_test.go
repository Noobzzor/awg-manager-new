package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"testing"
	"time"

	gatewayingress "github.com/hoaxisr/awg-manager/internal/gateway/ingress"
	gatewaysubscription "github.com/hoaxisr/awg-manager/internal/gateway/subscription"
)

func newGatewaySubscriptionTestHandler(t *testing.T) (*gatewayClientHTTP, *gatewayingress.Client, *gatewaysubscription.Manager) {
	t.Helper()
	allocator, err := gatewayingress.NewAllocator(netip.MustParsePrefix("10.66.0.0/29"))
	if err != nil {
		t.Fatal(err)
	}
	_ = allocator.ReserveAddress(netip.MustParseAddr("10.66.0.1"))
	registry := gatewayingress.NewRegistry(allocator, func(string) (string, string, error) {
		return "client-private", "client-public", nil
	})
	client, err := registry.Create("phone", "Phone")
	if err != nil {
		t.Fatal(err)
	}
	h := newGatewayClientHTTP(registry, filepath.Join(t.TempDir(), "clients.json"), &fixedGatewayPeerApplier{
		status: gatewayingress.PeerStatus{PublicKey: "client-public"},
	})
	h.SetClientConfig("server-public", "gateway.example:51820")
	store, err := gatewaysubscription.NewFileStore(filepath.Join(t.TempDir(), "subscriptions.json"))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := gatewaysubscription.NewManager(store)
	if err != nil {
		t.Fatal(err)
	}
	h.SetClientSubscriptions(manager)
	if err := h.SetSubscriptionBaseURL("https://gateway.example.com/"); err != nil {
		t.Fatal(err)
	}
	return h, client, manager
}

func TestGatewayClientSubscriptionServesNoStoreProfile(t *testing.T) {
	h, client, manager := newGatewaySubscriptionTestHandler(t)
	issued, err := manager.Create(client.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/s/"+issued.Token, nil)
	response := httptest.NewRecorder()
	h.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	for name, want := range map[string]string{
		"Cache-Control":          "no-store, private, max-age=0",
		"Referrer-Policy":        "no-referrer",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := response.Header().Get(name); got != want {
			t.Errorf("%s=%q want %q", name, got, want)
		}
	}
	if !strings.Contains(response.Body.String(), "PrivateKey = client-private") || !strings.Contains(response.Body.String(), "Address = 10.66.0.2/32") {
		t.Fatalf("unexpected subscription profile: %s", response.Body.String())
	}
}

func TestGatewayClientSubscriptionAdminCreateListAndRevoke(t *testing.T) {
	h, _, _ := newGatewaySubscriptionTestHandler(t)
	create := httptest.NewRecorder()
	h.ServeHTTP(create, httptest.NewRequest(http.MethodPost, "/api/gateway/subscriptions/create", strings.NewReader(`{"clientId":"phone","expiresInDays":3}`)))
	if create.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	var envelope struct {
		Data struct {
			Subscription gatewaysubscription.Metadata `json:"subscription"`
			URL          string                       `json:"url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	const pathPrefix = "https://gateway.example.com/s/"
	if !strings.HasPrefix(envelope.Data.URL, pathPrefix) || len(strings.TrimPrefix(envelope.Data.URL, pathPrefix)) != 43 {
		t.Fatalf("create response did not return one-time HTTPS subscription URL: %+v", envelope.Data)
	}
	token := strings.TrimPrefix(envelope.Data.URL, pathPrefix)
	if got := create.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Fatalf("create response Cache-Control=%q", got)
	}

	list := httptest.NewRecorder()
	h.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/gateway/subscriptions", nil))
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), token) {
		t.Fatalf("list status=%d leaked token or failed: %s", list.Code, list.Body.String())
	}

	revoke := httptest.NewRecorder()
	revokePath := fmt.Sprintf("/api/gateway/subscriptions/%s/revoke", envelope.Data.Subscription.ID)
	h.ServeHTTP(revoke, httptest.NewRequest(http.MethodPost, revokePath, nil))
	if revoke.Code != http.StatusOK {
		t.Fatalf("revoke status=%d body=%s", revoke.Code, revoke.Body.String())
	}
	public := httptest.NewRecorder()
	h.ServeHTTP(public, httptest.NewRequest(http.MethodGet, envelope.Data.URL, nil))
	if public.Code != http.StatusNotFound {
		t.Fatalf("revoked subscription status=%d body=%s", public.Code, public.Body.String())
	}
}

func TestGatewayClientSubscriptionCreateRequiresReadyProfile(t *testing.T) {
	h, _, manager := newGatewaySubscriptionTestHandler(t)
	h.SetClientConfig("server-public", "")
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/gateway/subscriptions/create", strings.NewReader(`{"clientId":"phone","expiresInDays":3}`)))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured endpoint create status=%d body=%s", response.Code, response.Body.String())
	}
	if subscriptions := manager.List(); len(subscriptions) != 0 {
		t.Fatalf("unready gateway created %d subscription(s)", len(subscriptions))
	}
}

func TestGatewayClientSubscriptionCreateRequiresPublicURL(t *testing.T) {
	h, _, manager := newGatewaySubscriptionTestHandler(t)
	if err := h.SetSubscriptionBaseURL(""); err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/gateway/subscriptions/create", strings.NewReader(`{"clientId":"phone","expiresInDays":3}`)))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing public URL create status=%d body=%s", response.Code, response.Body.String())
	}
	if subscriptions := manager.List(); len(subscriptions) != 0 {
		t.Fatalf("missing public URL created %d subscription(s)", len(subscriptions))
	}
}

func TestValidateGatewaySubscriptionBaseURLRequiresHTTPSOrigin(t *testing.T) {
	valid, err := validateGatewaySubscriptionBaseURL(" https://gateway.example.com/ ")
	if err != nil || valid != "https://gateway.example.com" {
		t.Fatalf("valid origin = %q, %v", valid, err)
	}
	for _, invalid := range []string{
		"http://gateway.example.com",
		"https://user@gateway.example.com",
		"https://gateway.example.com/path",
		"https://gateway.example.com/?token=x",
		"https://gateway.example.com/#fragment",
	} {
		if got, err := validateGatewaySubscriptionBaseURL(invalid); err == nil {
			t.Errorf("validateGatewaySubscriptionBaseURL(%q) = %q, nil error", invalid, got)
		}
	}
}

func TestGatewaySubscriptionPublicRateLimitIsPerSubscription(t *testing.T) {
	h, client, manager := newGatewaySubscriptionTestHandler(t)
	issued, err := manager.Create(client.ID, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	h.subscriptions.limiter = newSubscriptionRequestLimiter(2, time.Minute, 8)
	path := "/s/" + issued.Token
	for index, want := range []int{http.StatusOK, http.StatusOK, http.StatusTooManyRequests} {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != want {
			t.Fatalf("request %d status=%d want=%d body=%s", index+1, response.Code, want, response.Body.String())
		}
	}
}
