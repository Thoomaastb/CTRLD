// Package logs liest System-Logs aus journald und Log-Dateien.
package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Entry repräsentiert einen einzelnen Log-Eintrag.
type Entry struct {
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"` // error / warning / info / debug
	Unit      string    `json:"unit"`
	Message   string    `json:"message"`
	Source    string    `json:"source"` // journald / syslog / auth
	PID       string    `json:"pid,omitempty"`
}

// Source definiert eine Log-Quelle.
type Source struct {
	ID    string // journald / syslog / auth
	Label string
	Path  string // leer = journald
}

// AvailableSources gibt alle verfügbaren Log-Quellen zurück.
func AvailableSources() []Source {
	sources := []Source{
		{ID: "journald", Label: "systemd Journal"},
	}

	// Datei-basierte Quellen prüfen
	fileSources := []Source{
		{ID: "syslog",   Label: "syslog",   Path: "/var/log/syslog"},
		{ID: "auth",     Label: "auth.log", Path: "/var/log/auth.log"},
		{ID: "kern",     Label: "kern.log", Path: "/var/log/kern.log"},
		{ID: "messages", Label: "messages", Path: "/var/log/messages"},
	}

	for _, s := range fileSources {
		if _, err := os.Stat(s.Path); err == nil {
			sources = append(sources, s)
		}
	}

	return sources
}

// QueryParams enthält Filter-Parameter für Log-Abfragen.
type QueryParams struct {
	Source   string // journald / syslog / auth / ...
	Unit     string // systemd Unit (nur journald)
	Severity string // error / warning / info / debug / "" (alle)
	Search   string // Freitext-Suche
	Since    time.Time
	Until    time.Time
	Lines    int // Default: 100, Max: 500
}

// Read liest Log-Einträge anhand der QueryParams.
func Read(ctx context.Context, params QueryParams) ([]Entry, error) {
	if params.Lines <= 0 {
		params.Lines = 100
	}
	if params.Lines > 500 {
		params.Lines = 500
	}

	switch params.Source {
	case "journald", "":
		return readJournald(ctx, params)
	default:
		return readLogFile(ctx, params)
	}
}

// ── journald ──────────────────────────────────────────────────────────────────

// readJournald liest aus systemd-journald via journalctl.
func readJournald(ctx context.Context, params QueryParams) ([]Entry, error) {
	args := []string{
		"--output=json",
		"--no-pager",
		fmt.Sprintf("--lines=%d", params.Lines),
	}

	if params.Unit != "" {
		args = append(args, fmt.Sprintf("--unit=%s", params.Unit))
	}

	if !params.Since.IsZero() {
		args = append(args, fmt.Sprintf("--since=%s", params.Since.Format("2006-01-02 15:04:05")))
	}

	if !params.Until.IsZero() {
		args = append(args, fmt.Sprintf("--until=%s", params.Until.Format("2006-01-02 15:04:05")))
	}

	// Severity-Filter via journald Priority
	if params.Severity != "" {
		priority := severityToPriority(params.Severity)
		if priority != "" {
			args = append(args, fmt.Sprintf("--priority=%s", priority))
		}
	}

	cmd := exec.CommandContext(ctx, "journalctl", args...)
	out, err := cmd.Output()
	if err != nil {
		// journalctl nicht verfügbar (Windows/Dev) → leere Liste, kein Fehler
		if isNotFound(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("logs: journalctl fehlgeschlagen: %w", err)
	}

	entries, err := parseJournaldJSON(string(out))
	if err != nil {
		return nil, err
	}

	// Freitext-Filter anwenden
	if params.Search != "" {
		entries = filterBySearch(entries, params.Search)
	}

	return entries, nil
}

// journaldEntry ist das JSON-Format von journalctl --output=json.
type journaldEntry struct {
	RealtimeTimestamp string `json:"__REALTIME_TIMESTAMP"`
	Priority          string `json:"PRIORITY"`
	SyslogIdentifier  string `json:"SYSLOG_IDENTIFIER"`
	SystemdUnit       string `json:"_SYSTEMD_UNIT"`
	Message           string `json:"MESSAGE"`
	PID               string `json:"_PID"`
}

func parseJournaldJSON(output string) ([]Entry, error) {
	var entries []Entry
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var raw journaldEntry
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		ts := parseJournaldTimestamp(raw.RealtimeTimestamp)
		unit := raw.SystemdUnit
		if unit == "" {
			unit = raw.SyslogIdentifier
		}

		// Message kann []byte sein in manchen journald-Versionen
		msg := raw.Message

		entries = append(entries, Entry{
			Timestamp: ts,
			Severity:  priorityToSeverity(raw.Priority),
			Unit:      unit,
			Message:   msg,
			Source:    "journald",
			PID:       raw.PID,
		})
	}

	return entries, scanner.Err()
}

