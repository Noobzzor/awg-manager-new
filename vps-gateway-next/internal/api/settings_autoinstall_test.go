package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestUpdate_AutoInstallIntervalDays_OutOfRangeRejected verifies that
// updates.autoInstallIntervalDays outside 1..30 is rejected with 400 and
// INVALID_AUTO_INSTALL_INTERVAL, instead of being silently saved.
func TestUpdate_AutoInstallIntervalDays_OutOfRangeRejected(t *testing.T) {
	h, store := newSettingsHandlerForTest(t)
	current, _ := store.Get()

	for _, days := range []int{0, 31} {
		payload := *current
		payload.Updates.AutoInstallIntervalDays = days
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/settings/update", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		h.Update(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("days=%d: status = %d, want 400, body=%s", days, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "INVALID_AUTO_INSTALL_INTERVAL") {
			t.Fatalf("days=%d: missing INVALID_AUTO_INSTALL_INTERVAL, body=%s", days, rec.Body.String())
		}
	}
}

// TestUpdate_AutoInstallTime_InvalidFormatRejected verifies that
// updates.autoInstallTime not matching HH:MM (24h) is rejected with 400
// and INVALID_AUTO_INSTALL_TIME.
func TestUpdate_AutoInstallTime_InvalidFormatRejected(t *testing.T) {
	h, store := newSettingsHandlerForTest(t)
	current, _ := store.Get()

	for _, tm := range []string{"25:99", "5:00"} {
		payload := *current
		payload.Updates.AutoInstallTime = tm
		body, _ := json.Marshal(payload)

		req := httptest.NewRequest(http.MethodPost, "/settings/update", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		h.Update(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("time=%q: status = %d, want 400, body=%s", tm, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "INVALID_AUTO_INSTALL_TIME") {
			t.Fatalf("time=%q: missing INVALID_AUTO_INSTALL_TIME, body=%s", tm, rec.Body.String())
		}
	}
}

// TestUpdate_AutoInstallSettings_ValidAccepted verifies that valid
// autoInstall settings (interval within 1..30, time "05:00") are accepted
// and persisted.
func TestUpdate_AutoInstallSettings_ValidAccepted(t *testing.T) {
	h, store := newSettingsHandlerForTest(t)
	current, _ := store.Get()

	payload := *current
	payload.Updates.AutoInstallEnabled = true
	payload.Updates.AutoInstallIntervalDays = 14
	payload.Updates.AutoInstallTime = "05:00"
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/settings/update", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	got, _ := store.Get()
	if !got.Updates.AutoInstallEnabled || got.Updates.AutoInstallIntervalDays != 14 || got.Updates.AutoInstallTime != "05:00" {
		t.Errorf("Updates after update = %+v, want enabled=true interval=14 time=05:00", got.Updates)
	}
}
