package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	authmw "github.com/Thoomaastb/CTRLD/internal/middleware"
	"github.com/Thoomaastb/CTRLD/internal/pim"
	"github.com/Thoomaastb/CTRLD/internal/services"
)

// ServicesHandler kapselt Service-Management HTTP-Handler.
type ServicesHandler struct {
	pimSvc *pim.Service
	log    zerolog.Logger
}

// NewServicesHandler erstellt einen neuen ServicesHandler.
func NewServicesHandler(pimSvc *pim.Service, log zerolog.Logger) *ServicesHandler {
	return &ServicesHandler{pimSvc: pimSvc, log: log}
}

// RegisterRoutes registriert alle Service-Routen.
func (h *ServicesHandler) RegisterRoutes(r chi.Router, authn *authmw.Authenticator) {
	r.Group(func(r chi.Router) {
		r.Use(authn.Require)

		r.Get("/services", h.ListServices)
		r.Get("/services/{name}", h.GetService)
		r.Post("/services/{name}/action", h.ExecuteAction)
		r.Get("/services/{name}/logs", h.GetServiceLogs)
	})
}

// ListServices GET /api/v1/services
func (h *ServicesHandler) ListServices(w http.ResponseWriter, r *http.Request) {
	list, err := services.List(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("services laden fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "services konnten nicht geladen werden", "INTERNAL_ERROR")
		return
	}

	if list == nil {
		list = []services.Service{}
	}

	// Filter: ?state=active|inactive|failed
	if state := r.URL.Query().Get("state"); state != "" {
		filtered := list[:0]
		for _, s := range list {
			if s.ActiveState == state {
				filtered = append(filtered, s)
			}
		}
		list = filtered
	}

	// Filter: ?search=nginx
	if search := r.URL.Query().Get("search"); search != "" {
		filtered := list[:0]
		for _, s := range list {
			if contains(s.Name, search) || contains(s.Description, search) {
				filtered = append(filtered, s)
			}
		}
		list = filtered
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"services": list,
		"count":    len(list),
	})
}

// GetService GET /api/v1/services/{name}
func (h *ServicesHandler) GetService(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	svc, err := services.Get(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "service nicht gefunden", "NOT_FOUND")
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

// ExecuteAction POST /api/v1/services/{name}/action
// Body: {"action": "start|stop|restart|enable|disable|reload"}
// PIM-Pflicht: stop, disable, restart erfordern aktive PIM-Sitzung
func (h *ServicesHandler) ExecuteAction(w http.ResponseWriter, r *http.Request) {
	claims := authmw.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	name := chi.URLParam(r, "name")

	var req struct {
		Action string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "ungültiger request body", "INVALID_REQUEST")
		return
	}

	action := services.Action(req.Action)

	// PIM-Pflicht für destruktive Aktionen
	requiresPIM := action == services.ActionStop ||
		action == services.ActionDisable ||
		action == services.ActionRestart

	var pimID string
	if requiresPIM {
		var err error
		pimID, err = h.pimSvc.CheckAndRecord(r.Context(), claims.UserID)
		if err != nil {
			writeError(w, http.StatusForbidden,
				"aktive pim-sitzung erforderlich für "+req.Action,
				"PIM_REQUIRED")
			return
		}
	}

	if err := services.Execute(r.Context(), name, action); err != nil {
		h.log.Error().Err(err).Str("service", name).Str("action", req.Action).Msg("service aktion fehlgeschlagen")
		writeError(w, http.StatusBadRequest, err.Error(), "ACTION_FAILED")
		return
	}

	h.log.Info().
		Str("service", name).
		Str("action", req.Action).
		Str("user_id", claims.UserID).
		Str("pim_id", pimID).
		Msg("service aktion ausgeführt")

	writeJSON(w, http.StatusOK, map[string]string{
		"service": name,
		"action":  req.Action,
		"status":  "ok",
	})
}

// GetServiceLogs GET /api/v1/services/{name}/logs?lines=50
func (h *ServicesHandler) GetServiceLogs(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	lines := 50
	if l := r.URL.Query().Get("lines"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			lines = n
		}
	}

	logLines, err := services.GetLogs(r.Context(), name, lines)
	if err != nil {
		h.log.Error().Err(err).Str("service", name).Msg("service logs fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "logs konnten nicht geladen werden", "INTERNAL_ERROR")
		return
	}

	if logLines == nil {
		logLines = []string{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"service": name,
		"lines":   logLines,
		"count":   len(logLines),
	})
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > 0 && containsLower(s, substr)))
}

func containsLower(s, substr string) bool {
	sl := len(s)
	subl := len(substr)
	if subl > sl {
		return false
	}
	subLower := toLower(substr)
	for i := 0; i <= sl-subl; i++ {
		if toLower(s[i:i+subl]) == subLower {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}
