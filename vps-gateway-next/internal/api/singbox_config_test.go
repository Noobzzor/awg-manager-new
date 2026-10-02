package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSingboxConfigHandler_Preview_OK(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "00-base.json"),
		[]byte(`{"log":{"level":"trace"}}`), 0644); err != nil {
		t.Fatal(err)
	}

	h := &SingboxConfigHandler{configDirFn: func() string { return dir }}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/singbox/config-preview", nil)
	h.Preview(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Success bool `json:"success"`
		Data    struct {
			JSON string `json:"json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Success {
		t.Fatalf("expected success=true: %s", rec.Body.String())
	}
	if !strings.Contains(env.Data.JSON, `"trace"`) {
		t.Errorf("merged JSON missing log.level=trace:\n%s", env.Data.JSON)
	}
}

func TestSingboxConfigHandler_Preview_CollisionReturns500(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "10-a.json"),
		[]byte(`{"outbounds":[{"tag":"x","type":"vless"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "20-b.json"),
		[]byte(`{"outbounds":[{"tag":"x","type":"direct"}]}`), 0644); err != nil {
		t.Fatal(err)
	}

	h := &SingboxConfigHandler{configDirFn: func() string { return dir }}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/singbox/config-preview", nil)
	h.Preview(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on collision, got %d: %s", rec.Code, rec.Body.String())
	}
	// Body is JSON-encoded so the inner quotes around the tag get escaped
	// to \"x\". Match the unescaped form via the parsed message instead
	// of substring-checking the raw body.
	var env struct {
		Error   bool   `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode error envelope: %v\n%s", err, rec.Body.String())
	}
	if !env.Error {
		t.Fatalf("expected error=true: %s", rec.Body.String())
	}
	if !strings.Contains(env.Message, `"x"`) || !strings.Contains(env.Message, "10-a.json") || !strings.Contains(env.Message, "20-b.json") {
		t.Errorf("expected collision details in error message:\n%s", env.Message)
	}
}
