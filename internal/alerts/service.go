// Package alerts implementiert das Alert-System für CTRLD.
//
// Testfälle (Akzeptanzkriterien):
// - Alert wird ausgelöst wenn Metrik > Schwellwert für mindestens 2 aufeinanderfolgende Messungen
// - Alert löst sich automatisch auf wenn Metrik wieder unter Schwellwert fällt
// - Kein doppeltes Auslösen: aktiver Alert derselben Art blockiert neuen
// - Webhook sendet kontextreiche Nachricht (Wert, Schwellwert, Trend)
// - Nach Resolve wird Webhook erneut ausgelöst (resolved-Nachricht)
package alerts

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	database "github.com/Thoomaastb/CTRLD/internal/db"
	db "github.com/Thoomaastb/CTRLD/internal/db/generated"
	"github.com/Thoomaastb/CTRLD/internal/metrics"
)

// Threshold definiert einen konfigurierbaren Schwellwert.
type Threshold struct {
	Type     string  // cpu / ram / disk
	Warning  float64 // Prozent (0-100)
	Critical float64 // Prozent (0-100)
	Enabled  bool
}

// DefaultThresholds sind die Standard-Schwellwerte.
var DefaultThresholds = []Threshold{
	{Type: "cpu",  Warning: 80, Critical: 90, Enabled: true},
	{Type: "ram",  Warning: 85, Critical: 95, Enabled: true},
	{Type: "disk", Warning: 80, Critical: 90, Enabled: true},
}

