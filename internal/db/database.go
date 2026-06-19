package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite-Treiber
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
)

//go:embed migrations/*.sql
var migrations embed.FS

// DB kapselt die SQLite-Verbindung und den Query-Layer.
type DB struct {
	sql *sql.DB
	log zerolog.Logger
}

// Open öffnet die SQLite-Datenbank, konfiguriert PRAGMAs und führt Migrations aus.
func Open(path string, log zerolog.Logger) (*DB, error) {
	// WAL-Mode und foreign keys via DSN aktivieren
	dsn := fmt.Sprintf("%s?_journal_mode=WAL&_foreign_keys=ON&_busy_timeout=5000", path)

	sqlDB, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open fehler: %w", err)
	}

	// Connection Pool — SQLite ist single-writer, daher max 1 Writer
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// PRAGMAs für Performance und Sicherheit
	if err := applyPragmas(sqlDB); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	db := &DB{sql: sqlDB, log: log}

	// Migrations ausführen
	if err := db.migrate(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}

	log.Info().Str("path", path).Msg("datenbank geöffnet")
	return db, nil
}

// applyPragmas setzt SQLite-PRAGMAs für optimale Performance und Sicherheit.
func applyPragmas(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",        // Write-Ahead Logging für Concurrent Reads
		"PRAGMA synchronous=NORMAL",      // Gute Performance, akzeptables Durability-Risiko
		"PRAGMA cache_size=-64000",       // 64 MB Cache
		"PRAGMA temp_store=MEMORY",       // Temp-Tabellen im RAM
		"PRAGMA mmap_size=268435456",     // 256 MB Memory-Mapped I/O
		"PRAGMA foreign_keys=ON",         // FK-Constraints erzwingen
		"PRAGMA secure_delete=ON",        // Gelöschte Daten überschreiben
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			return fmt.Errorf("db: pragma fehler (%s): %w", p, err)
		}
	}
	return nil
}

// migrate führt ausstehende goose-Migrations aus.
func (db *DB) migrate() error {
	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger()) // zerolog übernimmt das Logging

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("db: goose dialect fehler: %w", err)
	}

	if err := goose.Up(db.sql, "migrations"); err != nil {
		return fmt.Errorf("db: migration fehler: %w", err)
	}

	db.log.Info().Msg("datenbank migrations erfolgreich")
	return nil
}

// Ping prüft ob die Datenbankverbindung aktiv ist.
func (db *DB) Ping(ctx context.Context) error {
	return db.sql.PingContext(ctx)
}

// Close schließt die Datenbankverbindung sauber.
func (db *DB) Close() error {
	db.log.Info().Msg("datenbank verbindung geschlossen")
	return db.sql.Close()
}

// SQL gibt die rohe *sql.DB zurück (für sqlc-generated Queries).
func (db *DB) SQL() *sql.DB {
	return db.sql
}
