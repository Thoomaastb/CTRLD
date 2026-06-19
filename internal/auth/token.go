package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenExpired  = errors.New("auth: token abgelaufen")
	ErrTokenInvalid  = errors.New("auth: token ungültig")
	ErrTokenMissing  = errors.New("auth: token fehlt")
)

// Claims enthält die JWT-Payload.
type Claims struct {
	UserID    string `json:"uid"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

// TokenPair enthält Access- und Refresh-Token.
type TokenPair struct {
	AccessToken        string
	RefreshToken       string
	AccessTokenHash    string // SHA-256, wird in DB gespeichert
	RefreshTokenHash   string // SHA-256, wird in DB gespeichert
	AccessExpiresAt    time.Time
	RefreshExpiresAt   time.Time
}

// TokenConfig konfiguriert die Token-Parameter.
type TokenConfig struct {
	Secret          []byte
	AccessTTLMin    int // Default: 15
	RefreshTTLDays  int // Default: 7
}

// IssueTokenPair erstellt ein neues Access- + Refresh-Token-Paar.
func IssueTokenPair(cfg TokenConfig, userID, email, role, sessionID string) (*TokenPair, error) {
	if cfg.AccessTTLMin == 0 {
		cfg.AccessTTLMin = 15
	}
	if cfg.RefreshTTLDays == 0 {
		cfg.RefreshTTLDays = 7
	}

	now := time.Now().UTC()
	accessExp := now.Add(time.Duration(cfg.AccessTTLMin) * time.Minute)
	refreshExp := now.Add(time.Duration(cfg.RefreshTTLDays) * 24 * time.Hour)

	// Access Token
	accessClaims := Claims{
		UserID:    userID,
		Email:     email,
		Role:      role,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExp),
			Issuer:    "ctrld",
			Subject:   userID,
		},
	}

	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(cfg.Secret)
	if err != nil {
		return nil, fmt.Errorf("auth: access token signierung fehlgeschlagen: %w", err)
	}

	// Refresh Token — opakes zufälliges Token (kein JWT)
	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return nil, fmt.Errorf("auth: refresh token generierung fehlgeschlagen: %w", err)
	}
	refreshToken := hex.EncodeToString(refreshBytes)

	return &TokenPair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessTokenHash:  hashToken(accessToken),
		RefreshTokenHash: hashToken(refreshToken),
		AccessExpiresAt:  accessExp,
		RefreshExpiresAt: refreshExp,
	}, nil
}

// ValidateAccessToken prüft und dekodiert einen JWT Access Token.
func ValidateAccessToken(tokenStr string, secret []byte) (*Claims, error) {
	if tokenStr == "" {
		return nil, ErrTokenMissing
	}

	token, err := jwt.ParseWithClaims(
		tokenStr,
		&Claims{},
		func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("auth: unerwartete signing method: %v", t.Header["alg"])
			}
			return secret, nil
		},
		jwt.WithIssuer("ctrld"),
		jwt.WithExpirationRequired(),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}

	return claims, nil
}

// HashToken erstellt einen SHA-256-Hash eines Tokens für die DB-Speicherung.
// Niemals den Rohtoken in der DB speichern.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// HashTokenPublic ist die öffentliche Version für externe Nutzung (z.B. Lookup).
func HashTokenPublic(token string) string {
	return hashToken(token)
}
