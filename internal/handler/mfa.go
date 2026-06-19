package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/auth"
	"github.com/Thoomaastb/CTRLD/internal/auth/service"
	"github.com/Thoomaastb/CTRLD/internal/mfa"
	"github.com/Thoomaastb/CTRLD/internal/middleware"
)

// MFAHandler kapselt alle MFA-HTTP-Handler.
type MFAHandler struct {
	mfaSvc  *mfa.Service
	authSvc *service.AuthService
	log     zerolog.Logger
}

// NewMFAHandler erstellt einen neuen MFAHandler.
func NewMFAHandler(mfaSvc *mfa.Service, authSvc *service.AuthService, log zerolog.Logger) *MFAHandler {
	return &MFAHandler{mfaSvc: mfaSvc, authSvc: authSvc, log: log}
}

// RegisterRoutes registriert alle MFA-Routen.
func (h *MFAHandler) RegisterRoutes(r chi.Router, authn *middleware.Authenticator) {
	r.Route("/auth/mfa", func(r chi.Router) {
		// MFA-Verifikation im Login-Flow (kein vollwertiger JWT — MFA-Pending-Token)
		r.Post("/verify", h.VerifyMFA)

		// Geschützte Endpunkte
		r.Group(func(r chi.Router) {
			r.Use(authn.Require)
			r.Get("/credentials", h.ListCredentials)
			r.Post("/credentials/totp/initiate", h.InitiateTOTPSetup)
			r.Post("/credentials/totp/confirm", h.ConfirmTOTPSetup)
			r.Delete("/credentials/{id}", h.RemoveCredential)
		})
	})
}

// ── Request/Response-Typen ────────────────────────────────────────────────────

type verifyMFARequest struct {
	MFASessionToken string `json:"mfa_session_token"`
	Code            string `json:"code"`
	// Type: "totp" oder "backup_code"
	Type string `json:"type"`
}

type confirmTOTPRequest struct {
	Secret     string `json:"secret"`
	Code       string `json:"code"`
	DeviceName string `json:"device_name"`
}

type credentialResponse struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	CreatedAt   string `json:"created_at"`
	LastUsedAt  string `json:"last_used_at,omitempty"`
}

// ── Handler ───────────────────────────────────────────────────────────────────

// VerifyMFA POST /api/v1/auth/mfa/verify
// Verifiziert den MFA-Code nach dem ersten Login-Schritt.
func (h *MFAHandler) VerifyMFA(w http.ResponseWriter, r *http.Request) {
	var req verifyMFARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	if req.MFASessionToken == "" || req.Code == "" {
		writeError(w, http.StatusBadRequest, "mfa_session_token und code erforderlich", "MISSING_FIELDS")
		return
	}

	// MFA-Session-Token validieren (kurzlebiger Token aus Login-Schritt 1)
	claims, err := auth.ValidateAccessToken(req.MFASessionToken, h.mfaSvc.TokenSecret())
	if err != nil {
		writeError(w, http.StatusUnauthorized, "ungültiger mfa session token", "INVALID_TOKEN")
		return
	}

	if claims.Role != "mfa_pending" {
		writeError(w, http.StatusUnauthorized, "ungültiger token typ", "INVALID_TOKEN")
		return
	}

	// MFA verifizieren
	var userClaims *auth.Claims
	switch req.Type {
	case "backup_code":
		userClaims, err = h.mfaSvc.UseBackupCodeForLogin(r.Context(), claims.UserID, req.Code)
	default: // "totp" ist Standard
		userClaims, err = h.mfaSvc.VerifyTOTPForLogin(
			r.Context(), claims.UserID, req.Code, realIP(r), r.UserAgent(),
		)
	}

	if err != nil {
		if errors.Is(err, mfa.ErrInvalidTOTPCode) || errors.Is(err, mfa.ErrInvalidBackupCode) {
			writeError(w, http.StatusUnauthorized, "ungültiger code", "INVALID_MFA_CODE")
			return
		}
		h.log.Error().Err(err).Msg("mfa verify interner fehler")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	// Vollwertige Session erstellen
	result, err := h.authSvc.CreateSessionAfterMFA(
		r.Context(),
		userClaims.UserID, userClaims.Email, userClaims.Role,
		realIP(r), r.UserAgent(),
	)
	if err != nil {
		h.log.Error().Err(err).Msg("session nach mfa erstellen fehlgeschlagen")
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

// InitiateTOTPSetup POST /api/v1/auth/mfa/credentials/totp/initiate
func (h *MFAHandler) InitiateTOTPSetup(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	setup, err := h.mfaSvc.InitiateTOTPSetup(r.Context(), claims.UserID)
	if err != nil {
		h.log.Error().Err(err).Msg("totp setup initieren fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, setup)
}

// ConfirmTOTPSetup POST /api/v1/auth/mfa/credentials/totp/confirm
func (h *MFAHandler) ConfirmTOTPSetup(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	var req confirmTOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	if req.Secret == "" || req.Code == "" {
		writeError(w, http.StatusBadRequest, "secret und code erforderlich", "MISSING_FIELDS")
		return
	}

	if req.DeviceName == "" {
		req.DeviceName = "Authenticator App"
	}

	result, err := h.mfaSvc.ConfirmTOTPSetup(r.Context(), mfa.ConfirmTOTPSetupParams{
		UserID:     claims.UserID,
		Secret:     req.Secret,
		Code:       req.Code,
		DeviceName: req.DeviceName,
	})
	if err != nil {
		if errors.Is(err, mfa.ErrVerificationFailed) {
			writeError(w, http.StatusUnauthorized, "ungültiger bestätigungscode", "INVALID_CODE")
			return
		}
		h.log.Error().Err(err).Msg("totp setup bestätigen fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"credential_id": result.CredentialID,
		"backup_codes":  result.BackupCodes,
		"message":       "TOTP erfolgreich eingerichtet. Backup-Codes sicher aufbewahren — sie werden nicht erneut angezeigt.",
	})
}

// ListCredentials GET /api/v1/auth/mfa/credentials
func (h *MFAHandler) ListCredentials(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	creds, err := h.mfaSvc.ListCredentials(r.Context(), claims.UserID)
	if err != nil {
		h.log.Error().Err(err).Msg("credentials laden fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	resp := make([]credentialResponse, 0, len(creds))
	for _, c := range creds {
		cr := credentialResponse{
			ID:        c.ID,
			Type:      c.Type,
			Name:      c.Name,
			CreatedAt: c.CreatedAt,
		}
		if c.LastUsedAt.Valid {
			cr.LastUsedAt = c.LastUsedAt.String
		}
		resp = append(resp, cr)
	}

	writeJSON(w, http.StatusOK, resp)
}

// RemoveCredential DELETE /api/v1/auth/mfa/credentials/{id}
func (h *MFAHandler) RemoveCredential(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	credID := chi.URLParam(r, "id")
	if credID == "" {
		writeError(w, http.StatusBadRequest, "credential id fehlt", "MISSING_PARAM")
		return
	}

	if err := h.mfaSvc.RemoveCredential(r.Context(), credID, claims.UserID); err != nil {
		h.log.Error().Err(err).Msg("credential entfernen fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
