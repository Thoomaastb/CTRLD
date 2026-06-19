package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/Thoomaastab/CTRLD/internal/config"
	database "github.com/Thoomaastab/CTRLD/internal/db"
	"github.com/Thoomaastab/CTRLD/internal/server"
)

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Host:            "127.0.0.1",
			Port:            8443,
			ReadTimeoutSec:  10,
			WriteTimeoutSec: 30,
			IdleTimeoutSec:  120,
		},
		Log: config.LogConfig{
			Level:  "error",
			Format: "json",
		},
		Security: config.SecurityConfig{
			JWTSecret:        "test-secret-min-32-bytes-long-xx",
			ArgonMemory:      65536,
			ArgonIterations:  3,
			ArgonParallelism: 2,
			JWTAccessTTLMin:  15,
			JWTRefreshTTLDay: 7,
		},
		Database: config.DatabaseConfig{
			Path: ":memory:",
		},
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	db, err := database.Open(":memory:", zerolog.Nop())
	if err != nil {
		t.Fatalf("db öffnen fehlgeschlagen: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := testConfig()
	logger := zerolog.Nop()
	srv := server.New(cfg, db, logger)
	return httptest.NewServer(srv.Handler())
}

func TestHealthEndpoint(t *testing.T) {
	ts := newTestServer(t)
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
	ts := newTestServer(t)
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
	ts := newTestServer(t)
	defer ts.Close()

	// Ohne abgeschlossenen Setup sollte /api/v1/users blockiert sein
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
	ts := newTestServer(t)
	defer ts.Close()

	// /setup/* sollte immer erreichbar sein
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
	ts := newTestServer(t)
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
