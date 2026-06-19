package setup

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
	ErrSetupAlreadyDone  = errors.New("setup: wizard bereits abgeschlossen")
	ErrSetupNotDone      = errors.New("setup: wizard noch nicht abgeschlossen")
	ErrInvalidEmail      = errors.New("setup: ungültige e-mail adresse")
	ErrPasswordTooShort  = errors.New("setup: passwort muss mindestens 12 zeichen haben")
	ErrAdminExists       = errors.New("setup: admin-account bereits vorhanden")
)

const (
	minPasswordLen     = 12
	configKeySetupDone = "setup.completed"
	configKeyAdminID   = "setup.admin_id"
)

// Status beschreibt den aktuellen Setup-Wizard-Zustand.
type Status struct {
	IsCompleted  bool   `json:"is_completed"`
	HasAdmin     bool   `json:"has_admin"`
	CurrentStep  int    `json:"current_step"`  // 1=Admin, 2=MFA, 3=Config, 4=Done
	AdminID      string `json:"admin_id,omitempty"`
}

// Service kapselt die Setup-Wizard-Logik.
type Service struct {
	db      *database.DB
	queries *db.Queries
	log     zerolog.Logger
}

// New erstellt einen neuen Setup-Service.
func New(d *database.DB, log zerolog.Logger) *Service {
	return &Service{
		db:      d,
		queries: db.New(d.SQL()),
		log:     log,
	}
}

// GetStatus gibt den aktuellen Setup-Status zurück.
func (s *Service) GetStatus(ctx context.Context) (*Status, error) {
	status := &Status{CurrentStep: 1}

	// Setup abgeschlossen?
	var val string
	err := s.db.SQL().QueryRowContext(ctx,
		"SELECT value FROM config WHERE key = ?", configKeySetupDone,
	).Scan(&val)

	if err == nil && val == "true" {
		status.IsCompleted = true
		status.CurrentStep = 4
	}

	// Admin vorhanden?
	users, err := s.queries.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("setup: user-check fehlgeschlagen: %w", err)
	}

	for _, u := range users {
		if u.Role == "admin" && u.IsActive == 1 {
			status.HasAdmin = true
			status.AdminID = u.ID
			if status.CurrentStep < 2 {
				status.CurrentStep = 2
			}
			break
		}
	}

	return status, nil
}

// IsCompleted prüft schnell ob der Setup-Wizard abgeschlossen ist.
// Wird als Middleware-Check verwendet.
func (s *Service) IsCompleted(ctx context.Context) bool {
	var val string
	err := s.db.SQL().QueryRowContext(ctx,
		"SELECT value FROM config WHERE key = ?", configKeySetupDone,
	).Scan(&val)
	return err == nil && val == "true"
}

// CreateAdminParams enthält die Parameter für die Admin-Erstellung.
type CreateAdminParams struct {
	Email    string
	Password string
}

// CreateAdminResult enthält das Ergebnis der Admin-Erstellung.
type CreateAdminResult struct {
	UserID string
	Email  string
}

// CreateAdmin erstellt den ersten Admin-Account.
// Nur möglich wenn noch kein Admin existiert.
func (s *Service) CreateAdmin(ctx context.Context, params CreateAdminParams) (*CreateAdminResult, error) {
	// Setup bereits abgeschlossen?
	if s.IsCompleted(ctx) {
		return nil, ErrSetupAlreadyDone
	}

	// Admin bereits vorhanden?
	users, err := s.queries.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("setup: user-check fehlgeschlagen: %w", err)
	}
	for _, u := range users {
		if u.Role == "admin" {
			return nil, ErrAdminExists
		}
	}

	// Validierung
	if !isValidEmail(params.Email) {
		return nil, ErrInvalidEmail
	}
	if len(params.Password) < minPasswordLen {
		return nil, ErrPasswordTooShort
	}

	// Passwort hashen
	hash, err := auth.HashPassword(params.Password)
	if err != nil {
		return nil, fmt.Errorf("setup: passwort hashing fehlgeschlagen: %w", err)
	}

	// Admin anlegen
	userID := uuid.New().String()
	_, err = s.queries.CreateUser(ctx, db.CreateUserParams{
		ID:           userID,
		Email:        strings.ToLower(strings.TrimSpace(params.Email)),
		PasswordHash: hash,
		Role:         "admin",
	})
	if err != nil {
		return nil, fmt.Errorf("setup: admin erstellen fehlgeschlagen: %w", err)
	}

	// Admin-ID in Config speichern
	s.setConfig(ctx, configKeyAdminID, userID, userID)

	s.log.Info().Str("user_id", userID).Str("email", params.Email).Msg("admin-account erstellt")

	return &CreateAdminResult{
		UserID: userID,
		Email:  strings.ToLower(strings.TrimSpace(params.Email)),
	}, nil
}

// Complete schließt den Setup-Wizard ab.
// Ab diesem Zeitpunkt ist der Panel nutzbar.
func (s *Service) Complete(ctx context.Context, adminID string) error {
	if s.IsCompleted(ctx) {
		return ErrSetupAlreadyDone
	}

	// Sicherstellen dass ein Admin existiert
	status, err := s.GetStatus(ctx)
	if err != nil {
		return err
	}
	if !status.HasAdmin {
		return errors.New("setup: kein admin-account vorhanden")
	}

	s.setConfig(ctx, configKeySetupDone, "true", adminID)

	s.log.Info().Str("admin_id", adminID).Msg("setup-wizard abgeschlossen")
	return nil
}

// setConfig schreibt einen Konfigurations-Wert.
func (s *Service) setConfig(ctx context.Context, key, value, updatedBy string) {
	updatedBySQL := sql.NullString{String: updatedBy, Valid: updatedBy != ""}
	_, _ = s.db.SQL().ExecContext(ctx,
		`INSERT INTO config (key, value, updated_at, updated_by)
		 VALUES (?, ?, CURRENT_TIMESTAMP, ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at, updated_by=excluded.updated_by`,
		key, value, updatedBySQL,
	)
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
