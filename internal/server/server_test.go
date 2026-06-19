package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/config"
	"github.com/Thoomaastb/CTRLD/internal/server"
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
			Level:  "error", // Tests ruhig halten
			Format: "json",
		},
		Security: config.SecurityConfig{
			ArgonMemory:      65536,
			ArgonIterations:  3,
			ArgonParallelism: 2,
			JWTAccessTTLMin:  15,
			JWTRefreshTTLDay: 7,
		},
		Database: config.DatabaseConfig{
			Path: "/tmp/ctrld_test.db",
		},
	}
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := testConfig()
	logger := zerolog.Nop()
	srv := server.New(cfg, logger)
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
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
		"X-XSS-Protection":          "1; mode=block",
	}

	for header, expected := range expectedHeaders {
		got := resp.Header.Get(header)
		if got != expected {
			t.Errorf("header %s: erwartet %q, bekommen %q", header, expected, got)
		}
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

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("erwartet 404, bekommen %d", resp.StatusCode)
	}
}
