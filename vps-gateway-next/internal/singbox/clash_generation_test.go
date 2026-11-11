package singbox

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestClashGenerationVerification(t *testing.T) {
	tag := generationTestPrefix + strings.Repeat("a", 64)
	other := generationTestPrefix + strings.Repeat("b", 64)
	for _, tc := range []struct {
		name, body string
		ok         bool
	}{
		{"exact", `{"proxies":{"` + tag + `":{"name":"` + tag + `","type":"Direct"}}}`, true},
		{"absent", `{"proxies":{"direct":{"name":"direct","type":"Direct"}}}`, false},
		{"wrong-type", `{"proxies":{"` + tag + `":{"name":"` + tag + `","type":"Selector"}}}`, false},
		{"wrong-name", `{"proxies":{"` + tag + `":{"name":"different","type":"Direct"}}}`, false},
		{"competing", `{"proxies":{"` + tag + `":{"name":"` + tag + `","type":"Direct"},"` + other + `":{"name":"` + other + `","type":"Direct"}}}`, false},
		{"malformed", `{`, false},
		{"trailing", `{"proxies":{"` + tag + `":{"name":"` + tag + `","type":"Direct"}}}{}`, false},
		{"oversize", `{"proxies":{"` + tag + `":{"name":"` + tag + `","type":"Direct"}}}` + strings.Repeat(" ", 4*1024*1024+1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/proxies" {
					t.Error("wrong live endpoint")
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer s.Close()
			c := NewClashClient(strings.TrimPrefix(s.URL, "http://"))
			err := c.VerifyRuntimeGeneration(context.Background(), tag)
			if (err == nil) != tc.ok {
				t.Fatalf("generation witness accepted=%v want=%v", err == nil, tc.ok)
			}
		})
	}
}

func TestClashGenerationCancelledBeforeRequest(t *testing.T) {
	var calls atomic.Int32
	tag := generationTestPrefix + strings.Repeat("a", 64)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"proxies":{"` + tag + `":{"name":"` + tag + `","type":"Direct"}}}`))
	}))
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := NewClashClient(strings.TrimPrefix(s.URL, "http://")).VerifyRuntimeGeneration(ctx, tag)
	if !errors.Is(err, context.Canceled) || calls.Load() != 0 {
		t.Fatal("cancelled witness still requested or accepted")
	}
}

func TestClashGenerationDeadlineCancelsActualRequest(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := NewClashClient(strings.TrimPrefix(s.URL, "http://")).VerifyRuntimeGeneration(ctx, generationTestPrefix+strings.Repeat("a", 64))
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 500*time.Millisecond {
		t.Fatal("live request ignored the caller deadline")
	}
}

func TestClashGenerationRejectsInvalidExpectedTag(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"proxies":{"arbitrary":{"name":"arbitrary","type":"Direct"}}}`))
	}))
	defer s.Close()
	if NewClashClient(strings.TrimPrefix(s.URL, "http://")).VerifyRuntimeGeneration(context.Background(), "arbitrary") == nil {
		t.Fatal("arbitrary outbound accepted as a generation witness")
	}
}

func TestClashGenerationRejectsDeepResponse(t *testing.T) {
	tag := generationTestPrefix + strings.Repeat("a", 64)
	body := `{"proxies":{"` + tag + `":{"name":"` + tag + `","type":"Direct"}},"custom":` + strings.Repeat("[", 260) + "0" + strings.Repeat("]", 260) + "}"
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
	defer s.Close()
	if NewClashClient(strings.TrimPrefix(s.URL, "http://")).VerifyRuntimeGeneration(context.Background(), tag) == nil {
		t.Fatal("deep live response accepted without a nesting bound")
	}
}
