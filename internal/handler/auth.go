package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/auth"
	"github.com/Thoomaastb/CTRLD/internal/auth/service"
	"github.com/Thoomaastb/CTRLD/internal/middleware"
	"github.com/Thoomaastb/CTRLD/internal/ratelimit"
)

// AuthHandler kapselt alle Auth-HTTP-Handler.
type AuthHandler struct {
	svc *service.AuthService
	log zerolog.Logger
}

// NewAuthHandler erstellt einen neuen AuthHandler.
func NewAuthHandler(svc *service.AuthService, log zerolog.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, log: log}
}

// RegisterRoutes registriert alle Auth-Routen auf dem Router.
func (h *AuthHandler) RegisterRoutes(r chi.Router, authn *middleware.Authenticator) {
	r.Route("/auth", func(r chi.Router) {
		// Öffentliche Endpunkte (kein JWT)
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)

		// Geschützte Endpunkte (JWT erforderlich)
		r.Group(func(r chi.Router) {
			r.Use(authn.Require)
			r.Post("/logout", h.Logout)
			r.Get("/sessions", h.ListSessions)
			r.Delete("/sessions/{id}", h.RevokeSession)
		})
	})
}

// ── Request/Response-Typen ────────────────────────────────────────────────────

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	Role             string    `json:"role"`
	// MFA-Flow
	MFARequired     bool   `json:"mfa_required,omitempty"`
	MFASessionToken string `json:"mfa_session_token,omitempty"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type sessionResponse struct {
	ID        string `json:"id"`
	IPAddress string `json:"ip_address"`
	UserAgent string `json:"user_agent,omitempty"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

// ── Handler ───────────────────────────────────────────────────────────────────

// Login POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	// Basis-Validierung
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email und passwort erforderlich", "MISSING_FIELDS")
		return
	}

	result, err := h.svc.Login(r.Context(), service.LoginRequest{
		Email:     strings.ToLower(strings.TrimSpace(req.Email)),
		Password:  req.Password,
		IPAddress: realIP(r),
		UserAgent: r.UserAgent(),
	})

	if err != nil {
		switch {
		case errors.Is(err, service.ErrRateLimited):
			writeError(w, http.StatusTooManyRequests, "zu viele fehlversuche — bitte warten", "RATE_LIMITED")
		case errors.Is(err, service.ErrInvalidCredentials),
			errors.Is(err, service.ErrAccountInactive):
			// Generische Fehlermeldung — kein User-Enumeration
			writeError(w, http.StatusUnauthorized, "ungültige anmeldedaten", "INVALID_CREDENTIALS")
		default:
			h.log.Error().Err(err).Msg("login interner fehler")
			writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		}
		return
	}

	// MFA erforderlich
	if result.MFARequired {
		writeJSON(w, http.StatusOK, loginResponse{
			MFARequired:     true,
			MFASessionToken: result.MFASessionToken,
		})
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken:      result.AccessToken,
		RefreshToken:     result.RefreshToken,
		AccessExpiresAt:  result.AccessExpiresAt,
		RefreshExpiresAt: result.RefreshExpiresAt,
		Role:             result.Role,
	})
}

// Logout POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	token := extractBearerToken(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "token fehlt", "MISSING_TOKEN")
		return
	}

	if err := h.svc.Logout(r.Context(), token, realIP(r)); err != nil {
		h.log.Error().Err(err).Msg("logout fehler")
		writeError(w, http.StatusInternalServerError, "logout fehlgeschlagen", "INTERNAL_ERROR")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Refresh POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token erforderlich", "INVALID_REQUEST")
		return
	}

	result, err := h.svc.Refresh(r.Context(), service.RefreshRequest{
		RefreshToken: req.RefreshToken,
		IPAddress:    realIP(r),
		UserAgent:    r.UserAgent(),
	})

	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			writeError(w, http.StatusUnauthorized, "ungültiger oder abgelaufener token", "INVALID_TOKEN")
			return
		}
		h.log.Error().Err(err).Msg("refresh interner fehler")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken:      result.AccessToken,
		RefreshToken:     result.RefreshToken,
		AccessExpiresAt:  result.AccessExpiresAt,
		RefreshExpiresAt: result.RefreshExpiresAt,
		Role:             result.Role,
	})
}

// ListSessions GET /api/v1/auth/sessions
func (h *AuthHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	sessions, err := h.svc.ListSessions(r.Context(), claims.UserID)
	if err != nil {
		h.log.Error().Err(err).Msg("sessions laden fehler")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	resp := make([]sessionResponse, 0, len(sessions))
	for _, s := range sessions {
		sr := sessionResponse{
			ID:        s.ID,
			IPAddress: s.IpAddress,
			CreatedAt: s.CreatedAt,
			ExpiresAt: s.ExpiresAt,
		}
		if s.UserAgent.Valid {
			sr.UserAgent = s.UserAgent.String
		}
		resp = append(resp, sr)
	}

	writeJSON(w, http.StatusOK, resp)
}

// RevokeSession DELETE /api/v1/auth/sessions/{id}
func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session id fehlt", "MISSING_PARAM")
		return
	}

	err := h.svc.RevokeSession(r.Context(), sessionID, claims.UserID, realIP(r))
	if err != nil {
		if errors.Is(err, service.ErrSessionNotFound) {
			writeError(w, http.StatusNotFound, "session nicht gefunden", "NOT_FOUND")
			return
		}
		h.log.Error().Err(err).Msg("session widerruf fehler")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ── Hilfsfunktionen ───────────────────────────────────────────────────────────

type errorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code"`
	RequestID string `json:"request_id,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, errorResponse{Error: msg, Code: code})
}

func extractBearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	// RemoteAddr ohne Port
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i != -1 {
		return addr[:i]
	}
	return addr
}

// Sicherstellen dass auth.ErrTokenMissing und ErrTokenInvalid importiert werden
var _ = auth.ErrTokenMissing
var _ = ratelimit.MaxAttemptsShort
