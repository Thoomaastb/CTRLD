package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	authmw "github.com/Thoomaastb/CTRLD/internal/middleware"
	"github.com/Thoomaastb/CTRLD/internal/tls"
	"github.com/Thoomaastb/CTRLD/internal/updater"
)

// SystemHandler kapselt System-Settings HTTP-Handler.
type SystemHandler struct {
	tlsMgr  *tls.Manager
	updater *updater.Checker
	log     zerolog.Logger
}

// NewSystemHandler erstellt einen neuen SystemHandler.
func NewSystemHandler(tlsMgr *tls.Manager, u *updater.Checker, log zerolog.Logger) *SystemHandler {
	return &SystemHandler{tlsMgr: tlsMgr, updater: u, log: log}
}

// RegisterRoutes registriert alle System-Routen.
func (h *SystemHandler) RegisterRoutes(r chi.Router, authn *authmw.Authenticator) {
	r.Group(func(r chi.Router) {
		r.Use(authn.Require)

		// Nur Admins
		r.Get("/system/update-status", h.GetUpdateStatus)
		r.Get("/system/tls", h.GetTLSInfo)
		r.Post("/system/tls/generate", h.GenerateSelfSigned)
	})
}

// GetUpdateStatus GET /api/v1/system/update-status
func (h *SystemHandler) GetUpdateStatus(w http.ResponseWriter, r *http.Request) {
	claims := authmw.ClaimsFromContext(r.Context())
	if claims == nil || claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "nur admins", "FORBIDDEN")
		return
	}
	writeJSON(w, http.StatusOK, h.updater.Status())
}

// GetTLSInfo GET /api/v1/system/tls
func (h *SystemHandler) GetTLSInfo(w http.ResponseWriter, r *http.Request) {
	claims := authmw.ClaimsFromContext(r.Context())
	if claims == nil || claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "nur admins", "FORBIDDEN")
		return
	}

	if !h.tlsMgr.HasCertificate() {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"has_certificate": false,
		})
		return
	}

	info, err := h.tlsMgr.GetCertInfo()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "zertifikat laden fehlgeschlagen", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"has_certificate": true,
		"cert":            info,
	})
}

// GenerateSelfSigned POST /api/v1/system/tls/generate
// Body: {"hosts": ["example.com", "192.168.1.1"], "valid_days": 365}
func (h *SystemHandler) GenerateSelfSigned(w http.ResponseWriter, r *http.Request) {
	claims := authmw.ClaimsFromContext(r.Context())
	if claims == nil || claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "nur admins", "FORBIDDEN")
		return
	}

	var req struct {
		Hosts     []string `json:"hosts"`
		ValidDays int      `json:"valid_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	if len(req.Hosts) == 0 {
		writeError(w, http.StatusBadRequest, "mindestens ein host erforderlich", "INVALID_REQUEST")
		return
	}
	for _, host := range req.Hosts {
		if strings.TrimSpace(host) == "" {
			writeError(w, http.StatusBadRequest, "leere hosts nicht erlaubt", "INVALID_REQUEST")
			return
		}
	}
	if req.ValidDays <= 0 {
		req.ValidDays = 365
	}
	if req.ValidDays > 3650 {
		req.ValidDays = 3650
	}

	if err := h.tlsMgr.GenerateSelfSigned(req.Hosts, req.ValidDays); err != nil {
		h.log.Error().Err(err).Msg("tls generieren fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "zertifikat generieren fehlgeschlagen", "INTERNAL_ERROR")
		return
	}

	info, err := h.tlsMgr.GetCertInfo()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "zertifikat laden fehlgeschlagen", "INTERNAL_ERROR")
		return
	}

	h.log.Info().
		Strs("hosts", req.Hosts).
		Int("valid_days", req.ValidDays).
		Str("user_id", claims.UserID).
		Msg("self-signed tls zertifikat generiert")

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":  "Zertifikat erfolgreich generiert",
		"cert":     info,
		"cert_file": h.tlsMgr.CertFile(),
		"key_file":  h.tlsMgr.KeyFile(),
		"restart_required": true,
	})
}
