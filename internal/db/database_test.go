package database_test

import (
	"context"
	"os"
	"testing"

	"github.com/rs/zerolog"

	database "github.com/Thoomaastb/CTRLD/internal/db"
)

func newTestDB(t *testing.T) *database.DB {
	t.Helper()

	// In-Memory SQLite für Tests
	db, err := database.Open(":memory:", zerolog.Nop())
	if err != nil {
		t.Fatalf("datenbank konnte nicht geöffnet werden: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("fehler beim schließen der test-datenbank: %v", err)
		}
	})

	return db
}

func TestOpen(t *testing.T) {
	db := newTestDB(t)
	if db == nil {
		t.Fatal("db ist nil")
	}
}

func TestPing(t *testing.T) {
	db := newTestDB(t)
	if err := db.Ping(context.Background()); err != nil {
		t.Errorf("ping fehlgeschlagen: %v", err)
	}
}

func TestMigrationsApplied(t *testing.T) {
	db := newTestDB(t)

	// Prüfen ob alle erwarteten Tabellen existieren
	tables := []string{
		"users",
		"mfa_credentials",
		"sessions",
		"pim_sessions",
		"audit_log",
		"config",
		"alerts",
	}

	for _, table := range tables {
		var name string
		err := db.SQL().QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)

		if err != nil {
			t.Errorf("tabelle %q nicht gefunden: %v", table, err)
		}
	}
}

func TestAuditLogAppendOnly_NoUpdate(t *testing.T) {
	db := newTestDB(t)

	// Eintrag anlegen
	_, err := db.SQL().Exec(`
		INSERT INTO audit_log (id, action_type, result, severity, created_at)
		VALUES ('test-uuid-1', 'test.action', 'success', 'info', CURRENT_TIMESTAMP)
	`)
	if err != nil {
		t.Fatalf("insert fehlgeschlagen: %v", err)
	}

	// UPDATE muss fehlschlagen
	_, err = db.SQL().Exec(`
		UPDATE audit_log SET result = 'failure' WHERE id = 'test-uuid-1'
	`)
	if err == nil {
		t.Error("UPDATE auf audit_log sollte fehlschlagen (append-only trigger)")
	}
}

func TestAuditLogAppendOnly_NoDelete(t *testing.T) {
	db := newTestDB(t)

	// Eintrag anlegen
	_, err := db.SQL().Exec(`
		INSERT INTO audit_log (id, action_type, result, severity, created_at)
		VALUES ('test-uuid-2', 'test.action', 'success', 'info', CURRENT_TIMESTAMP)
	`)
	if err != nil {
		t.Fatalf("insert fehlgeschlagen: %v", err)
	}

	// DELETE muss fehlschlagen
	_, err = db.SQL().Exec(`DELETE FROM audit_log WHERE id = 'test-uuid-2'`)
	if err == nil {
		t.Error("DELETE auf audit_log sollte fehlschlagen (append-only trigger)")
	}
}

func TestForeignKeysEnabled(t *testing.T) {
	db := newTestDB(t)

	// Insert in sessions mit ungültiger user_id muss fehlschlagen
	_, err := db.SQL().Exec(`
		INSERT INTO sessions (
			id, user_id, access_token_hash, refresh_token_hash,
			ip_address, created_at, expires_at
		) VALUES (
			'sess-1', 'nonexistent-user', 'hash1', 'hash2',
			'127.0.0.1', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
		)
	`)
	if err == nil {
		t.Error("FK-Verletzung sollte fehlschlagen (foreign_keys=ON)")
	}
}

func TestFileDB(t *testing.T) {
	// Test mit echter Datei (nicht in-memory)
	tmpFile, err := os.CreateTemp(t.TempDir(), "ctrld-test-*.db")
	if err != nil {
		t.Fatalf("temp-datei konnte nicht erstellt werden: %v", err)
	}
	tmpFile.Close()

	db, err := database.Open(tmpFile.Name(), zerolog.Nop())
	if err != nil {
		t.Fatalf("datei-db konnte nicht geöffnet werden: %v", err)
	}
	defer db.Close()

	if err := db.Ping(context.Background()); err != nil {
		t.Errorf("ping auf datei-db fehlgeschlagen: %v", err)
	}
}
