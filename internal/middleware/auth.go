package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/Thoomaastb/CTRLD/internal/auth"
)

type contextKey string

const (
	ClaimsKey contextKey = "claims"
)

// Authenticator hält die Konfiguration für die Auth-Middleware.
type Authenticator struct {
	secret []byte
}

// NewAuthenticator erstellt eine neue Auth-Middleware.
func NewAuthenticator(secret []byte) *Authenticator {
	return &Authenticator{secret: secret}
}

// Require ist eine Middleware die einen gültigen JWT-Token erfordert.
// Legt die Claims im Context ab für nachfolgende Handler.
func (a *Authenticator) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := extractBearerToken(r)
		if err != nil {
			writeUnauthorized(w, "token fehlt")
			return
		}

		claims, err := auth.ValidateAccessToken(token, a.secret)
		if err != nil {
			if errors.Is(err, auth.ErrTokenExpired) {
				writeUnauthorized(w, "token abgelaufen")
				return
			}
			writeUnauthorized(w, "token ungültig")
			return
		}

		// Claims in Context legen
		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin erweitert Require um eine Admin-Rollen-Prüfung.
func (a *Authenticator) RequireAdmin(next http.Handler) http.Handler {
	return a.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := ClaimsFromContext(r.Context())
		if claims == nil || claims.Role != "admin" {
			writeForbidden(w, "admin-rechte erforderlich")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

// ClaimsFromContext extrahiert die JWT-Claims aus dem Context.
// Gibt nil zurück wenn keine Claims vorhanden.
func ClaimsFromContext(ctx context.Context) *auth.Claims {
	claims, _ := ctx.Value(ClaimsKey).(*auth.Claims)
	return claims
}

// extractBearerToken liest den Bearer-Token aus dem Authorization-Header.
func extractBearerToken(r *http.Request) (string, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", auth.ErrTokenMissing
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", auth.ErrTokenInvalid
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", auth.ErrTokenMissing
	}

	return token, nil
}

// writeUnauthorized schreibt eine 401-Antwort.
func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"` + msg + `","code":"UNAUTHORIZED"}`))
}

// writeForbidden schreibt eine 403-Antwort.
func writeForbidden(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"error":"` + msg + `","code":"FORBIDDEN"}`))
}
