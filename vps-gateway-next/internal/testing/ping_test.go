package testing

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/sys/httpclient"
)

// TestPingByIface_Success verifies latency computation from a real httptest server.
func TestPingByIface_Success(t *testing.T) {
	s := NewService(nil, nil)

	tsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer tsrv.Close()

	hostPort := strings.TrimPrefix(tsrv.URL, "http://")
	hostPort = strings.TrimSuffix(hostPort, "/")
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ms, err := s.PingByIface(ctx, "", host, port)
	if err != nil {
		t.Fatalf("PingByIface error: %v", err)
	}
	if ms < 0 {
		t.Errorf("latency = %d, want >= 0", ms)
	}
}

// stubDoer implements HTTPDoer for call-site tests.
type stubDoer struct {
	result *httpclient.Result
	err    error
}

func (s stubDoer) Do(_ context.Context, _ httpclient.CallConfig) (*httpclient.Result, error) {
	return s.result, s.err
}