// parseJournaldTimestamp parst den journald REALTIME_TIMESTAMP (Mikrosekunden seit Epoch).
func parseJournaldTimestamp(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	// Mikrosekunden → Nanosekunden
	var micros int64
	if _, err := fmt.Sscanf(s, "%d", &micros); err != nil {
		return time.Now()
	}
	return time.Unix(micros/1e6, (micros%1e6)*1000).UTC()
}

// ── Log-Dateien ───────────────────────────────────────────────────────────────

func readLogFile(ctx context.Context, params QueryParams) ([]Entry, error) {
	// Quelle → Dateipfad
	var path string
	for _, s := range AvailableSources() {
		if s.ID == params.Source {
			path = s.Path
			break
		}
	}
	if path == "" {
		return nil, fmt.Errorf("logs: unbekannte quelle: %s", params.Source)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("logs: datei öffnen fehlgeschlagen: %w", err)
	}
	defer f.Close()

	var all []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}

	// Letzte N Zeilen
	start := len(all) - params.Lines
	if start < 0 {
		start = 0
	}
	lines := all[start:]

	var entries []Entry
	for _, line := range lines {
		entry := parseSyslogLine(line, params.Source)
		if entry == nil {
			continue
		}

		// Filter
		if params.Severity != "" && entry.Severity != params.Severity {
			continue
		}
		if params.Search != "" && !strings.Contains(strings.ToLower(entry.Message), strings.ToLower(params.Search)) {
			continue
		}

		entries = append(entries, *entry)
	}

	return entries, nil
}

// parseSyslogLine parst eine Standard-Syslog-Zeile.
// Format: "Jun 22 10:30:00 hostname unit[pid]: message"
func parseSyslogLine(line, source string) *Entry {
	if len(line) < 15 {
		return nil
	}

	// Timestamp (erste 15 Zeichen: "Jun 22 10:30:00")
	tsStr := line[:15]
	ts, err := time.Parse("Jan  2 15:04:05", tsStr)
	if err != nil {
		ts, err = time.Parse("Jan 02 15:04:05", tsStr)
		if err != nil {
			ts = time.Now()
		}
	}
	// Jahr ergänzen
	ts = ts.AddDate(time.Now().Year(), 0, 0)

	rest := line[15:]
	// Unit aus "hostname unit[pid]:" extrahieren
	unit := ""
	pid := ""
	parts := strings.Fields(rest)
	if len(parts) >= 2 {
		unitPID := parts[1]
		if idx := strings.Index(unitPID, "["); idx > 0 {
			unit = unitPID[:idx]
			pid = strings.Trim(unitPID[idx:], "[]:")
		} else {
			unit = strings.TrimSuffix(unitPID, ":")
		}
	}

	// Message
	msg := rest
	if colonIdx := strings.Index(rest, "]: "); colonIdx > 0 {
		msg = rest[colonIdx+3:]
	} else if colonIdx := strings.Index(rest, ": "); colonIdx > 0 {
		msg = rest[colonIdx+2:]
	}

	severity := "info"
	msgLower := strings.ToLower(msg)
	if strings.Contains(msgLower, "error") || strings.Contains(msgLower, "failed") || strings.Contains(msgLower, "fatal") {
		severity = "error"
	} else if strings.Contains(msgLower, "warn") {
		severity = "warning"
	} else if strings.Contains(msgLower, "debug") {
		severity = "debug"
	}

	return &Entry{
		Timestamp: ts,
		Severity:  severity,
		Unit:      unit,
		Message:   strings.TrimSpace(msg),
		Source:    source,
		PID:       pid,
	}
}

// ── Hilfsfunktionen ───────────────────────────────────────────────────────────

func severityToPriority(severity string) string {
	m := map[string]string{
		"error":   "err",
		"warning": "warning",
		"info":    "info",
		"debug":   "debug",
	}
	return m[severity]
}

func priorityToSeverity(priority string) string {
	m := map[string]string{
		"0": "error", "1": "error", "2": "error", "3": "error",
		"4": "warning",
		"5": "info", "6": "info",
		"7": "debug",
	}
	if s, ok := m[priority]; ok {
		return s
	}
	return "info"
}

func filterBySearch(entries []Entry, search string) []Entry {
	lower := strings.ToLower(search)
	var result []Entry
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.Message), lower) ||
			strings.Contains(strings.ToLower(e.Unit), lower) {
			result = append(result, e)
		}
	}
	return result
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "executable file not found")
}
