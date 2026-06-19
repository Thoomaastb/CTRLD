package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/auth"
	database "github.com/Thoomaastb/CTRLD/internal/db"
	"github.com/Thoomaastb/CTRLD/internal/db/generated"
	"github.com/Thoomaastb/CTRLD/internal/ratelimit"
)

var (
	ErrInvalidCredentials = errors.New("service: ungültige anmeldedaten")
	ErrAccountInactive    = errors.New("service: account deaktiviert")
	ErrSessionNotFound    = errors.New("service: session nicht gefunden")
	ErrRateLimited        = errors.New("service: zu viele fehlversuche")
)

// AuthService kapselt die gesamte Auth-Logik.
type AuthService struct {
	db          *database.DB
	queries     *db.Queries
	tokenCfg    auth.TokenConfig
	rateLimiter *ratelimit.Limiter
	log         zerolog.Logger
}

// New erstellt einen neuen AuthService.
func New(database *database.DB, tokenCfg auth.TokenConfig, log zerolog.Logger) *AuthService {
	return &AuthService{
		db:          database,
		queries:     db.New(database.SQL()),
		tokenCfg:    tokenCfg,
		rateLimiter: ratelimit.New(),
		log:         log,
	}
}

// LoginRequest enthält die Login-Eingabedaten.
type LoginRequest struct {
	Email     string
	Password  string
	IPAddress string
	UserAgent string
}

// LoginResult enthält das Ergebnis eines erfolgreichen Logins.
type LoginResult struct {
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
	UserID           string
	Role             string
	// MFARequired: true wenn MFA noch ausstehend ist
	MFARequired bool
	// MFASessionToken: temporärer Token für MFA-Schritt (kein vollwertiger JWT)
	MFASessionToken string
}

// Login prüft Credentials und gibt ein Token-Paar zurück.
// Kein User-Enumeration: immer generische Fehlermeldung bei falschen Credentials.
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResult, error) {
	// Rate-Limit prüfen
	if blocked, _ := s.rateLimiter.IsBlocked(req.IPAddress); blocked {
		s.log.Warn().
			Str("ip", req.IPAddress).
			Msg("login blockiert: rate limit")
		return nil, ErrRateLimited
	}

	// User laden — kein Unterschied bei "nicht gefunden" vs "falsches Passwort"
	user, err := s.queries.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Trotzdem hash-Vergleich simulieren gegen Timing-Angriffe
			_, _ = auth.VerifyPassword(req.Password, "$argon2id$v=19$m=65536,t=3,p=2$dummysalt123456$dummyhash123456789012345678901234")
			s.rateLimiter.RecordFailure(req.IPAddress)
			s.logAudit(ctx, "", req.IPAddress, "auth.login.failure", "email:"+req.Email, "failure", "info")
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("service: user lookup fehlgeschlagen: %w", err)
	}

	// Account aktiv?
	if user.IsActive == 0 {
		s.rateLimiter.RecordFailure(req.IPAddress)
		return nil, ErrAccountInactive
	}

	// Passwort prüfen
	ok, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !ok {
		s.rateLimiter.RecordFailure(req.IPAddress)
		s.logAudit(ctx, user.ID, req.IPAddress, "auth.login.failure", "", "failure", "warning")
		return nil, ErrInvalidCredentials
	}

	// Rate-Limit zurücksetzen bei Erfolg
	s.rateLimiter.Reset(req.IPAddress)

	// Last-Login aktualisieren
	_ = s.queries.UpdateLastLogin(ctx, user.ID)

	// MFA prüfen — hat der User MFA eingerichtet?
	mfaCreds, err := s.queries.ListMFACredentialsByUserID(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("service: mfa check fehlgeschlagen: %w", err)
	}

	// MFA vorhanden → MFA-Schritt erforderlich
	// Wir geben einen temporären Session-Token zurück, kein vollwertiges JWT
	if len(mfaCreds) > 0 {
		mfaToken, err := s.issueMFASessionToken(user.ID)
		if err != nil {
			return nil, err
		}
		s.logAudit(ctx, user.ID, req.IPAddress, "auth.login.mfa_required", "", "success", "info")
		return &LoginResult{
			MFARequired:     true,
			MFASessionToken: mfaToken,
			UserID:          user.ID,
			Role:            user.Role,
		}, nil
	}

	// Kein MFA → direkt Session erstellen
	return s.createSession(ctx, user.ID, user.Email, user.Role, req.IPAddress, req.UserAgent)
}

// CreateSessionAfterMFA erstellt eine vollwertige Session nach erfolgreichem MFA.
func (s *AuthService) CreateSessionAfterMFA(ctx context.Context, userID, email, role, ip, userAgent string) (*LoginResult, error) {
	return s.createSession(ctx, userID, email, role, ip, userAgent)
}

