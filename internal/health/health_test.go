package health_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Thoomaastb/CTRLD/internal/health"
)

func TestHandler_StatusOK(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	health.Handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("erwartet status 200, bekommen %d", rec.Code)
	}
}

func TestHandler_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	health.Handler(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("erwartet Content-Type application/json, bekommen %q", ct)
	}
}

func TestHandler_ResponseBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	before := time.Now().UTC()
	health.Handler(rec, req)
	after := time.Now().UTC()

	var resp health.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("fehler beim dekodieren der antwort: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("erwartet status 'ok', bekommen %q", resp.Status)
	}

	if resp.Timestamp.Before(before) || resp.Timestamp.After(after) {
		t.Errorf("timestamp %v liegt außerhalb des erwarteten bereichs [%v, %v]",
			resp.Timestamp, before, after)
	}
}

func TestHandler_NoCacheHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()

	health.Handler(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Errorf("erwartet Cache-Control: no-store, bekommen %q", cc)
	}
}
