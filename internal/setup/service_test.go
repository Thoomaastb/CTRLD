package setup_test

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	database "github.com/Thoomaastb/CTRLD/internal/db"
	"github.com/Thoomaastb/CTRLD/internal/setup"
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

func TestGetStatus_Initial(t *testing.T) {
	db := newTestDB(t)
	svc := setup.New(db, zerolog.Nop())

	status, err := svc.GetStatus(context.Background())
	if err != nil {
		t.Fatalf("status fehlgeschlagen: %v", err)
	}

	if status.IsCompleted {
		t.Error("frische installation sollte nicht abgeschlossen sein")
	}
	if status.HasAdmin {
		t.Error("frische installation sollte keinen admin haben")
	}
	if status.CurrentStep != 1 {
		t.Errorf("erwartet step 1, bekommen %d", status.CurrentStep)
	}
}

func TestCreateAdmin_Valid(t *testing.T) {
	db := newTestDB(t)
	svc := setup.New(db, zerolog.Nop())

	result, err := svc.CreateAdmin(context.Background(), setup.CreateAdminParams{
		Email:    "admin@example.com",
		Password: "sicheres-passwort-123",
	})

	if err != nil {
		t.Fatalf("admin erstellen fehlgeschlagen: %v", err)
	}
	if result.UserID == "" {
		t.Error("user id ist leer")
	}
	if result.Email != "admin@example.com" {
		t.Errorf("email mismatch: %s", result.Email)
	}
}

func TestCreateAdmin_InvalidEmail(t *testing.T) {
	db := newTestDB(t)
	svc := setup.New(db, zerolog.Nop())

	_, err := svc.CreateAdmin(context.Background(), setup.CreateAdminParams{
		Email:    "kein-email",
		Password: "sicheres-passwort-123",
	})

	if err == nil {
		t.Error("ungültige email sollte fehler ergeben")
	}
}

func TestCreateAdmin_PasswordTooShort(t *testing.T) {
	db := newTestDB(t)
	svc := setup.New(db, zerolog.Nop())

	_, err := svc.CreateAdmin(context.Background(), setup.CreateAdminParams{
		Email:    "admin@example.com",
		Password: "kurz",
	})

	if err == nil {
		t.Error("zu kurzes passwort sollte fehler ergeben")
	}
}

func TestCreateAdmin_DuplicatePrevented(t *testing.T) {
	db := newTestDB(t)
	svc := setup.New(db, zerolog.Nop())

	_, err := svc.CreateAdmin(context.Background(), setup.CreateAdminParams{
		Email:    "admin@example.com",
		Password: "sicheres-passwort-123",
	})
	if err != nil {
		t.Fatalf("erster admin fehlgeschlagen: %v", err)
	}

	_, err = svc.CreateAdmin(context.Background(), setup.CreateAdminParams{
		Email:    "admin2@example.com",
		Password: "anderes-sicheres-passwort",
	})
	if err == nil {
		t.Error("zweiter admin sollte verhindert werden")
	}
}

func TestComplete_Valid(t *testing.T) {
	db := newTestDB(t)
	svc := setup.New(db, zerolog.Nop())

	result, _ := svc.CreateAdmin(context.Background(), setup.CreateAdminParams{
		Email:    "admin@example.com",
		Password: "sicheres-passwort-123",
	})

	err := svc.Complete(context.Background(), result.UserID)
	if err != nil {
		t.Fatalf("complete fehlgeschlagen: %v", err)
	}

	if !svc.IsCompleted(context.Background()) {
		t.Error("setup sollte als abgeschlossen markiert sein")
	}
}

func TestComplete_WithoutAdmin(t *testing.T) {
	db := newTestDB(t)
	svc := setup.New(db, zerolog.Nop())

	err := svc.Complete(context.Background(), "fake-id")
	if err == nil {
		t.Error("complete ohne admin sollte fehlschlagen")
	}
}

func TestIsCompleted_AfterComplete(t *testing.T) {
	db := newTestDB(t)
	svc := setup.New(db, zerolog.Nop())

	if svc.IsCompleted(context.Background()) {
		t.Error("initial sollte nicht abgeschlossen sein")
	}

	result, _ := svc.CreateAdmin(context.Background(), setup.CreateAdminParams{
		Email:    "admin@example.com",
		Password: "sicheres-passwort-123",
	})
	_ = svc.Complete(context.Background(), result.UserID)

	if !svc.IsCompleted(context.Background()) {
		t.Error("nach complete sollte abgeschlossen sein")
	}
}
