package websocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"

	"github.com/Thoomaastab/CTRLD/internal/metrics"
	authmw "github.com/Thoomaastab/CTRLD/internal/middleware"
	"github.com/Thoomaastab/CTRLD/internal/auth"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 4096,
	// Origin-Check: nur eigene Domain erlaubt (wird via Config konfigurierbar)
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Origin gegen Config prüfen
		return true
	},
}

// Message ist das WebSocket-Nachrichtenformat.
type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// client repräsentiert eine WebSocket-Verbindung.
type client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub verwaltet alle aktiven WebSocket-Verbindungen.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*client]bool
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
	metricsSvc *metrics.Service
	jwtSecret  []byte
	log        zerolog.Logger
}

// NewHub erstellt einen neuen WebSocket Hub.
func NewHub(metricsSvc *metrics.Service, jwtSecret []byte, log zerolog.Logger) *Hub {
	return &Hub{
		clients:    make(map[*client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *client),
		unregister: make(chan *client),
		metricsSvc: metricsSvc,
		jwtSecret:  jwtSecret,
		log:        log,
	}
}

// Run startet den Hub — läuft bis Context abgebrochen wird.
func (h *Hub) Run(ctx context.Context) {
	// Metriken-Ticker: alle 1s broadcasten
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()
			h.log.Debug().Int("clients", len(h.clients)).Msg("ws client verbunden")

			// Initiale History senden
			go h.sendHistory(c)

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
			h.log.Debug().Int("clients", len(h.clients)).Msg("ws client getrennt")

		case msg := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Client zu langsam — trennen
					close(c.send)
					delete(h.clients, c)
				}
			}
			h.mu.RUnlock()

		case <-ticker.C:
			snap := h.metricsSvc.Latest()
			if snap == nil {
				continue
			}
			h.broadcastSnapshot(snap)
		}
	}
}

// ServeMetrics upgrades eine HTTP-Verbindung zu WebSocket.
// Erfordert gültigen JWT im Query-Parameter oder Authorization-Header.
func (h *Hub) ServeMetrics(w http.ResponseWriter, r *http.Request) {
	// Auth-Check
	token := r.URL.Query().Get("token")
	if token == "" {
		token = extractBearerFromHeader(r)
	}

	if _, err := auth.ValidateAccessToken(token, h.jwtSecret); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error().Err(err).Msg("ws upgrade fehlgeschlagen")
		return
	}

	c := &client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- c

	go c.writePump()
	go c.readPump()
}

// broadcastSnapshot sendet einen Snapshot an alle Clients.
func (h *Hub) broadcastSnapshot(snap *metrics.Snapshot) {
	data, err := json.Marshal(snap)
	if err != nil {
		return
	}

	msg := Message{Type: "metrics", Data: data}
	encoded, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.broadcast <- encoded
}

// sendHistory sendet die History an einen neuen Client.
func (h *Hub) sendHistory(c *client) {
	history := h.metricsSvc.History()
	if len(history) == 0 {
		return
	}

	data, err := json.Marshal(history)
	if err != nil {
		return
	}

	msg := Message{Type: "history", Data: data}
	encoded, err := json.Marshal(msg)
	if err != nil {
		return
	}

	select {
	case c.send <- encoded:
	default:
	}
}

// writePump schreibt Nachrichten an den WebSocket-Client.
func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readPump liest eingehende Nachrichten (Pong + Subscribe-Requests).
func (c *client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.log.Debug().Err(err).Msg("ws verbindung unerwartet getrennt")
			}
			break
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

// ClientCount gibt die Anzahl aktiver WebSocket-Verbindungen zurück.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Sicherstellen dass authmw importiert wird (für spätere Nutzung)
var _ = authmw.ClaimsFromContext