// createSession erstellt eine DB-Session und gibt Token-Paar zurück.
func (s *AuthService) createSession(ctx context.Context, userID, email, role, ip, userAgent string) (*LoginResult, error) {
	sessionID := uuid.New().String()

	pair, err := auth.IssueTokenPair(s.tokenCfg, userID, email, role, sessionID)
	if err != nil {
		return nil, fmt.Errorf("service: token ausstellung fehlgeschlagen: %w", err)
	}

	userAgentSQL := sql.NullString{String: userAgent, Valid: userAgent != ""}

	_, err = s.queries.CreateSession(ctx, db.CreateSessionParams{
		ID:               sessionID,
		UserID:           userID,
		AccessTokenHash:  pair.AccessTokenHash,
		RefreshTokenHash: pair.RefreshTokenHash,
		IpAddress:        ip,
		UserAgent:        userAgentSQL,
		ExpiresAt:        pair.RefreshExpiresAt.Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("service: session erstellen fehlgeschlagen: %w", err)
	}

	s.logAudit(ctx, userID, ip, "auth.login.success", "", "success", "info")
	s.log.Info().Str("user_id", userID).Str("session_id", sessionID).Msg("login erfolgreich")

	return &LoginResult{
		AccessToken:      pair.AccessToken,
		RefreshToken:     pair.RefreshToken,
		AccessExpiresAt:  pair.AccessExpiresAt,
		RefreshExpiresAt: pair.RefreshExpiresAt,
		UserID:           userID,
		Role:             role,
	}, nil
}

// RefreshRequest enthält den Refresh-Token.
type RefreshRequest struct {
	RefreshToken string
	IPAddress    string
	UserAgent    string
}

// Refresh tauscht einen Refresh-Token gegen ein neues Token-Paar.
func (s *AuthService) Refresh(ctx context.Context, req RefreshRequest) (*LoginResult, error) {
	tokenHash := auth.HashTokenPublic(req.RefreshToken)

	session, err := s.queries.GetSessionByRefreshTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("service: session lookup fehlgeschlagen: %w", err)
	}

	// Session abgelaufen?
	expiresAt, err := time.Parse(time.RFC3339, session.ExpiresAt)
	if err != nil || time.Now().After(expiresAt) {
		return nil, ErrSessionNotFound
	}

	// User laden
	user, err := s.queries.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("service: user nicht gefunden: %w", err)
	}

	// Alte Session widerrufen (Token-Rotation)
	_ = s.queries.RevokeSession(ctx, session.ID)

	// Neue Session erstellen
	userAgent := req.UserAgent
	if session.UserAgent.Valid {
		userAgent = session.UserAgent.String
	}

	return s.createSession(ctx, user.ID, user.Email, user.Role, req.IPAddress, userAgent)
}

// Logout widerruft eine Session anhand des Access-Tokens.
func (s *AuthService) Logout(ctx context.Context, accessToken, ip string) error {
	tokenHash := auth.HashTokenPublic(accessToken)

	session, err := s.queries.GetSessionByAccessTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // Bereits ausgeloggt — kein Fehler
		}
		return fmt.Errorf("service: session lookup fehlgeschlagen: %w", err)
	}

	if err := s.queries.RevokeSession(ctx, session.ID); err != nil {
		return fmt.Errorf("service: session widerruf fehlgeschlagen: %w", err)
	}

	s.logAudit(ctx, session.UserID, ip, "auth.logout", "", "success", "info")
	s.log.Info().Str("session_id", session.ID).Msg("logout erfolgreich")
	return nil
}

// ListSessions gibt alle aktiven Sessions eines Users zurück.
func (s *AuthService) ListSessions(ctx context.Context, userID string) ([]db.Session, error) {
	return s.queries.ListActiveSessionsByUserID(ctx, userID)
}

// RevokeSession widerruft eine spezifische Session (remote logout).
func (s *AuthService) RevokeSession(ctx context.Context, sessionID, requestingUserID, ip string) error {
	// Sicherheit: User darf nur eigene Sessions widerrufen
	sessions, err := s.queries.ListActiveSessionsByUserID(ctx, requestingUserID)
	if err != nil {
		return fmt.Errorf("service: sessions laden fehlgeschlagen: %w", err)
	}

	found := false
	for _, sess := range sessions {
		if sess.ID == sessionID {
			found = true
			break
		}
	}

	if !found {
		return ErrSessionNotFound
	}

	if err := s.queries.RevokeSession(ctx, sessionID); err != nil {
		return fmt.Errorf("service: session widerruf fehlgeschlagen: %w", err)
	}

	s.logAudit(ctx, requestingUserID, ip, "auth.session.revoked", sessionID, "success", "info")
	return nil
}

// issueMFASessionToken erstellt einen kurzlebigen Token für den MFA-Schritt.
// Dies ist kein vollwertiges JWT — wird in v0.x durch TOTP-Implementation ersetzt.
func (s *AuthService) issueMFASessionToken(userID string) (string, error) {
	// Kurzlebiger MFA-Pending-Token (5 Minuten)
	cfg := auth.TokenConfig{
		Secret:        s.tokenCfg.Secret,
		AccessTTLMin:  5,
	}
	pair, err := auth.IssueTokenPair(cfg, userID, "", "mfa_pending", "mfa")
	if err != nil {
		return "", fmt.Errorf("service: mfa token fehlgeschlagen: %w", err)
	}
	return pair.AccessToken, nil
}

// logAudit schreibt einen Eintrag ins Audit-Log.
func (s *AuthService) logAudit(ctx context.Context, userID, ip, actionType, resource, result, severity string) {
	userIDSQL := sql.NullString{String: userID, Valid: userID != ""}
	resourceSQL := sql.NullString{String: resource, Valid: resource != ""}
	ipSQL := sql.NullString{String: ip, Valid: ip != ""}

	_, err := s.queries.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
		ID:         uuid.New().String(),
		UserID:     userIDSQL,
		ActionType: actionType,
		Resource:   resourceSQL,
		Result:     result,
		IpAddress:  ipSQL,
		Severity:   severity,
	})
	if err != nil {
		s.log.Error().Err(err).Str("action", actionType).Msg("audit log fehlgeschlagen")
	}
}