// Alert repräsentiert einen ausgelösten Alert.
type Alert struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Severity     string    `json:"severity"` // warning / critical
	Threshold    float64   `json:"threshold"`
	CurrentValue float64   `json:"current_value"`
	Message      string    `json:"message"`
	TriggeredAt  time.Time `json:"triggered_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	Notified     bool      `json:"notified"`
}

// WebhookPayload ist das Format für externe Benachrichtigungen.
// Bewusst kontextreich — keine generischen "high CPU"-Messages.
type WebhookPayload struct {
	Event     string    `json:"event"`    // triggered / resolved
	Alert     Alert     `json:"alert"`
	Timestamp time.Time `json:"timestamp"`
	// Kontext für bessere Verständlichkeit
	Context   AlertContext `json:"context"`
}

// AlertContext enthält zusätzliche Informationen für aussagekräftige Meldungen.
type AlertContext struct {
	Hostname    string  `json:"hostname"`
	MetricLabel string  `json:"metric_label"`
	Value       string  `json:"value_formatted"`
	Threshold   string  `json:"threshold_formatted"`
	Trend       string  `json:"trend"` // "rising" / "falling" / "stable"
	LoadAvg1    float64 `json:"load_avg_1,omitempty"`
}

// Service verwaltet Alerts und prüft Metriken.
type Service struct {
	db         *database.DB
	queries    *db.Queries
	thresholds []Threshold
	webhookURL string
	hostname   string
	// consecutiveViolations zählt aufeinanderfolgende Überschreitungen
	// pro Metric-Type — verhindert Flapping bei kurzen Spitzen
	mu                   sync.Mutex
	consecutiveViolations map[string]int
	lastValues           map[string]float64
	log                  zerolog.Logger
}

// NewService erstellt einen neuen Alert-Service.
func NewService(d *database.DB, webhookURL, hostname string, log zerolog.Logger) *Service {
	return &Service{
		db:                    d,
		queries:               db.New(d.SQL()),
		thresholds:            DefaultThresholds,
		webhookURL:            webhookURL,
		hostname:              hostname,
		consecutiveViolations: make(map[string]int),
		lastValues:            make(map[string]float64),
		log:                   log,
	}
}

// Evaluate prüft einen Metriken-Snapshot gegen alle Schwellwerte.
// Wird vom Metrics-Service nach jeder Messung aufgerufen.
func (s *Service) Evaluate(ctx context.Context, snap *metrics.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// CPU prüfen
	s.evaluateMetric(ctx, snap, "cpu", snap.CPU.UsagePercent, "")

	// RAM prüfen
	s.evaluateMetric(ctx, snap, "ram", snap.RAM.UsagePercent, "")

	// Disk prüfen (alle Mounts)
	for _, disk := range snap.Disks {
		s.evaluateMetric(ctx, snap, "disk", disk.UsagePercent, disk.MountPoint)
	}
}

// evaluateMetric prüft einen einzelnen Metrik-Wert.
func (s *Service) evaluateMetric(ctx context.Context, snap *metrics.Snapshot, metricType string, value float64, resource string) {
	key := metricType
	if resource != "" {
		key = metricType + ":" + resource
	}

	threshold := s.thresholdFor(metricType)
	if threshold == nil || !threshold.Enabled {
		return
	}

	severity := ""
	alertThreshold := 0.0
	if value >= threshold.Critical {
		severity = "critical"
		alertThreshold = threshold.Critical
	} else if value >= threshold.Warning {
		severity = "warning"
		alertThreshold = threshold.Warning
	}

	if severity != "" {
		// Aufeinanderfolgende Überschreitungen zählen (Anti-Flapping: min. 2)
		s.consecutiveViolations[key]++
		if s.consecutiveViolations[key] < 2 {
			s.lastValues[key] = value
			return
		}

		// Aktiven Alert prüfen — kein doppeltes Auslösen
		existing := s.findActiveAlert(ctx, metricType, resource)
		if existing == nil {
			s.triggerAlert(ctx, snap, metricType, resource, severity, alertThreshold, value)
		}
	} else {
		// Unter Schwellwert — Counter zurücksetzen + Alert auflösen
		s.consecutiveViolations[key] = 0

		if active := s.findActiveAlert(ctx, metricType, resource); active != nil {
			s.resolveAlert(ctx, snap, *active, value)
		}
	}

	s.lastValues[key] = value
}

// triggerAlert erstellt einen neuen Alert in der DB und sendet Webhook.
func (s *Service) triggerAlert(ctx context.Context, snap *metrics.Snapshot, alertType, resource, severity string, threshold, value float64) {
	msg := s.formatMessage(alertType, resource, severity, value, threshold, snap)

	row, err := s.queries.CreateAlert(ctx, db.CreateAlertParams{
		ID:           uuid.New().String(),
		Type:         alertType,
		Threshold:    threshold,
		CurrentValue: value,
		Severity:     severity,
		Resource:     sql.NullString{String: resource, Valid: resource != ""},
	})
	if err != nil {
		s.log.Error().Err(err).Str("type", alertType).Msg("alert erstellen fehlgeschlagen")
		return
	}

	alert := Alert{
		ID:           row.ID,
		Type:         alertType,
		Severity:     severity,
		Threshold:    threshold,
		CurrentValue: value,
		Message:      msg,
		TriggeredAt:  time.Now(),
	}

	s.log.Warn().
		Str("type", alertType).
		Str("severity", severity).
		Float64("value", value).
		Float64("threshold", threshold).
		Msg("alert ausgelöst: " + msg)

	go s.sendWebhook(alert, "triggered", snap)
}

// resolveAlert löst einen aktiven Alert auf.
func (s *Service) resolveAlert(ctx context.Context, snap *metrics.Snapshot, alertID string, currentValue float64) {
	if err := s.queries.ResolveAlert(ctx, alertID); err != nil {
		s.log.Error().Err(err).Str("id", alertID).Msg("alert auflösen fehlgeschlagen")
		return
	}

	now := time.Now()
	alert := Alert{
		ID:          alertID,
		ResolvedAt:  &now,
		CurrentValue: currentValue,
	}

	s.log.Info().Str("id", alertID).Float64("value", currentValue).Msg("alert aufgelöst")
	go s.sendWebhook(alert, "resolved", snap)
}

// findActiveAlert sucht nach einem aktiven Alert in der DB.
func (s *Service) findActiveAlert(ctx context.Context, alertType, resource string) *string {
	// Direkte DB-Abfrage für aktive Alerts
	var id string
	query := `SELECT id FROM alerts WHERE type = ? AND resolved_at IS NULL LIMIT 1`
	err := s.db.SQL().QueryRowContext(ctx, query, alertType).Scan(&id)
	if err != nil {
		return nil
	}
	return &id
}

// formatMessage erstellt eine kontextreiche Alert-Nachricht.
func (s *Service) formatMessage(alertType, resource, severity string, value, threshold float64, snap *metrics.Snapshot) string {
	labels := map[string]string{
		"cpu":  "CPU-Auslastung",
		"ram":  "RAM-Auslastung",
		"disk": "Disk-Auslastung",
	}
	label := labels[alertType]
	if resource != "" {
		label += " (" + resource + ")"
	}

	base := fmt.Sprintf("%s bei %.1f%% (Schwellwert: %.0f%%)", label, value, threshold)

	// Zusätzlicher Kontext je nach Typ
	switch alertType {
	case "cpu":
		if snap != nil {
			base += fmt.Sprintf(" — Load: %.2f/%.2f/%.2f",
				snap.LoadAvg.Load1, snap.LoadAvg.Load5, snap.LoadAvg.Load15)
		}
	case "ram":
		if snap != nil {
			base += fmt.Sprintf(" — Verfügbar: %s",
				formatBytesShort(snap.RAM.AvailableBytes))
		}
	case "disk":
		// Resource ist bereits im Label
	}

	return base
}

// sendWebhook sendet eine Benachrichtigung an den konfigurierten Webhook.
func (s *Service) sendWebhook(alert Alert, event string, snap *metrics.Snapshot) {
	if s.webhookURL == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	trend := "stable"
	key := alert.Type
	if last, ok := s.lastValues[key]; ok {
		if alert.CurrentValue > last+1 {
			trend = "rising"
		} else if alert.CurrentValue < last-1 {
			trend = "falling"
		}
	}

	alertCtx := AlertContext{
		Hostname:    s.hostname,
		MetricLabel: alert.Type,
		Value:       fmt.Sprintf("%.1f%%", alert.CurrentValue),
		Threshold:   fmt.Sprintf("%.0f%%", alert.Threshold),
		Trend:       trend,
	}
	if snap != nil {
		alertCtx.LoadAvg1 = snap.LoadAvg.Load1
	}

	payload := WebhookPayload{
		Event:     event,
		Alert:     alert,
		Timestamp: time.Now(),
		Context:   alertCtx,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CTRLD-Alert/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.log.Warn().Err(err).Str("url", s.webhookURL).Msg("webhook fehlgeschlagen")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		s.log.Warn().Int("status", resp.StatusCode).Str("url", s.webhookURL).Msg("webhook nicht-200 antwort")
	}
}

// ListActive gibt alle aktiven Alerts zurück.
func (s *Service) ListActive(ctx context.Context) ([]Alert, error) {
	return s.listAlerts(ctx, false)
}

// ListAll gibt alle Alerts (inkl. resolved) zurück.
func (s *Service) ListAll(ctx context.Context, limit int) ([]Alert, error) {
	return s.listAlerts(ctx, true)
}

func (s *Service) listAlerts(ctx context.Context, includeResolved bool) ([]Alert, error) {
	query := `
		SELECT id, type, threshold, current_value, severity,
		       triggered_at, resolved_at, notified,
		       COALESCE(resource, '') as resource
		FROM alerts
		ORDER BY triggered_at DESC
		LIMIT 100`

	if !includeResolved {
		query = `
			SELECT id, type, threshold, current_value, severity,
			       triggered_at, resolved_at, notified,
			       COALESCE(resource, '') as resource
			FROM alerts
			WHERE resolved_at IS NULL
			ORDER BY triggered_at DESC`
	}

	rows, err := s.db.SQL().QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []Alert
	for rows.Next() {
		var a Alert
		var resolvedAt sql.NullString
		var triggeredAtStr string
		var resource string

		err := rows.Scan(&a.ID, &a.Type, &a.Threshold, &a.CurrentValue,
			&a.Severity, &triggeredAtStr, &resolvedAt, &a.Notified, &resource)
		if err != nil {
			continue
		}

		a.TriggeredAt, _ = time.Parse(time.RFC3339, triggeredAtStr)
		if resolvedAt.Valid {
			t, _ := time.Parse(time.RFC3339, resolvedAt.String)
			a.ResolvedAt = &t
		}
		if resource != "" {
			a.Message = resource
		}

		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// UpdateThresholds aktualisiert die Schwellwerte zur Laufzeit.
func (s *Service) UpdateThresholds(thresholds []Threshold) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.thresholds = thresholds
}

// GetThresholds gibt die aktuellen Schwellwerte zurück.
func (s *Service) GetThresholds() []Threshold {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.thresholds
}

// thresholdFor gibt den Schwellwert für einen Metric-Type zurück.
func (s *Service) thresholdFor(metricType string) *Threshold {
	for i := range s.thresholds {
		if s.thresholds[i].Type == metricType {
			return &s.thresholds[i]
		}
	}
	return nil
}

func formatBytesShort(bytes uint64) string {
	const gb = 1024 * 1024 * 1024
	const mb = 1024 * 1024
	if bytes >= gb {
		return fmt.Sprintf("%.1f GB", float64(bytes)/gb)
	}
	return fmt.Sprintf("%.0f MB", float64(bytes)/mb)
}
