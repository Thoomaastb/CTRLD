package pim

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	database "github.com/Thoomaastb/CTRLD/internal/db"
	db "github.com/Thoomaastb/CTRLD/internal/db/generated"
)

var (
	ErrPIMAlreadyActive  = errors.New("pim: bereits eine aktive pim-sitzung vorhanden")
	ErrPIMNotActive      = errors.New("pim: keine aktive pim-sitzung")
	ErrPIMNotFound       = errors.New("pim: pim-sitzung nicht gefunden")
	ErrReasonTooShort    = errors.New("pim: begründung muss mindestens 10 zeichen haben")
	ErrInvalidDuration   = errors.New("pim: ungültige dauer — erlaubt: 15, 30, 60 oder 1-480 minuten")
)

// Erlaubte Standarddauern in Minuten
var allowedDurations = map[int]bool{15: true, 30: true, 60: true}

const (
	minReason        = 10  // Mindestlänge Begründung
	maxDurationMin   = 480 // Max. 8 Stunden Custom-Dauer
)

// Service kapselt die PIM-Logik.
type Service struct {
	db      *database.DB
	queries *db.Queries
	log     zerolog.Logger
}

// New erstellt einen neuen PIM-Service.
func New(d *database.DB, log zerolog.Logger) *Service {
	return &Service{
		db:      d,
		queries: db.New(d.SQL()),
		log:     log,
	}
}

// RequestParams enthält die Parameter für eine PIM-Anfrage.
type RequestParams struct {
	UserID      string
	SessionID   string
	Reason      string
	DurationMin int
	IsBreakGlass bool
}

// ActivePIMSession enthält Infos zur aktiven PIM-Sitzung.
type ActivePIMSession struct {
	ID                   string
	Reason               string
	RequestedDurationMin int
	StartedAt            time.Time
	ExpiresAt            time.Time
	IsBreakGlass         bool
	ActionCount          int
	RemainingSeconds     int
}

// Request erstellt eine neue PIM-Sitzung.
// Erfordert frische MFA-Verifikation (wird vom Handler sichergestellt).
func (s *Service) Request(ctx context.Context, params RequestParams) (*ActivePIMSession, error) {
	// Validierung
	if len(params.Reason) < minReason {
		return nil, ErrReasonTooShort
	}

	if !isValidDuration(params.DurationMin) {
		return nil, ErrInvalidDuration
	}

	// Prüfen ob bereits eine aktive PIM-Sitzung läuft
	existing, err := s.queries.GetActivePIMSessionByUserID(ctx, params.UserID)
	if err == nil && existing.ID != "" {
		return nil, ErrPIMAlreadyActive
	}

	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(params.DurationMin) * time.Minute)

	breakGlass := int64(0)
	if params.IsBreakGlass {
		breakGlass = 1
	}

	session, err := s.queries.CreatePIMSession(ctx, db.CreatePIMSessionParams{
		ID:                   uuid.New().String(),
		UserID:               params.UserID,
		SessionID:            params.SessionID,
		Reason:               params.Reason,
		RequestedDurationMin: int64(params.DurationMin),
		ExpiresAt:            expiresAt.Format(time.RFC3339),
		IsBreakGlass:         breakGlass,
	})
	if err != nil {
		return nil, fmt.Errorf("pim: sitzung erstellen fehlgeschlagen: %w", err)
	}

	s.logAudit(ctx, params.UserID, params.SessionID, session.ID,
		"pim.session.started", params.Reason, "success",
		map[string]interface{}{
			"duration_min":   params.DurationMin,
			"is_break_glass": params.IsBreakGlass,
		},
	)

	if params.IsBreakGlass {
		s.log.Warn().
			Str("user_id", params.UserID).
			Str("pim_id", session.ID).
			Msg("BREAK-GLASS PIM aktiviert")
	} else {
		s.log.Info().
			Str("user_id", params.UserID).
			Str("pim_id", session.ID).
			Int("duration_min", params.DurationMin).
			Msg("PIM-Sitzung gestartet")
	}

	return toActivePIMSession(session), nil
}

// GetActive gibt die aktive PIM-Sitzung eines Users zurück.
func (s *Service) GetActive(ctx context.Context, userID string) (*ActivePIMSession, error) {
	session, err := s.queries.GetActivePIMSessionByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPIMNotActive
		}
		return nil, fmt.Errorf("pim: aktive sitzung laden fehlgeschlagen: %w", err)
	}

	return toActivePIMSession(session), nil
}

