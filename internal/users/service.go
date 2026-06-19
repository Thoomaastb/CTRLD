package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/auth"
	database "github.com/Thoomaastb/CTRLD/internal/db"
	db "github.com/Thoomaastb/CTRLD/internal/db/generated"
)

var (
	ErrUserNotFound     = errors.New("users: user nicht gefunden")
	ErrEmailTaken       = errors.New("users: e-mail bereits vergeben")
	ErrInvalidRole      = errors.New("users: ungültige rolle — erlaubt: admin, viewer")
	ErrPasswordTooShort = errors.New("users: passwort muss mindestens 12 zeichen haben")
	ErrInvalidEmail     = errors.New("users: ungültige e-mail adresse")
	ErrCannotDeleteSelf = errors.New("users: eigener account kann nicht gelöscht werden")
	ErrLastAdmin        = errors.New("users: letzter admin kann nicht deaktiviert werden")
)

const minPasswordLen = 12

var validRoles = map[string]bool{"admin": true, "viewer": true}

// Service kapselt die Benutzerverwaltungs-Logik.
type Service struct {
	db      *database.DB
	queries *db.Queries
	log     zerolog.Logger
}

// New erstellt einen neuen User-Service.
func New(d *database.DB, log zerolog.Logger) *Service {
	return &Service{
		db:      d,
		queries: db.New(d.SQL()),
		log:     log,
	}
}

// User ist die API-Repräsentation eines Users (ohne Passwort-Hash).
type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	LastLoginAt string `json:"last_login_at,omitempty"`
	HasMFA      bool   `json:"has_mfa"`
}

// List gibt alle User zurück.
func (s *Service) List(ctx context.Context) ([]User, error) {
	rows, err := s.queries.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("users: list fehlgeschlagen: %w", err)
	}

	users := make([]User, 0, len(rows))
	for _, row := range rows {
		users = append(users, toUser(row))
	}
	return users, nil
}

// Get gibt einen einzelnen User zurück.
func (s *Service) Get(ctx context.Context, userID string) (*User, error) {
	row, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("users: get fehlgeschlagen: %w", err)
	}

	u := toUser(row)

	// MFA-Status prüfen
	creds, err := s.queries.ListMFACredentialsByUserID(ctx, userID)
	if err == nil {
		u.HasMFA = len(creds) > 0
	}

	return &u, nil
}

// CreateParams enthält die Parameter für die User-Erstellung.
type CreateParams struct {
	Email       string
	Password    string
	Role        string
	RequestorID string // Wer erstellt den User (für Audit)
}

// Create erstellt einen neuen User.
// Erfordert aktive PIM-Sitzung (wird vom Handler sichergestellt).
func (s *Service) Create(ctx context.Context, params CreateParams) (*User, error) {
	// Validierung
	if !isValidEmail(params.Email) {
		return nil, ErrInvalidEmail
	}
	if len(params.Password) < minPasswordLen {
		return nil, ErrPasswordTooShort
	}
	if !validRoles[params.Role] {
		return nil, ErrInvalidRole
	}

	// E-Mail-Duplikat prüfen
	if _, err := s.queries.GetUserByEmail(ctx, strings.ToLower(params.Email)); err == nil {
		return nil, ErrEmailTaken
	}

	hash, err := auth.HashPassword(params.Password)
	if err != nil {
		return nil, fmt.Errorf("users: passwort hashing fehlgeschlagen: %w", err)
	}

	row, err := s.queries.CreateUser(ctx, db.CreateUserParams{
		ID:           uuid.New().String(),
		Email:        strings.ToLower(strings.TrimSpace(params.Email)),
		PasswordHash: hash,
		Role:         params.Role,
	})
	if err != nil {
		return nil, fmt.Errorf("users: erstellen fehlgeschlagen: %w", err)
	}

	s.logAudit(ctx, params.RequestorID, "user.created", row.ID, "success")
	s.log.Info().Str("user_id", row.ID).Str("email", row.Email).Str("role", row.Role).Msg("user erstellt")

	u := toUser(row)
	return &u, nil
}

