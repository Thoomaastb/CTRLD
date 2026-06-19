package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/audit"
	"github.com/Thoomaastb/CTRLD/internal/middleware"
	"github.com/Thoomaastb/CTRLD/internal/pim"
)

// PIMHandler kapselt PIM + Audit HTTP-Handler.
type PIMHandler struct {
	pimSvc   *pim.Service
	auditSvc *audit.Service
	log      zerolog.Logger
}

// NewPIMHandler erstellt einen neuen PIMHandler.
func NewPIMHandler(pimSvc *pim.Service, auditSvc *audit.Service, log zerolog.Logger) *PIMHandler {
	return &PIMHandler{pimSvc: pimSvc, auditSvc: auditSvc, log: log}
}

// RegisterRoutes registriert alle PIM + Audit-Routen.
func (h *PIMHandler) RegisterRoutes(r chi.Router, authn *middleware.Authenticator) {
	r.Group(func(r chi.Router) {
		r.Use(authn.Require)

		// PIM
		r.Post("/pim/request", h.RequestPIM)
		r.Get("/pim/active", h.GetActivePIM)
		r.Delete("/pim/active", h.EndPIM)
		r.Post("/pim/break-glass", h.BreakGlass)

		// Audit
		r.Get("/audit", h.ListAudit)
		r.Get("/audit/export", h.ExportAudit)
	})
}

// ── Request/Response-Typen ────────────────────────────────────────────────────

type pimRequestBody struct {
	Reason      string `json:"reason"`
	DurationMin int    `json:"duration_min"`
}

type breakGlassBody struct {
	Reason string `json:"reason"`
}

type pimResponse struct {
	ID                   string `json:"id"`
	Reason               string `json:"reason"`
	RequestedDurationMin int    `json:"requested_duration_min"`
	StartedAt            string `json:"started_at"`
	ExpiresAt            string `json:"expires_at"`
	IsBreakGlass         bool   `json:"is_break_glass"`
	ActionCount          int    `json:"action_count"`
	RemainingSeconds     int    `json:"remaining_seconds"`
}

// ── PIM Handler ───────────────────────────────────────────────────────────────

// RequestPIM POST /api/v1/pim/request
func (h *PIMHandler) RequestPIM(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	// Nur Admins dürfen PIM anfordern
	if claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "nur admins können pim anfordern", "FORBIDDEN")
		return
	}

	var req pimRequestBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	if req.DurationMin == 0 {
		req.DurationMin = 30 // Default
	}

	session, err := h.pimSvc.Request(r.Context(), pim.RequestParams{
		UserID:      claims.UserID,
		SessionID:   claims.SessionID,
		Reason:      req.Reason,
		DurationMin: req.DurationMin,
	})

	if err != nil {
		switch {
		case errors.Is(err, pim.ErrPIMAlreadyActive):
			writeError(w, http.StatusConflict, "bereits eine aktive pim-sitzung vorhanden", "PIM_ALREADY_ACTIVE")
		case errors.Is(err, pim.ErrReasonTooShort):
			writeError(w, http.StatusBadRequest, "begründung muss mindestens 10 zeichen haben", "REASON_TOO_SHORT")
		case errors.Is(err, pim.ErrInvalidDuration):
			writeError(w, http.StatusBadRequest, "ungültige dauer — erlaubt: 15, 30, 60 oder 1-480 minuten", "INVALID_DURATION")
		default:
			h.log.Error().Err(err).Msg("pim request fehlgeschlagen")
			writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		}
		return
	}

	writeJSON(w, http.StatusCreated, toPIMResponse(session))
}

// GetActivePIM GET /api/v1/pim/active
func (h *PIMHandler) GetActivePIM(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	session, err := h.pimSvc.GetActive(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, pim.ErrPIMNotActive) {
			writeJSON(w, http.StatusOK, map[string]interface{}{"active": false})
			return
		}
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	resp := toPIMResponse(session)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"active":  true,
		"session": resp,
	})
}

