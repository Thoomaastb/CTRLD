package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	database "github.com/Thoomaastb/CTRLD/internal/db"
)

// newTestHTTPServer erstellt einen Test-Server mit minimaler Konfiguration.
// Nutzt die interne testConfig()-Funktion um Config-Import im Test zu vermeiden.
func newTestHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()

	db, err := database.Open(":memory:", zerolog.Nop())
	if err != nil {
		t.Fatalf("db öffnen fehlgeschlagen: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := defaultTestConfig()
	srv := New(cfg, db, zerolog.Nop())
	return httptest.NewServer(srv.Handler())
}

func TestHealthEndpoint(t *testing.T) {
	ts := newTestHTTPServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("erwartet 200, bekommen %d", resp.StatusCode)
	}
}

func TestSecurityHeaders(t *testing.T) {
	ts := newTestHTTPServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	expectedHeaders := map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"X-XSS-Protection":       "1; mode=block",
	}

	for header, expected := range expectedHeaders {
		got := resp.Header.Get(header)
		if got != expected {
			t.Errorf("header %s: erwartet %q, bekommen %q", header, expected, got)
		}
	}
}

func TestSetupGuard_BlocksWithoutSetup(t *testing.T) {
	ts := newTestHTTPServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/users")
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("erwartet 503 ohne setup, bekommen %d", resp.StatusCode)
	}
}

func TestSetupGuard_AllowsSetupRoutes(t *testing.T) {
	ts := newTestHTTPServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/setup/status")
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusServiceUnavailable {
		t.Error("/setup/status sollte nicht durch setup-guard blockiert werden")
	}
}

func TestNotFound(t *testing.T) {
	ts := newTestHTTPServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/nonexistent")
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("erwartet 404 oder 503, bekommen %d", resp.StatusCode)
	}
}
