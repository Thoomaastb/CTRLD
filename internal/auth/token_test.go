package auth_test

import (
	"testing"
	"time"

	"github.com/Thoomaastb/CTRLD/internal/auth"
)

var testSecret = []byte("test-secret-min-32-bytes-long-xx")

func testTokenConfig() auth.TokenConfig {
	return auth.TokenConfig{
		Secret:         testSecret,
		AccessTTLMin:   15,
		RefreshTTLDays: 7,
	}
}

func TestIssueTokenPair_Valid(t *testing.T) {
	pair, err := auth.IssueTokenPair(
		testTokenConfig(),
		"user-123", "test@example.com", "admin", "session-456",
	)
	if err != nil {
		t.Fatalf("token pair fehlgeschlagen: %v", err)
	}

	if pair.AccessToken == "" {
		t.Error("access token ist leer")
	}
	if pair.RefreshToken == "" {
		t.Error("refresh token ist leer")
	}
	if pair.AccessTokenHash == "" {
		t.Error("access token hash ist leer")
	}
	if pair.RefreshTokenHash == "" {
		t.Error("refresh token hash ist leer")
	}
}

func TestIssueTokenPair_Expiry(t *testing.T) {
	pair, _ := auth.IssueTokenPair(
		testTokenConfig(),
		"user-123", "test@example.com", "admin", "session-456",
	)

	now := time.Now().UTC()
	if pair.AccessExpiresAt.Before(now) {
		t.Error("access token ist bereits abgelaufen")
	}
	if pair.RefreshExpiresAt.Before(now) {
		t.Error("refresh token ist bereits abgelaufen")
	}

	// Access Token sollte ~15 Min gültig sein
	diff := pair.AccessExpiresAt.Sub(now)
	if diff < 14*time.Minute || diff > 16*time.Minute {
		t.Errorf("access token TTL unerwartet: %v", diff)
	}
}

func TestValidateAccessToken_Valid(t *testing.T) {
	pair, _ := auth.IssueTokenPair(
		testTokenConfig(),
		"user-123", "test@example.com", "admin", "session-456",
	)

	claims, err := auth.ValidateAccessToken(pair.AccessToken, testSecret)
	if err != nil {
		t.Fatalf("validierung fehlgeschlagen: %v", err)
	}

	if claims.UserID != "user-123" {
		t.Errorf("user id: erwartet 'user-123', bekommen '%s'", claims.UserID)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("email: erwartet 'test@example.com', bekommen '%s'", claims.Email)
	}
	if claims.Role != "admin" {
		t.Errorf("role: erwartet 'admin', bekommen '%s'", claims.Role)
	}
	if claims.SessionID != "session-456" {
		t.Errorf("session id: erwartet 'session-456', bekommen '%s'", claims.SessionID)
	}
}

func TestValidateAccessToken_WrongSecret(t *testing.T) {
	pair, _ := auth.IssueTokenPair(
		testTokenConfig(),
		"user-123", "test@example.com", "admin", "session-456",
	)

	_, err := auth.ValidateAccessToken(pair.AccessToken, []byte("falsches-secret-min-32-bytes-xx"))
	if err == nil {
		t.Error("falsches secret sollte fehler ergeben")
	}
}

func TestValidateAccessToken_Empty(t *testing.T) {
	_, err := auth.ValidateAccessToken("", testSecret)
	if err == nil {
		t.Error("leerer token sollte fehler ergeben")
	}
}

func TestValidateAccessToken_Tampered(t *testing.T) {
	pair, _ := auth.IssueTokenPair(
		testTokenConfig(),
		"user-123", "test@example.com", "admin", "session-456",
	)

	tampered := pair.AccessToken + "x"
	_, err := auth.ValidateAccessToken(tampered, testSecret)
	if err == nil {
		t.Error("manipulierter token sollte fehler ergeben")
	}
}

func TestHashTokenPublic_Consistent(t *testing.T) {
	token := "mein-test-token"
	h1 := auth.HashTokenPublic(token)
	h2 := auth.HashTokenPublic(token)

	if h1 != h2 {
		t.Error("gleicher token sollte gleichen hash ergeben")
	}
}
