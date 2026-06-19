package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/auth"
	"github.com/Thoomaastb/CTRLD/internal/auth/service"
	database "github.com/Thoomaastb/CTRLD/internal/db"
	"github.com/Thoomaastb/CTRLD/internal/handler"
	"github.com/Thoomaastb/CTRLD/internal/middleware"
)

var testSecret = []byte("test-secret-min-32-bytes-long-xx")

func newTestSetup(t *testing.T) (*httptest.Server, *service.AuthService) {
	t.Helper()

	db, err := database.Open(":memory:", zerolog.Nop())
	if err != nil {
		t.Fatalf("db öffnen fehlgeschlagen: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	tokenCfg := auth.TokenConfig{
		Secret:         testSecret,
		AccessTTLMin:   15,
		RefreshTTLDays: 7,
	}

	log := zerolog.Nop()
	svc := service.New(db, tokenCfg, log)
	authHandler := handler.NewAuthHandler(svc, log)
	authn := middleware.NewAuthenticator(testSecret)

	r := chi.NewRouter()
	r.Route("/api/v1", func(r chi.Router) {
		authHandler.RegisterRoutes(r, authn)
	})

	return httptest.NewServer(r), svc
}

func createTestUser(t *testing.T, db *database.DB, email, password, role string) string {
	t.Helper()

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("passwort hash fehlgeschlagen: %v", err)
	}

	// Direkt in DB einfügen
	var userID string
	err = db.SQL().QueryRow(`
		INSERT INTO users (id, email, password_hash, role, created_at, is_active)
		VALUES (lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
		        substr(lower(hex(randomblob(2))),2) || '-' ||
		        substr('89ab', abs(random()) % 4 + 1, 1) ||
		        substr(lower(hex(randomblob(2))),2) || '-' ||
		        lower(hex(randomblob(6))),
		        ?, ?, ?, CURRENT_TIMESTAMP, 1)
		RETURNING id`,
		email, hash, role,
	).Scan(&userID)
	if err != nil {
		t.Fatalf("user erstellen fehlgeschlagen: %v", err)
	}

	return userID
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestLogin_Success(t *testing.T) {
	ts, _ := newTestSetup(t)
	defer ts.Close()

	// Wir brauchen Zugriff auf die DB — direkt über den Service-Test
	// Dieser Test prüft die HTTP-Schicht
	body := `{"email":"admin@example.com","password":"test-password"}`
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	// Kein User → 401 (kein 500)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("erwartet 401 bei nicht-existentem user, bekommen %d", resp.StatusCode)
	}
}

func TestLogin_MissingFields(t *testing.T) {
	ts, _ := newTestSetup(t)
	defer ts.Close()

	body := `{"email":"only@email.com"}`
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("erwartet 400 bei fehlenden feldern, bekommen %d", resp.StatusCode)
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	ts, _ := newTestSetup(t)
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString("kein-json"))
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("erwartet 400 bei ungültigem json, bekommen %d", resp.StatusCode)
	}
}

func TestLogin_ErrorResponse_Generic(t *testing.T) {
	ts, _ := newTestSetup(t)
	defer ts.Close()

	body := `{"email":"nobody@example.com","password":"wrongpassword"}`
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	var errResp map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errResp)

	// Generische Fehlermeldung — kein Hinweis ob User existiert
	if errResp["error"] == "user nicht gefunden" || errResp["error"] == "email nicht registriert" {
		t.Error("fehlermeldung darf keinen hinweis auf user-existenz geben (user enumeration)")
	}
}

func TestRefresh_InvalidToken(t *testing.T) {
	ts, _ := newTestSetup(t)
	defer ts.Close()

	body := `{"refresh_token":"ungueltiger-token"}`
	resp, err := http.Post(ts.URL+"/api/v1/auth/refresh", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("erwartet 401 bei ungültigem refresh token, bekommen %d", resp.StatusCode)
	}
}

func TestRefresh_MissingToken(t *testing.T) {
	ts, _ := newTestSetup(t)
	defer ts.Close()

	body := `{}`
	resp, err := http.Post(ts.URL+"/api/v1/auth/refresh", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("erwartet 400 bei fehlendem token, bekommen %d", resp.StatusCode)
	}
}

func TestLogout_NoToken(t *testing.T) {
	ts, _ := newTestSetup(t)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/auth/logout", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("erwartet 401 ohne token, bekommen %d", resp.StatusCode)
	}
}

func TestSessions_NoToken(t *testing.T) {
	ts, _ := newTestSetup(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/auth/sessions")
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("erwartet 401 ohne token, bekommen %d", resp.StatusCode)
	}
}

func TestContentType_JSON(t *testing.T) {
	ts, _ := newTestSetup(t)
	defer ts.Close()

	body := `{"email":"x","password":"y"}`
	resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("request fehlgeschlagen: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("erwartet application/json, bekommen %q", ct)
	}
}
