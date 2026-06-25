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
	"github.com/Thoomaastb/CTRLD/internal/metrics"
	mfapkg "github.com/Thoomaastb/CTRLD/internal/mfa"
	authmw "github.com/Thoomaastb/CTRLD/internal/middleware"
	"github.com/Thoomaastb/CTRLD/internal/pim"
	"github.com/Thoomaastb/CTRLD/internal/setup"
	"github.com/Thoomaastb/CTRLD/internal/users"
	wshub "github.com/Thoomaastb/CTRLD/internal/websocket"
	"github.com/Thoomaastb/CTRLD/pkg/version"
)

// Server kapselt den HTTP-Server und alle Services.
type Server struct {
	cfg        *config.Config
	log        zerolog.Logger
	httpSv     *http.Server
	db         *database.DB
	metricsSvc *metrics.Service
	wsHub      *wshub.Hub
}

// New erstellt einen vollständig verdrahteten Server.
func New(cfg *config.Config, db *database.DB, log zerolog.Logger) *Server {
	s := &Server{cfg: cfg, log: log, db: db}

	tokenCfg := auth.TokenConfig{
		Secret:         []byte(cfg.Security.JWTSecret),
		AccessTTLMin:   cfg.Security.JWTAccessTTLMin,
		RefreshTTLDays: cfg.Security.JWTRefreshTTLDay,
	}

	// ── Services ─────────────────────────────────────────────────────────────
	authSvc     := authservice.New(db, tokenCfg, log)
	mfaSvc      := mfapkg.NewService(db, tokenCfg, log)
	pimSvc      := pim.New(db, log)
	auditSvc    := audit.New(db, log)
	setupSvc    := setup.New(db, log)
	usersSvc    := users.New(db, log)
	metricsSvc  := metrics.NewService(log)
	s.metricsSvc = metricsSvc

	// WebSocket Hub
	wsHub := wshub.NewHub(metricsSvc, []byte(cfg.Security.JWTSecret), log)
	s.wsHub = wsHub

	// ── Handler ───────────────────────────────────────────────────────────────
	authn          := authmw.NewAuthenticator([]byte(cfg.Security.JWTSecret))
	authHandler    := handler.NewAuthHandler(authSvc, log)
	mfaHandler     := handler.NewMFAHandler(mfaSvc, authSvc, log)
	pimHandler     := handler.NewPIMHandler(pimSvc, auditSvc, log)
	setupHandler   := handler.NewSetupHandler(setupSvc, usersSvc, log)
	metricsHandler := handler.NewMetricsHandler(metricsSvc, pimSvc, log)

	router := s.buildRouter(authn, authHandler, mfaHandler, pimHandler, setupHandler, metricsHandler, setupSvc, wsHub)

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
	metricsHandler *handler.MetricsHandler,
	setupSvc *setup.Service,
	wsHub *wshub.Hub,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(s.loggerMiddleware())
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	// Health (immer erreichbar)
	r.Get("/api/v1/health", health.Handler)

	// WebSocket (Auth via Query-Param)
	r.Get("/ws/metrics", wsHub.ServeMetrics)

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.setupGuard(setupSvc))

		authHandler.RegisterRoutes(r, authn)
		mfaHandler.RegisterRoutes(r, authn)
		setupHandler.RegisterRoutes(r, authn)
		pimHandler.RegisterRoutes(r, authn)
		metricsHandler.RegisterRoutes(r, authn)
	})

	return r
}

func (s *Server) setupGuard(setupSvc *setup.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if setupSvc.IsCompleted(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}
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

// Start startet den HTTP-Server + Metrics-Collection + WebSocket-Hub.
func (s *Server) Start() error {
	ctx, cancel := context.WithCancel(context.Background())
	s.log.Info().
		Str("addr", s.httpSv.Addr).
		Str("version", version.Version).
		Msg("CTRLD server startet")

	// Metrics-Collection starten
	go s.metricsSvc.Start(ctx)

	// WebSocket-Hub starten
	go s.wsHub.Run(ctx)

	var err error
	if s.cfg.Server.TLSCertFile != "" && s.cfg.Server.TLSKeyFile != "" {
		err = s.httpSv.ListenAndServeTLS(s.cfg.Server.TLSCertFile, s.cfg.Server.TLSKeyFile)
	} else {
		s.log.Warn().Msg("TLS nicht konfiguriert — HTTP only (nur für development)")
		err = s.httpSv.ListenAndServe()
	}

	cancel()

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
