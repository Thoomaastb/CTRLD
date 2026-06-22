package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/auth"
	"github.com/Thoomaastb/CTRLD/internal/logs"
	authmw "github.com/Thoomaastb/CTRLD/internal/middleware"
)

var logUpgrader = websocket.Upgrader{
	ReadBufferSize:  512,
	WriteBufferSize: 4096,
	CheckOrigin:    func(r *http.Request) bool { return true },
}

// LogHandler kapselt Log-HTTP-Handler.
type LogHandler struct {
	jwtSecret []byte
	log       zerolog.Logger
}

// NewLogHandler erstellt einen neuen LogHandler.
func NewLogHandler(jwtSecret []byte, log zerolog.Logger) *LogHandler {
	return &LogHandler{jwtSecret: jwtSecret, log: log}
}

// RegisterRoutes registriert alle Log-Routen.
func (h *LogHandler) RegisterRoutes(r chi.Router, authn *authmw.Authenticator) {
	r.Group(func(r chi.Router) {
		r.Use(authn.Require)
		r.Get("/logs",         h.GetLogs)
		r.Get("/logs/units",   h.GetUnits)
		r.Get("/logs/sources", h.GetSources)
		r.Get("/logs/export",  h.ExportLogs)
	})

	// WebSocket — Auth via Query-Param
	r.Get("/ws/logs", h.TailLogs)
}

// GetLogs GET /api/v1/logs
func (h *LogHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	lines, _ := strconv.Atoi(q.Get("lines"))
	if lines <= 0 {
		lines = 100
	}

	params := logs.QueryParams{
		Source:   q.Get("source"),
		Unit:     q.Get("unit"),
		Severity: q.Get("severity"),
		Search:   q.Get("search"),
		Lines:    lines,
	}

	if since := q.Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			params.Since = t
		}
	}

	entries, err := logs.Read(r.Context(), params)
	if err != nil {
		h.log.Error().Err(err).Msg("logs lesen fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "logs konnten nicht geladen werden", "INTERNAL_ERROR")
		return
	}

	if entries == nil {
		entries = []logs.Entry{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
		"params":  params,
	})
}

// GetUnits GET /api/v1/logs/units
func (h *LogHandler) GetUnits(w http.ResponseWriter, r *http.Request) {
	units, err := logs.ListUnits(r.Context())
	if err != nil {
		h.log.Error().Err(err).Msg("units laden fehlgeschlagen")
		writeError(w, http.StatusInternalServerError, "units konnten nicht geladen werden", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, units)
}

// GetSources GET /api/v1/logs/sources
func (h *LogHandler) GetSources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, logs.AvailableSources())
}

// ExportLogs GET /api/v1/logs/export?format=txt|json
func (h *LogHandler) ExportLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	format := q.Get("format")
	if format == "" {
		format = "txt"
	}

	params := logs.QueryParams{
		Source:   q.Get("source"),
		Unit:     q.Get("unit"),
		Severity: q.Get("severity"),
		Search:   q.Get("search"),
		Lines:    500,
	}

	entries, err := logs.Read(r.Context(), params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export fehlgeschlagen", "INTERNAL_ERROR")
		return
	}

	switch format {
	case "json":
		data, err := logs.ExportJSON(entries)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "export fehlgeschlagen", "INTERNAL_ERROR")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=ctrld-logs.json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(data))

	default: // txt
		data := logs.ExportTXT(entries)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=ctrld-logs.txt")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(data))
	}
}

// TailLogs GET /ws/logs?token=<JWT>&unit=<unit>
func (h *LogHandler) TailLogs(w http.ResponseWriter, r *http.Request) {
	// Auth
	token := r.URL.Query().Get("token")
	if token == "" {
		token = extractBearerFromHeader(r)
	}
	if _, err := auth.ValidateAccessToken(token, h.jwtSecret); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := logUpgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error().Err(err).Msg("ws log upgrade fehlgeschlagen")
		return
	}
	defer conn.Close()

	unit := r.URL.Query().Get("unit")

	tailCh := make(chan logs.TailEntry, 256)

	ctx := r.Context()
	if err := logs.Tail(ctx, logs.TailParams{Unit: unit}, tailCh); err != nil {
		h.log.Error().Err(err).Msg("log tail fehlgeschlagen")
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-tailCh:
			data, err := json.Marshal(map[string]interface{}{
				"type": "log",
				"data": entry,
			})
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}
	}
}

func extractBearerFromHeader(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && h[:7] == "Bearer " {
		return h[7:]
	}
	return ""
}
