package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/Thoomaastb/CTRLD/internal/config"
	"github.com/Thoomaastb/CTRLD/internal/server"
	"github.com/Thoomaastb/CTRLD/pkg/version"
)

func main() {
	// --- Flags ---
	cfgFile := flag.String("config", "", "Pfad zur Konfigurationsdatei (optional)")
	flag.Parse()

	// --- Konfiguration laden ---
	cfg, err := config.Load(*cfgFile)
	if err != nil {
		// Vor Logger-Setup: Fallback auf stderr
		log.Fatal().Err(err).Msg("konfiguration konnte nicht geladen werden")
	}

	// --- Logger initialisieren ---
	logger := buildLogger(cfg)
	logger.Info().
		Str("version", version.Version).
		Str("log_level", cfg.Log.Level).
		Msg("CTRLD startet")

	// --- Server erstellen ---
	srv := server.New(cfg, logger)

	// --- Server in Goroutine starten ---
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Start()
	}()

	// --- Shutdown-Signal abfangen (SIGINT, SIGTERM) ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info().Str("signal", sig.String()).Msg("shutdown signal empfangen")
	case err := <-serverErr:
		if err != nil {
			logger.Error().Err(err).Msg("server fehler")
			os.Exit(1)
		}
	}

	// --- Graceful Shutdown (30s Timeout) ---
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("shutdown fehler")
		os.Exit(1)
	}

	logger.Info().Msg("CTRLD beendet")
}

// buildLogger erstellt einen zerolog.Logger basierend auf der Konfiguration.
func buildLogger(cfg *config.Config) zerolog.Logger {
	// Log-Level parsen
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Format: pretty (Dev) oder JSON (Prod)
	if cfg.Log.Format == "pretty" {
		return log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	}

	// JSON-Output mit Timestamp
	zerolog.TimeFieldFormat = time.RFC3339
	return zerolog.New(os.Stderr).With().Timestamp().Logger()
}
