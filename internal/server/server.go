package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/config"
	"github.com/Thoomaastb/CTRLD/internal/health"
)

// Server kapselt den HTTP-Server und seine Abhängigkeiten.
type Server struct {
	cfg    *config.Config
	log    zerolog.Logger
	httpSv *http.Server
}

// New erstellt einen neuen Server mit allen Routen und Middleware.
func New(cfg *config.Config, log zerolog.Logger) *Server {
	s := &Server{
		cfg: cfg,
		log: log,
	}

	router := s.buildRouter()

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	s.httpSv = &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeoutSec) * time.Second,
		// Verhindert Slowloris-Angriffe
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

// buildRouter registriert alle Routen und Middleware.
func (s *Server) buildRouter() *chi.Mux {
	r := chi.NewRouter()

	// --- Middleware-Stack (Reihenfolge ist wichtig) ---

	// Request-ID für Tracing
	r.Use(middleware.RequestID)

	// Structured Logging via zerolog
	r.Use(s.loggerMiddleware())

	// Panic Recovery — gibt 500 zurück statt den Prozess zu beenden
	r.Use(middleware.Recoverer)

	// Security-Headers — immer, auf allen Routes
	r.Use(securityHeaders)

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/health", health.Handler)
	})

	return r
}

// securityHeaders setzt HTTP Security-Headers auf alle Antworten.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		// Verhindert Clickjacking
		h.Set("X-Frame-Options", "DENY")

		// Verhindert MIME-Type Sniffing
		h.Set("X-Content-Type-Options", "nosniff")

		// XSS-Schutz (legacy, aber schadet nicht)
		h.Set("X-XSS-Protection", "1; mode=block")

		// HSTS — nur HTTPS erlaubt (1 Jahr, inkl. Subdomains)
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		// Referrer-Policy — kein Leaking von URLs
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content-Security-Policy — restriktiv für API-Responses
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// Keine Permissions für Browser-APIs
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		next.ServeHTTP(w, r)
	})
}

// loggerMiddleware erstellt einen zerolog-basierten Request-Logger.
func (s *Server) loggerMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				s.log.Info().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Str("remote_ip", realIP(r)).
					Str("request_id", middleware.GetReqID(r.Context())).
					Int("status", ww.Status()).
					Int("bytes", ww.BytesWritten()).
					Dur("latency", time.Since(start)).
					Msg("request")
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

// realIP extrahiert die echte Client-IP (hinter Proxies).
func realIP(r *http.Request) string {
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

// Handler gibt den http.Handler des Servers zurück (für Tests).
func (s *Server) Handler() http.Handler {
	return s.httpSv.Handler
}

// Start startet den HTTP-Server.
// Gibt einen Fehler zurück, wenn der Server nicht gestartet werden kann
// (z.B. Port bereits belegt). http.ErrServerClosed gilt nicht als Fehler.
func (s *Server) Start() error {
	addr := s.httpSv.Addr
	s.log.Info().
		Str("addr", addr).
		Bool("tls", s.cfg.Server.TLSCertFile != "").
		Msg("server starting")

	var err error
	if s.cfg.Server.TLSCertFile != "" && s.cfg.Server.TLSKeyFile != "" {
		err = s.httpSv.ListenAndServeTLS(
			s.cfg.Server.TLSCertFile,
			s.cfg.Server.TLSKeyFile,
		)
	} else {
		s.log.Warn().Msg("TLS nicht konfiguriert — server läuft über HTTP (nur für development)")
		err = s.httpSv.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: fehler beim starten: %w", err)
	}
	return nil
}

// Shutdown fährt den Server graceful herunter.
// Wartet bis alle aktiven Requests abgeschlossen sind oder der Context abläuft.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info().Msg("server shutdown initiiert")
	if err := s.httpSv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server: fehler beim shutdown: %w", err)
	}
	s.log.Info().Msg("server graceful shutdown abgeschlossen")
	return nil
}
