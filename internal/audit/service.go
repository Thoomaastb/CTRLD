package audit

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rs/zerolog"

	database "github.com/Thoomaastb/CTRLD/internal/db"
	db "github.com/Thoomaastb/CTRLD/internal/db/generated"
)

// Service kapselt die Audit-Log-Abfrage-Logik.
type Service struct {
	db      *database.DB
	queries *db.Queries
	log     zerolog.Logger
}

// New erstellt einen neuen Audit-Service.
func New(d *database.DB, log zerolog.Logger) *Service {
	return &Service{
		db:      d,
		queries: db.New(d.SQL()),
		log:     log,
	}
}

// QueryParams enthält Filter-Parameter für Audit-Log-Abfragen.
type QueryParams struct {
	UserID     string
	Severity   string // info / warning / critical / "" (alle)
	ActionType string // z.B. "auth.login" / "" (alle)
	Page       int    // 1-basiert
	PerPage    int    // Default: 50, Max: 200
}

// AuditEntry ist die API-Repräsentation eines Audit-Log-Eintrags.
type AuditEntry struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
	PIMSessionID string `json:"pim_session_id,omitempty"`
	ActionType   string `json:"action_type"`
	Resource     string `json:"resource,omitempty"`
	Result       string `json:"result"`
	IPAddress    string `json:"ip_address,omitempty"`
	Severity     string `json:"severity"`
	CreatedAt    string `json:"created_at"`
	// IsPIMAction zeigt ob die Aktion während einer PIM-Sitzung stattfand
	IsPIMAction bool `json:"is_pim_action"`
}

// PagedResult enthält paginierte Audit-Log-Einträge.
type PagedResult struct {
	Entries  []AuditEntry `json:"entries"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PerPage  int          `json:"per_page"`
	HasMore  bool         `json:"has_more"`
}

// Query gibt paginierte Audit-Log-Einträge zurück.
func (s *Service) Query(ctx context.Context, params QueryParams) (*PagedResult, error) {
	// Defaults
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 {
		params.PerPage = 50
	}
	if params.PerPage > 200 {
		params.PerPage = 200
	}

	offset := int64((params.Page - 1) * params.PerPage)
	limit := int64(params.PerPage)

	var (
		rows []db.AuditLog
		err  error
	)

	// Filter anwenden
	switch {
	case params.UserID != "":
		rows, err = s.queries.ListAuditEntriesByUserID(ctx, db.ListAuditEntriesByUserIDParams{
			UserID: params.UserID,
			Limit:  limit,
			Offset: offset,
		})
	case params.Severity != "":
		rows, err = s.queries.ListAuditEntriesBySeverity(ctx, db.ListAuditEntriesBySeverityParams{
			Severity: params.Severity,
			Limit:    limit,
			Offset:   offset,
		})
	default:
		rows, err = s.queries.ListAuditEntries(ctx, db.ListAuditEntriesParams{
			Limit:  limit,
			Offset: offset,
		})
	}

	if err != nil {
		return nil, fmt.Errorf("audit: abfrage fehlgeschlagen: %w", err)
	}

	total, err := s.queries.CountAuditEntries(ctx)
	if err != nil {
		return nil, fmt.Errorf("audit: count fehlgeschlagen: %w", err)
	}

	entries := make([]AuditEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, toAuditEntry(row))
	}

	return &PagedResult{
		Entries: entries,
		Total:   total,
		Page:    params.Page,
		PerPage: params.PerPage,
		HasMore: offset+int64(len(rows)) < total,
	}, nil
}

// ExportCSV exportiert Audit-Einträge als CSV.
func (s *Service) ExportCSV(ctx context.Context, params QueryParams) (string, error) {
	params.PerPage = 200 // Max für Export
	result, err := s.Query(ctx, params)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	w := csv.NewWriter(&buf)

	// Header
	_ = w.Write([]string{
		"id", "created_at", "user_id", "action_type", "resource",
		"result", "severity", "ip_address", "is_pim_action", "pim_session_id",
	})

	for _, e := range result.Entries {
		isPIM := "false"
		if e.IsPIMAction {
			isPIM = "true"
		}
		_ = w.Write([]string{
			e.ID, e.CreatedAt, e.UserID, e.ActionType, e.Resource,
			e.Result, e.Severity, e.IPAddress, isPIM, e.PIMSessionID,
		})
	}

	w.Flush()
	return buf.String(), nil
}

// ExportJSON exportiert Audit-Einträge als JSON.
func (s *Service) ExportJSON(ctx context.Context, params QueryParams) (string, error) {
	params.PerPage = 200
	result, err := s.Query(ctx, params)
	if err != nil {
		return "", err
	}

	data, err := json.MarshalIndent(result.Entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("audit: json export fehlgeschlagen: %w", err)
	}

	return string(data), nil
}

// toAuditEntry konvertiert einen DB-Row in eine API-Repräsentation.
func toAuditEntry(row db.AuditLog) AuditEntry {
	e := AuditEntry{
		ID:          row.ID,
		ActionType:  row.ActionType,
		Result:      row.Result,
		Severity:    row.Severity,
		CreatedAt:   row.CreatedAt,
		IsPIMAction: row.PimSessionID.Valid && row.PimSessionID.String != "",
	}
	if row.UserID.Valid {
		e.UserID = row.UserID.String
	}
	if row.SessionID.Valid {
		e.SessionID = row.SessionID.String
	}
	if row.PimSessionID.Valid {
		e.PIMSessionID = row.PimSessionID.String
	}
	if row.Resource.Valid {
		e.Resource = row.Resource.String
	}
	if row.IpAddress.Valid {
		e.IPAddress = row.IpAddress.String
	}
	return e
}
