package handler

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/Thoomaastab/CTRLD/internal/metrics"
	authmw "github.com/Thoomaastab/CTRLD/internal/middleware"
	"github.com/Thoomaastab/CTRLD/internal/pim"
)

// MetricsHandler kapselt Monitoring HTTP-Handler.
type MetricsHandler struct {
	metricsSvc *metrics.Service
	pimSvc     *pim.Service
	log        zerolog.Logger
}

// NewMetricsHandler erstellt einen neuen MetricsHandler.
func NewMetricsHandler(metricsSvc *metrics.Service, pimSvc *pim.Service, log zerolog.Logger) *MetricsHandler {
	return &MetricsHandler{
		metricsSvc: metricsSvc,
		pimSvc:     pimSvc,
		log:        log,
	}
}

// RegisterRoutes registriert alle Monitoring-Routen.
func (h *MetricsHandler) RegisterRoutes(r chi.Router, authn *authmw.Authenticator) {
	r.Group(func(r chi.Router) {
		r.Use(authn.Require)

		r.Get("/system/metrics", h.GetMetrics)
		r.Get("/system/metrics/history", h.GetHistory)
		r.Get("/system/processes", h.GetProcesses)
		r.Delete("/system/processes/{pid}", h.KillProcess)
	})
}

// GetMetrics GET /api/v1/system/metrics
// Gibt den aktuellen Metriken-Snapshot zurück.
func (h *MetricsHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	snap := h.metricsSvc.Latest()
	if snap == nil {
		writeError(w, http.StatusServiceUnavailable, "metriken noch nicht verfügbar", "METRICS_UNAVAILABLE")
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// GetHistory GET /api/v1/system/metrics/history
// Gibt die letzten 60 Snapshots zurück (für Chart-Initialisierung).
func (h *MetricsHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	history := h.metricsSvc.History()
	if history == nil {
		history = []*metrics.Snapshot{}
	}
	writeJSON(w, http.StatusOK, history)
}

// GetProcesses GET /api/v1/system/processes
// Gibt die Prozessliste zurück, sortierbar nach CPU/RAM/PID.
func (h *MetricsHandler) GetProcesses(w http.ResponseWriter, r *http.Request) {
	procs, err := h.metricsSvc.CollectProcesses()
	if err != nil {
		h.log.Error().Err(err).Msg("prozesse laden fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "prozesse konnten nicht geladen werden", "INTERNAL_ERROR")
		return
	}

	// Sortierung
	sortBy := r.URL.Query().Get("sort")
	switch sortBy {
	case "cpu":
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].CPUPercent > procs[j].CPUPercent
		})
	case "mem":
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].MemBytes > procs[j].MemBytes
		})
	case "pid":
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].PID < procs[j].PID
		})
	default: // Standard: nach RAM sortieren
		sort.Slice(procs, func(i, j int) bool {
			return procs[i].MemBytes > procs[j].MemBytes
		})
	}

	// Top N (Default: 50)
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if len(procs) > limit {
		procs = procs[:limit]
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"processes": procs,
		"total":     len(procs),
	})
}

// KillProcess DELETE /api/v1/system/processes/{pid}
// Beendet einen Prozess — erfordert aktive PIM-Sitzung.
func (h *MetricsHandler) KillProcess(w http.ResponseWriter, r *http.Request) {
	claims := authmw.ClaimsFromContext(r.Context())
	if claims == nil {
		writeError(w, http.StatusUnauthorized, "nicht authentifiziert", "UNAUTHORIZED")
		return
	}

	// PIM-Check
	pimID, err := h.pimSvc.CheckAndRecord(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusForbidden, "aktive pim-sitzung erforderlich", "PIM_REQUIRED")
		return
	}

	pidStr := chi.URLParam(r, "pid")
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		writeError(w, http.StatusBadRequest, "ungültige prozess-id", "INVALID_PID")
		return
	}

	// SIGTERM senden
	if err := killProcess(pid); err != nil {
		h.log.Error().Err(err).Int("pid", pid).Str("pim_id", pimID).Msg("prozess beenden fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "prozess konnte nicht beendet werden", "KILL_FAILED")
		return
	}

	h.log.Info().
		Int("pid", pid).
		Str("user_id", claims.UserID).
		Str("pim_id", pimID).
		Msg("prozess beendet")

	w.WriteHeader(http.StatusNoContent)
}

// writeJSON schreibt eine JSON-Antwort.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError schreibt eine JSON-Fehlerantwort.
func writeError(w http.ResponseWriter, status int, msg, code string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}
