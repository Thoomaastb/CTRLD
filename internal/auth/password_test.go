package auth_test

import (
	"strings"
	"testing"

	"github.com/Thoomaastb/CTRLD/internal/auth"
)

func TestHashPassword_Format(t *testing.T) {
	hash, err := auth.HashPassword("test-passwort-123")
	if err != nil {
		t.Fatalf("hash fehlgeschlagen: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash hat falsches format: %s", hash)
	}
}

func TestHashPassword_Unique(t *testing.T) {
	// Gleiche Eingabe → unterschiedliche Hashes (random salt)
	hash1, _ := auth.HashPassword("gleiches-passwort")
	hash2, _ := auth.HashPassword("gleiches-passwort")

	if hash1 == hash2 {
		t.Error("zwei hashes desselben passworts sollten unterschiedlich sein (salt)")
	}
}

func TestVerifyPassword_Correct(t *testing.T) {
	password := "mein-sicheres-passwort-2024!"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash fehlgeschlagen: %v", err)
	}

	ok, err := auth.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("verify fehlgeschlagen: %v", err)
	}
	if !ok {
		t.Error("korrektes passwort wurde abgelehnt")
	}
}

func TestVerifyPassword_Wrong(t *testing.T) {
	hash, _ := auth.HashPassword("richtiges-passwort")

	ok, err := auth.VerifyPassword("falsches-passwort", hash)
	if err != nil {
		t.Fatalf("unerwarteter fehler: %v", err)
	}
	if ok {
		t.Error("falsches passwort wurde akzeptiert")
	}
}

func TestVerifyPassword_InvalidHash(t *testing.T) {
	_, err := auth.VerifyPassword("passwort", "kein-valider-hash")
	if err == nil {
		t.Error("ungültiger hash sollte fehler zurückgeben")
	}
}

func TestVerifyPassword_EmptyPassword(t *testing.T) {
	hash, _ := auth.HashPassword("nicht-leer")

	ok, err := auth.VerifyPassword("", hash)
	if err != nil {
		t.Fatalf("unerwarteter fehler: %v", err)
	}
	if ok {
		t.Error("leeres passwort sollte abgelehnt werden")
	}
}