// UpdateRoleParams enthält die Parameter für die Rollen-Änderung.
type UpdateRoleParams struct {
	UserID      string
	NewRole     string
	RequestorID string
}

// UpdateRole ändert die Rolle eines Users.
func (s *Service) UpdateRole(ctx context.Context, params UpdateRoleParams) (*User, error) {
	if !validRoles[params.NewRole] {
		return nil, ErrInvalidRole
	}

	// Prüfen ob letzter Admin degradiert werden soll
	if params.NewRole == "viewer" {
		if err := s.checkLastAdmin(ctx, params.UserID); err != nil {
			return nil, err
		}
	}

	row, err := s.queries.UpdateUserRole(ctx, db.UpdateUserRoleParams{
		Role: params.NewRole,
		ID:   params.UserID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("users: rolle ändern fehlgeschlagen: %w", err)
	}

	s.logAudit(ctx, params.RequestorID, "user.role_changed", params.UserID, "success")
	s.log.Info().Str("user_id", params.UserID).Str("new_role", params.NewRole).Msg("user rolle geändert")

	u := toUser(row)
	return &u, nil
}

// Deactivate deaktiviert einen User-Account (kein echtes Löschen).
func (s *Service) Deactivate(ctx context.Context, userID, requestorID string) error {
	// Nicht den eigenen Account deaktivieren
	if userID == requestorID {
		return ErrCannotDeleteSelf
	}

	// Letzten Admin schützen
	if err := s.checkLastAdmin(ctx, userID); err != nil {
		return err
	}

	if err := s.queries.DeactivateUser(ctx, userID); err != nil {
		return fmt.Errorf("users: deaktivieren fehlgeschlagen: %w", err)
	}

	// Alle Sessions des Users widerrufen
	if err := s.queries.RevokeAllUserSessions(ctx, userID); err != nil {
		s.log.Error().Err(err).Str("user_id", userID).Msg("sessions widerrufen fehlgeschlagen")
	}

	s.logAudit(ctx, requestorID, "user.deactivated", userID, "success")
	s.log.Info().Str("user_id", userID).Msg("user deaktiviert")

	return nil
}

// checkLastAdmin verhindert dass der letzte Admin-Account entfernt/degradiert wird.
func (s *Service) checkLastAdmin(ctx context.Context, userID string) error {
	// Ist der User überhaupt Admin?
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}
	if user.Role != "admin" {
		return nil // Kein Admin → kein Problem
	}

	// Alle aktiven Admins zählen
	users, err := s.queries.ListUsers(ctx)
	if err != nil {
		return fmt.Errorf("users: admin-check fehlgeschlagen: %w", err)
	}

	adminCount := 0
	for _, u := range users {
		if u.Role == "admin" && u.IsActive == 1 {
			adminCount++
		}
	}

	if adminCount <= 1 {
		return ErrLastAdmin
	}
	return nil
}

// logAudit schreibt einen Audit-Log-Eintrag.
func (s *Service) logAudit(ctx context.Context, requestorID, actionType, resource, result string) {
	userIDSQL := sql.NullString{String: requestorID, Valid: requestorID != ""}
	resourceSQL := sql.NullString{String: resource, Valid: resource != ""}

	_, err := s.queries.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
		ID:         uuid.New().String(),
		UserID:     userIDSQL,
		ActionType: actionType,
		Resource:   resourceSQL,
		Result:     result,
		Severity:   "info",
	})
	if err != nil {
		s.log.Error().Err(err).Str("action", actionType).Msg("audit log fehlgeschlagen")
	}
}

// toUser konvertiert einen DB-Row in eine API-Repräsentation.
func toUser(row db.User) User {
	u := User{
		ID:        row.ID,
		Email:     row.Email,
		Role:      row.Role,
		IsActive:  row.IsActive == 1,
		CreatedAt: row.CreatedAt,
	}
	if row.LastLoginAt.Valid {
		u.LastLoginAt = row.LastLoginAt.String
	}
	return u
}

// isValidEmail prüft das grundlegende E-Mail-Format.
func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	at := strings.LastIndex(email, "@")
	if at < 1 {
		return false
	}
	domain := email[at+1:]
	return strings.Contains(domain, ".") && len(domain) > 2
}
