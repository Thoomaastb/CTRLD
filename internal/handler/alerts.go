package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/alerts"
	authmw "github.com/Thoomaastb/CTRLD/internal/middleware"
)

// AlertHandler kapselt Alert-HTTP-Handler.
type AlertHandler struct {
	svc *alerts.Service
	log zerolog.Logger
}

// NewAlertHandler erstellt einen neuen AlertHandler.
func NewAlertHandler(svc *alerts.Service, log zerolog.Logger) *AlertHandler {
	return &AlertHandler{svc: svc, log: log}
}

// RegisterRoutes registriert alle Alert-Routen.
func (h *AlertHandler) RegisterRoutes(r chi.Router, authn *authmw.Authenticator) {
	r.Group(func(r chi.Router) {
		r.Use(authn.Require)
		r.Get("/alerts",              h.ListActive)
		r.Get("/alerts/history",      h.ListAll)
		r.Get("/alerts/thresholds",   h.GetThresholds)
		r.Put("/alerts/thresholds",   h.UpdateThresholds)
	})
}

// ListActive GET /api/v1/alerts
func (h *AlertHandler) ListActive(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListActive(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("alerts laden fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}
	if items == nil {
		items = []alerts.Alert{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": items,
		"count":  len(items),
	})
}

// ListAll GET /api/v1/alerts/history
func (h *AlertHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListAll(r.Context(), 100)
	if err != nil {
		h.log.Error().Err(err).Msg("alert-history laden fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}
	if items == nil {
		items = []alerts.Alert{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": items,
		"count":  len(items),
	})
}

// GetThresholds GET /api/v1/alerts/thresholds
func (h *AlertHandler) GetThresholds(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.svc.GetThresholds())
}

// UpdateThresholds PUT /api/v1/alerts/thresholds
func (h *AlertHandler) UpdateThresholds(w http.ResponseWriter, r *http.Request) {
	claims := authmw.ClaimsFromContext(r.Context())
	if claims == nil || claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "nur admins können schwellwerte ändern", "FORBIDDEN")
		return
	}

	var thresholds []alerts.Threshold
	if err := json.NewDecoder(r.Body).Decode(&thresholds); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	// Validierung
	for _, t := range thresholds {
		if t.Warning >= t.Critical {
			writeError(w, http.StatusBadRequest,
				"warning-schwellwert muss kleiner als critical sein", "INVALID_THRESHOLD")
			return
		}
		if t.Warning < 0 || t.Critical > 100 {
			writeError(w, http.StatusBadRequest,
				"schwellwerte müssen zwischen 0 und 100 liegen", "INVALID_THRESHOLD")
			return
		}
	}

	h.svc.UpdateThresholds(thresholds)
	h.log.Info().Str("user_id", claims.UserID).Msg("alert-schwellwerte aktualisiert")
	writeJSON(w, http.StatusOK, h.svc.GetThresholds())
}
