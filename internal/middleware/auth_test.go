package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Thoomaastb/CTRLD/internal/auth"
	"github.com/Thoomaastb/CTRLD/internal/middleware"
)

var testSecret = []byte("test-secret-min-32-bytes-long-xx")

func newTokenForTest(t *testing.T, role string) string {
	t.Helper()
	pair, err := auth.IssueTokenPair(
		auth.TokenConfig{Secret: testSecret, AccessTTLMin: 15},
		"user-1", "test@example.com", role, "session-1",
	)
	if err != nil {
		t.Fatalf("token erstellen fehlgeschlagen: %v", err)
	}
	return pair.AccessToken
}

func TestRequire_NoToken(t *testing.T) {
	a := middleware.NewAuthenticator(testSecret)
	handler := a.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("erwartet 401, bekommen %d", rec.Code)
	}
}

func TestRequire_ValidToken(t *testing.T) {
	a := middleware.NewAuthenticator(testSecret)
	token := newTokenForTest(t, "admin")

	handler := a.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.ClaimsFromContext(r.Context())
		if claims == nil {
			t.Error("claims sollten im context sein")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("erwartet 200, bekommen %d", rec.Code)
	}
}

func TestRequire_InvalidToken(t *testing.T) {
	a := middleware.NewAuthenticator(testSecret)

	handler := a.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer ungueltig.token.hier")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("erwartet 401, bekommen %d", rec.Code)
	}
}

func TestRequireAdmin_ViewerRole(t *testing.T) {
	a := middleware.NewAuthenticator(testSecret)
	token := newTokenForTest(t, "viewer")

	handler := a.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("viewer sollte 403 bekommen, bekommen %d", rec.Code)
	}
}

func TestRequireAdmin_AdminRole(t *testing.T) {
	a := middleware.NewAuthenticator(testSecret)
	token := newTokenForTest(t, "admin")

	handler := a.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("admin sollte 200 bekommen, bekommen %d", rec.Code)
	}
}
