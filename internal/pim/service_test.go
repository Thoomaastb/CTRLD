package pim_test

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	database "github.com/Thoomaastb/CTRLD/internal/db"
	"github.com/Thoomaastb/CTRLD/internal/pim"
)

func newTestDB(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(":memory:", zerolog.Nop())
	if err != nil {
		t.Fatalf("db öffnen fehlgeschlagen: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func createTestUser(t *testing.T, db *database.DB) string {
	t.Helper()
	var id string
	err := db.SQL().QueryRow(`
		INSERT INTO users (id, email, password_hash, role, created_at, is_active)
		VALUES (lower(hex(randomblob(16))), 'test@example.com', 'hash', 'admin', CURRENT_TIMESTAMP, 1)
		RETURNING id`).Scan(&id)
	if err != nil {
		t.Fatalf("user erstellen fehlgeschlagen: %v", err)
	}
	return id
}

func createTestSession(t *testing.T, db *database.DB, userID string) string {
	t.Helper()
	var id string
	err := db.SQL().QueryRow(`
		INSERT INTO sessions (id, user_id, access_token_hash, refresh_token_hash, ip_address, created_at, expires_at)
		VALUES (lower(hex(randomblob(16))), ?, 'hash1', 'hash2', '127.0.0.1', CURRENT_TIMESTAMP, datetime('now', '+7 days'))
		RETURNING id`, userID).Scan(&id)
	if err != nil {
		t.Fatalf("session erstellen fehlgeschlagen: %v", err)
	}
	return id
}

func TestRequest_Valid(t *testing.T) {
	db := newTestDB(t)
	svc := pim.New(db, zerolog.Nop())
	userID := createTestUser(t, db)
	sessionID := createTestSession(t, db, userID)

	session, err := svc.Request(context.Background(), pim.RequestParams{
		UserID:      userID,
		SessionID:   sessionID,
		Reason:      "Kritisches System-Update durchführen",
		DurationMin: 30,
	})

	if err != nil {
		t.Fatalf("pim request fehlgeschlagen: %v", err)
	}
	if session.ID == "" {
		t.Error("session id ist leer")
	}
	if session.RemainingSeconds <= 0 {
		t.Error("remaining seconds sollte > 0 sein")
	}
	if session.IsBreakGlass {
		t.Error("sollte kein break-glass sein")
	}
}

func TestRequest_ReasonTooShort(t *testing.T) {
	db := newTestDB(t)
	svc := pim.New(db, zerolog.Nop())
	userID := createTestUser(t, db)
	sessionID := createTestSession(t, db, userID)

	_, err := svc.Request(context.Background(), pim.RequestParams{
		UserID:      userID,
		SessionID:   sessionID,
		Reason:      "kurz",
		DurationMin: 15,
	})

	if err == nil {
		t.Error("zu kurze begründung sollte fehler ergeben")
	}
}

func TestRequest_InvalidDuration(t *testing.T) {
	db := newTestDB(t)
	svc := pim.New(db, zerolog.Nop())
	userID := createTestUser(t, db)
	sessionID := createTestSession(t, db, userID)

	_, err := svc.Request(context.Background(), pim.RequestParams{
		UserID:      userID,
		SessionID:   sessionID,
		Reason:      "Gültige Begründung mit mehr als 10 Zeichen",
		DurationMin: 999,
	})

	if err == nil {
		t.Error("ungültige dauer sollte fehler ergeben")
	}
}

func TestRequest_AlreadyActive(t *testing.T) {
	db := newTestDB(t)
	svc := pim.New(db, zerolog.Nop())
	userID := createTestUser(t, db)
	sessionID := createTestSession(t, db, userID)

	params := pim.RequestParams{
		UserID:      userID,
		SessionID:   sessionID,
		Reason:      "Erste PIM-Sitzung starten jetzt",
		DurationMin: 15,
	}

	_, err := svc.Request(context.Background(), params)
	if err != nil {
		t.Fatalf("erste pim request fehlgeschlagen: %v", err)
	}

	_, err = svc.Request(context.Background(), params)
	if err == nil {
		t.Error("zweite pim request sollte fehlschlagen")
	}
}

func TestGetActive_NoSession(t *testing.T) {
	db := newTestDB(t)
	svc := pim.New(db, zerolog.Nop())
	userID := createTestUser(t, db)

	_, err := svc.GetActive(context.Background(), userID)
	if err == nil {
		t.Error("keine aktive session sollte fehler ergeben")
	}
}

func TestEnd_Valid(t *testing.T) {
	db := newTestDB(t)
	svc := pim.New(db, zerolog.Nop())
	userID := createTestUser(t, db)
	sessionID := createTestSession(t, db, userID)

	session, _ := svc.Request(context.Background(), pim.RequestParams{
		UserID:      userID,
		SessionID:   sessionID,
		Reason:      "PIM-Sitzung die beendet werden soll",
		DurationMin: 15,
	})

	err := svc.End(context.Background(), userID, sessionID, session.ID)
	if err != nil {
		t.Fatalf("pim beenden fehlgeschlagen: %v", err)
	}

	// Danach sollte keine aktive Sitzung mehr da sein
	_, err = svc.GetActive(context.Background(), userID)
	if err == nil {
		t.Error("nach beenden sollte keine aktive sitzung vorhanden sein")
	}
}

func TestBreakGlass(t *testing.T) {
	db := newTestDB(t)
	svc := pim.New(db, zerolog.Nop())
	userID := createTestUser(t, db)
	sessionID := createTestSession(t, db, userID)

	session, err := svc.Request(context.Background(), pim.RequestParams{
		UserID:       userID,
		SessionID:    sessionID,
		Reason:       "Notfallzugriff kritischer Produktionsausfall",
		DurationMin:  60,
		IsBreakGlass: true,
	})

	if err != nil {
		t.Fatalf("break-glass fehlgeschlagen: %v", err)
	}
	if !session.IsBreakGlass {
		t.Error("session sollte als break-glass markiert sein")
	}
}

func TestCheckAndRecord(t *testing.T) {
	db := newTestDB(t)
	svc := pim.New(db, zerolog.Nop())
	userID := createTestUser(t, db)
	sessionID := createTestSession(t, db, userID)

	session, _ := svc.Request(context.Background(), pim.RequestParams{
		UserID:      userID,
		SessionID:   sessionID,
		Reason:      "Aktion durchführen die gezählt wird",
		DurationMin: 15,
	})

	pimID, err := svc.CheckAndRecord(context.Background(), userID)
	if err != nil {
		t.Fatalf("check and record fehlgeschlagen: %v", err)
	}
	if pimID != session.ID {
		t.Errorf("pim id mismatch: %s vs %s", pimID, session.ID)
	}
}
