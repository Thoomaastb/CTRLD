package users_test

import (
	"context"
	"testing"

	"github.com/rs/zerolog"

	database "github.com/Thoomaastb/CTRLD/internal/db"
	"github.com/Thoomaastb/CTRLD/internal/users"
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

func createAdmin(t *testing.T, svc *users.Service) string {
	t.Helper()
	u, err := svc.Create(context.Background(), users.CreateParams{
		Email:       "admin@example.com",
		Password:    "sicheres-passwort-123",
		Role:        "admin",
		RequestorID: "system",
	})
	if err != nil {
		t.Fatalf("admin erstellen fehlgeschlagen: %v", err)
	}
	return u.ID
}

func TestCreate_Valid(t *testing.T) {
	svc := users.New(newTestDB(t), zerolog.Nop())

	u, err := svc.Create(context.Background(), users.CreateParams{
		Email:       "user@example.com",
		Password:    "sicheres-passwort-123",
		Role:        "viewer",
		RequestorID: "system",
	})

	if err != nil {
		t.Fatalf("user erstellen fehlgeschlagen: %v", err)
	}
	if u.ID == "" {
		t.Error("user id ist leer")
	}
	if u.Role != "viewer" {
		t.Errorf("erwartet viewer, bekommen %s", u.Role)
	}
}

func TestCreate_InvalidEmail(t *testing.T) {
	svc := users.New(newTestDB(t), zerolog.Nop())

	_, err := svc.Create(context.Background(), users.CreateParams{
		Email:       "kein-email",
		Password:    "sicheres-passwort-123",
		Role:        "viewer",
		RequestorID: "system",
	})
	if err == nil {
		t.Error("ungültige email sollte fehler ergeben")
	}
}

func TestCreate_PasswordTooShort(t *testing.T) {
	svc := users.New(newTestDB(t), zerolog.Nop())

	_, err := svc.Create(context.Background(), users.CreateParams{
		Email:       "user@example.com",
		Password:    "kurz",
		Role:        "viewer",
		RequestorID: "system",
	})
	if err == nil {
		t.Error("zu kurzes passwort sollte fehler ergeben")
	}
}

func TestCreate_InvalidRole(t *testing.T) {
	svc := users.New(newTestDB(t), zerolog.Nop())

	_, err := svc.Create(context.Background(), users.CreateParams{
		Email:       "user@example.com",
		Password:    "sicheres-passwort-123",
		Role:        "superuser",
		RequestorID: "system",
	})
	if err == nil {
		t.Error("ungültige rolle sollte fehler ergeben")
	}
}

func TestCreate_DuplicateEmail(t *testing.T) {
	svc := users.New(newTestDB(t), zerolog.Nop())

	_, _ = svc.Create(context.Background(), users.CreateParams{
		Email:    "dupe@example.com",
		Password: "sicheres-passwort-123",
		Role:     "viewer",
	})

	_, err := svc.Create(context.Background(), users.CreateParams{
		Email:    "dupe@example.com",
		Password: "anderes-sicheres-passwort",
		Role:     "viewer",
	})
	if err == nil {
		t.Error("doppelte email sollte fehler ergeben")
	}
}

func TestList(t *testing.T) {
	svc := users.New(newTestDB(t), zerolog.Nop())

	_, _ = svc.Create(context.Background(), users.CreateParams{
		Email: "a@example.com", Password: "sicheres-passwort-123", Role: "admin",
	})
	_, _ = svc.Create(context.Background(), users.CreateParams{
		Email: "b@example.com", Password: "sicheres-passwort-456", Role: "viewer",
	})

	list, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("list fehlgeschlagen: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("erwartet 2 user, bekommen %d", len(list))
	}
}

func TestUpdateRole(t *testing.T) {
	svc := users.New(newTestDB(t), zerolog.Nop())
	adminID := createAdmin(t, svc)

	// Zweiten Admin erstellen
	u, _ := svc.Create(context.Background(), users.CreateParams{
		Email: "viewer@example.com", Password: "sicheres-passwort-123", Role: "viewer",
	})

	updated, err := svc.UpdateRole(context.Background(), users.UpdateRoleParams{
		UserID:      u.ID,
		NewRole:     "admin",
		RequestorID: adminID,
	})
	if err != nil {
		t.Fatalf("rolle ändern fehlgeschlagen: %v", err)
	}
	if updated.Role != "admin" {
		t.Errorf("erwartet admin, bekommen %s", updated.Role)
	}
}

func TestDeactivate_CannotDeleteSelf(t *testing.T) {
	svc := users.New(newTestDB(t), zerolog.Nop())
	adminID := createAdmin(t, svc)

	err := svc.Deactivate(context.Background(), adminID, adminID)
	if err == nil {
		t.Error("eigenen account deaktivieren sollte fehlschlagen")
	}
}

func TestDeactivate_LastAdmin(t *testing.T) {
	svc := users.New(newTestDB(t), zerolog.Nop())
	adminID := createAdmin(t, svc)
	otherUser, _ := svc.Create(context.Background(), users.CreateParams{
		Email: "other@example.com", Password: "sicheres-passwort-123", Role: "viewer",
	})

	// Letzten Admin deaktivieren — anderer User macht es
	err := svc.Deactivate(context.Background(), adminID, otherUser.ID)
	if err == nil {
		t.Error("letzten admin deaktivieren sollte fehlschlagen")
	}
}

func TestDeactivate_Valid(t *testing.T) {
	svc := users.New(newTestDB(t), zerolog.Nop())
	adminID := createAdmin(t, svc)

	// Zweiten Admin erstellen damit Deaktivierung möglich
	_, _ = svc.Create(context.Background(), users.CreateParams{
		Email: "admin2@example.com", Password: "sicheres-passwort-123", Role: "admin",
	})

	viewer, _ := svc.Create(context.Background(), users.CreateParams{
		Email: "viewer@example.com", Password: "sicheres-passwort-123", Role: "viewer",
	})

	err := svc.Deactivate(context.Background(), viewer.ID, adminID)
	if err != nil {
		t.Fatalf("deaktivieren fehlgeschlagen: %v", err)
	}
}
