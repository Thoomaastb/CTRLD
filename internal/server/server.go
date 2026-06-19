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

	"github.com/Thoomaastb/CTRLD/internal/audit"
	"github.com/Thoomaastb/CTRLD/internal/auth"
	authservice "github.com/Thoomaastb/CTRLD/internal/auth/service"
	"github.com/Thoomaastb/CTRLD/internal/config"
	database "github.com/Thoomaastb/CTRLD/internal/db"
	"github.com/Thoomaastb/CTRLD/internal/handler"
	"github.com/Thoomaastb/CTRLD/internal/health"
	mfapkg "github.com/Thoomaastb/CTRLD/internal/mfa"
	authmw "github.com/Thoomaastb/CTRLD/internal/middleware"
	"github.com/Thoomaastb/CTRLD/internal/pim"
	"github.com/Thoomaastb/CTRLD/internal/setup"
	"github.com/Thoomaastb/CTRLD/internal/users"
	"github.com/Thoomaastb/CTRLD/pkg/version"
)

// Server kapselt den HTTP-Server und alle Services.
type Server struct {
	cfg    *config.Config
	log    zerolog.Logger
	httpSv *http.Server
	db     *database.DB
}

// New erstellt einen vollständig verdrahteten Server.
func New(cfg *config.Config, db *database.DB, log zerolog.Logger) *Server {
	s := &Server{cfg: cfg, log: log, db: db}

	// ── Services instanziieren ────────────────────────────────────────────────
	tokenCfg := auth.TokenConfig{
		Secret:         []byte(cfg.Security.JWTSecret),
		AccessTTLMin:   cfg.Security.JWTAccessTTLMin,
		RefreshTTLDays: cfg.Security.JWTRefreshTTLDay,
	}

	authSvc   := authservice.New(db, tokenCfg, log)
	mfaSvc    := mfapkg.NewService(db, tokenCfg, log)
	pimSvc    := pim.New(db, log)
	auditSvc  := audit.New(db, log)
	setupSvc  := setup.New(db, log)
	usersSvc  := users.New(db, log)

	// ── Auth-Middleware ───────────────────────────────────────────────────────
	authn := authmw.NewAuthenticator([]byte(cfg.Security.JWTSecret))

	// ── Handler instanziieren ─────────────────────────────────────────────────
	authHandler  := handler.NewAuthHandler(authSvc, log)
	mfaHandler   := handler.NewMFAHandler(mfaSvc, authSvc, log)
	pimHandler   := handler.NewPIMHandler(pimSvc, auditSvc, log)
	setupHandler := handler.NewSetupHandler(setupSvc, usersSvc, log)

	// ── Router aufbauen ───────────────────────────────────────────────────────
	router := s.buildRouter(authn, authHandler, mfaHandler, pimHandler, setupHandler, setupSvc)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	s.httpSv = &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout:      time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
		IdleTimeout:       time.Duration(cfg.Server.IdleTimeoutSec) * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return s
}

func (s *Server) buildRouter(
	authn *authmw.Authenticator,
	authHandler *handler.AuthHandler,
	mfaHandler *handler.MFAHandler,
	pimHandler *handler.PIMHandler,
	setupHandler *handler.SetupHandler,
	setupSvc *setup.Service,
) *chi.Mux {
	r := chi.NewRouter()

	// ── Globale Middleware ────────────────────────────────────────────────────
	r.Use(middleware.RequestID)
	r.Use(s.loggerMiddleware())
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	// ── Health-Endpoint (immer erreichbar) ───────────────────────────────────
	r.Get("/api/v1/health", health.Handler)

	// ── API v1 ───────────────────────────────────────────────────────────────
	r.Route("/api/v1", func(r chi.Router) {
		// Setup-Wizard-Guard: Wenn Setup noch nicht abgeschlossen,
		// sind nur /setup/* und /auth/login erreichbar
		r.Use(s.setupGuard(setupSvc))

		// Auth-Endpunkte
		authHandler.RegisterRoutes(r, authn)

		// MFA-Endpunkte
		mfaHandler.RegisterRoutes(r, authn)

		// Setup-Wizard + User-Management
		setupHandler.RegisterRoutes(r, authn)

		// PIM + Audit
		pimHandler.RegisterRoutes(r, authn)
	})

	return r
}

// setupGuard blockiert API-Zugriff wenn Setup noch nicht abgeschlossen.
// Ausnahmen: /setup/*, /auth/login, /auth/refresh, /health
func (s *Server) setupGuard(setupSvc *setup.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Setup abgeschlossen → alles erlaubt
			if setupSvc.IsCompleted(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}

			// Erlaubte Pfade auch ohne abgeschlossenen Setup
			allowed := []string{
				"/api/v1/setup/",
				"/api/v1/auth/login",
				"/api/v1/auth/refresh",
				"/api/v1/health",
			}

			path := r.URL.Path
			for _, prefix := range allowed {
				if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
					next.ServeHTTP(w, r)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"setup nicht abgeschlossen","code":"SETUP_REQUIRED","setup_url":"/api/v1/setup/status"}`))
		})
	}
}

// securityHeaders setzt HTTP Security-Headers auf alle Antworten.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-XSS-Protection", "1; mode=block")
		h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
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

// Handler gibt den http.Handler zurück (für Tests).
func (s *Server) Handler() http.Handler {
	return s.httpSv.Handler
}

// Start startet den HTTP-Server.
func (s *Server) Start() error {
	addr := s.httpSv.Addr
	s.log.Info().
		Str("addr", addr).
		Str("version", version.Version).
		Bool("tls", s.cfg.Server.TLSCertFile != "").
		Msg("CTRLD server startet")

	var err error
	if s.cfg.Server.TLSCertFile != "" && s.cfg.Server.TLSKeyFile != "" {
		err = s.httpSv.ListenAndServeTLS(s.cfg.Server.TLSCertFile, s.cfg.Server.TLSKeyFile)
	} else {
		s.log.Warn().Msg("TLS nicht konfiguriert — HTTP only (nur für development)")
		err = s.httpSv.ListenAndServe()
	}

	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server: fehler: %w", err)
	}
	return nil
}

// Shutdown fährt den Server graceful herunter.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info().Msg("server shutdown initiiert")
	if err := s.httpSv.Shutdown(ctx); err != nil {
		return fmt.Errorf("server: shutdown fehler: %w", err)
	}
	s.log.Info().Msg("server graceful shutdown abgeschlossen")
	return nil
}