// EndPIM DELETE /api/v1/pim/active
func (h *PIMHandler) EndPIM(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	// Aktive Session holen um ID zu bekommen
	active, err := h.pimSvc.GetActive(r.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, pim.ErrPIMNotActive) {
			writeError(w, http.StatusNotFound, "keine aktive pim-sitzung", "PIM_NOT_ACTIVE")
			return
		}
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	if err := h.pimSvc.End(r.Context(), claims.UserID, claims.SessionID, active.ID); err != nil {
		h.log.Error().Err(err).Msg("pim beenden fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// BreakGlass POST /api/v1/pim/break-glass
func (h *PIMHandler) BreakGlass(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	if claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "nur admins können break-glass aktivieren", "FORBIDDEN")
		return
	}

	var req breakGlassBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	session, err := h.pimSvc.Request(r.Context(), pim.RequestParams{
		UserID:       claims.UserID,
		SessionID:    claims.SessionID,
		Reason:       req.Reason,
		DurationMin:  60,
		IsBreakGlass: true,
	})

	if err != nil {
		if errors.Is(err, pim.ErrPIMAlreadyActive) {
			writeError(w, http.StatusConflict, "bereits eine aktive pim-sitzung vorhanden", "PIM_ALREADY_ACTIVE")
			return
		}
		if errors.Is(err, pim.ErrReasonTooShort) {
			writeError(w, http.StatusBadRequest, "begründung muss mindestens 10 zeichen haben", "REASON_TOO_SHORT")
			return
		}
		h.log.Error().Err(err).Msg("break-glass fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	// Break-Glass: immer 201 mit kritischem Hinweis
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"session":  toPIMResponse(session),
		"warning":  "BREAK-GLASS aktiviert — alle Aktionen werden mit erhöhter Priorität protokolliert",
		"severity": "critical",
	})
}

// ── Audit Handler ─────────────────────────────────────────────────────────────

// ListAudit GET /api/v1/audit
func (h *PIMHandler) ListAudit(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	perPage, _ := strconv.Atoi(q.Get("per_page"))

	params := audit.QueryParams{
		Severity:   q.Get("severity"),
		ActionType: q.Get("action_type"),
		Page:       page,
		PerPage:    perPage,
	}

	// Viewer sieht nur eigene Einträge
	if claims.Role != "admin" {
		params.UserID = claims.UserID
	}

	result, err := h.auditSvc.Query(r.Context(), params)
	if err != nil {
		h.log.Error().Err(err).Msg("audit abfrage fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "interner fehler", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ExportAudit GET /api/v1/audit/export?format=csv|json
func (h *PIMHandler) ExportAudit(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	// Nur Admins dürfen exportieren
	if claims.Role != "admin" {
		writeError(w, http.StatusForbidden, "nur admins können exportieren", "FORBIDDEN")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	params := audit.QueryParams{Page: 1, PerPage: 200}

	switch format {
	case "csv":
		data, err := h.auditSvc.ExportCSV(r.Context(), params)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export fehlgeschlagen", "INTERNAL_ERROR")
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", "attachment; filename=audit-log.csv")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(data))

	default: // json
		data, err := h.auditSvc.ExportJSON(r.Context(), params)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export fehlgeschlagen", "INTERNAL_ERROR")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=audit-log.json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(data))
	}
}

// ── Hilfsfunktionen ───────────────────────────────────────────────────────────

func toPIMResponse(s *pim.ActivePIMSession) pimResponse {
	return pimResponse{
		ID:                   s.ID,
		Reason:               s.Reason,
		RequestedDurationMin: s.RequestedDurationMin,
		StartedAt:            s.StartedAt.Format("2006-01-02T15:04:05Z"),
		ExpiresAt:            s.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		IsBreakGlass:         s.IsBreakGlass,
		ActionCount:          s.ActionCount,
		RemainingSeconds:     s.RemainingSeconds,
	}
}