// End beendet die aktive PIM-Sitzung manuell.
func (s *Service) End(ctx context.Context, userID, sessionID, pimID string) error {
	// Sicherheit: nur eigene PIM-Sitzung beenden
	active, err := s.GetActive(ctx, userID)
	if err != nil {
		return ErrPIMNotActive
	}
	if active.ID != pimID {
		return ErrPIMNotFound
	}

	if err := s.queries.EndPIMSession(ctx, pimID); err != nil {
		return fmt.Errorf("pim: sitzung beenden fehlgeschlagen: %w", err)
	}

	s.logAudit(ctx, userID, sessionID, pimID,
		"pim.session.ended", "manuell beendet", "success", nil,
	)

	s.log.Info().
		Str("user_id", userID).
		Str("pim_id", pimID).
		Msg("PIM-Sitzung beendet")

	return nil
}

// RecordAction inkrementiert den Action-Counter einer PIM-Sitzung.
// Wird bei jeder privilegierten Aktion aufgerufen.
func (s *Service) RecordAction(ctx context.Context, pimID string) error {
	return s.queries.IncrementPIMActionCount(ctx, pimID)
}

// ExpireOverdue läuft periodisch und beendet abgelaufene PIM-Sitzungen.
func (s *Service) ExpireOverdue(ctx context.Context) error {
	_, err := s.db.SQL().ExecContext(ctx,
		"UPDATE pim_sessions SET ended_at = CURRENT_TIMESTAMP WHERE ended_at IS NULL AND expires_at <= CURRENT_TIMESTAMP",
	)
	return err
}

// CheckAndRecord prüft ob eine aktive PIM-Sitzung vorhanden ist
// und inkrementiert den Action-Counter. Gibt die PIM-Session-ID zurück.
func (s *Service) CheckAndRecord(ctx context.Context, userID string) (string, error) {
	active, err := s.GetActive(ctx, userID)
	if err != nil {
		return "", ErrPIMNotActive
	}

	if err := s.RecordAction(ctx, active.ID); err != nil {
		s.log.Error().Err(err).Str("pim_id", active.ID).Msg("action count fehler")
	}

	return active.ID, nil
}

// ── Hilfsfunktionen ───────────────────────────────────────────────────────────

func toActivePIMSession(p db.PimSession) *ActivePIMSession {
	startedAt, _ := time.Parse(time.RFC3339, p.StartedAt)
	expiresAt, _ := time.Parse(time.RFC3339, p.ExpiresAt)
	remaining := int(time.Until(expiresAt).Seconds())
	if remaining < 0 {
		remaining = 0
	}

	return &ActivePIMSession{
		ID:                   p.ID,
		Reason:               p.Reason,
		RequestedDurationMin: int(p.RequestedDurationMin),
		StartedAt:            startedAt,
		ExpiresAt:            expiresAt,
		IsBreakGlass:         p.IsBreakGlass == 1,
		ActionCount:          int(p.ActionCount),
		RemainingSeconds:     remaining,
	}
}

func isValidDuration(min int) bool {
	if allowedDurations[min] {
		return true
	}
	// Custom-Dauer: 1-480 Minuten
	return min >= 1 && min <= maxDurationMin
}

func (s *Service) logAudit(ctx context.Context, userID, sessionID, pimID, actionType, resource, result string, metadata map[string]interface{}) {
	userIDSQL := sql.NullString{String: userID, Valid: userID != ""}
	sessionIDSQL := sql.NullString{String: sessionID, Valid: sessionID != ""}
	pimIDSQL := sql.NullString{String: pimID, Valid: pimID != ""}
	resourceSQL := sql.NullString{String: resource, Valid: resource != ""}

	severity := "info"
	if pimID != "" {
		severity = "warning"
	}

	_, err := s.queries.CreateAuditEntry(ctx, db.CreateAuditEntryParams{
		ID:           uuid.New().String(),
		UserID:       userIDSQL,
		SessionID:    sessionIDSQL,
		PimSessionID: pimIDSQL,
		ActionType:   actionType,
		Resource:     resourceSQL,
		Result:       result,
		Severity:     severity,
	})
	if err != nil {
		s.log.Error().Err(err).Str("action", actionType).Msg("audit log fehlgeschlagen")
	}
}
