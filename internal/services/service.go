// Package services verwaltet systemd-Services via systemctl.
//
// Testfälle (Akzeptanzkriterien):
// - List gibt alle Services zurück inkl. Status
// - Kritische Services (ssh, ctrld) können nicht gestoppt werden
// - Start/Stop/Restart erfordert aktive PIM-Sitzung
// - Enable/Disable erfordert aktive PIM-Sitzung
// - Windows-Dev: graceful Fallback mit Demo-Daten
package services

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Service repräsentiert einen systemd-Service.
type Service struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	LoadState   string `json:"load_state"`   // loaded / not-found / error
	ActiveState string `json:"active_state"` // active / inactive / failed / activating
	SubState    string `json:"sub_state"`    // running / dead / failed / exited
	Enabled     bool   `json:"enabled"`
	IsProtected bool   `json:"is_protected"` // Kritischer Service — kein Stop/Disable
}

// Action definiert eine Service-Aktion.
type Action string

const (
	ActionStart   Action = "start"
	ActionStop    Action = "stop"
	ActionRestart Action = "restart"
	ActionEnable  Action = "enable"
	ActionDisable Action = "disable"
	ActionReload  Action = "reload"
)

// protectedServices sind kritische Services die nicht gestoppt werden dürfen.
var protectedServices = map[string]bool{
	"ssh.service":           true,
	"sshd.service":          true,
	"ctrld.service":         true,
	"systemd-journald.service": true,
	"dbus.service":          true,
	"network.service":       true,
	"NetworkManager.service": true,
	"systemd-networkd.service": true,
}

// List gibt alle aktiven/bekannten systemd-Services zurück.
func List(ctx context.Context) ([]Service, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "list-units",
		"--type=service",
		"--all",
		"--no-pager",
		"--no-legend",
		"--plain",
	)
	out, err := cmd.Output()
	if err != nil {
		if isNotFound(err) {
			return demoServices(), nil
		}
		return nil, fmt.Errorf("services: list fehlgeschlagen: %w", err)
	}

	return parseServiceList(string(out)), nil
}

// Get gibt einen einzelnen Service zurück.
func Get(ctx context.Context, name string) (*Service, error) {
	// Status holen
	cmd := exec.CommandContext(ctx, "systemctl", "show", name,
		"--property=Id,Description,LoadState,ActiveState,SubState,UnitFileState",
		"--no-pager",
	)
	out, err := cmd.Output()
	if err != nil {
		if isNotFound(err) {
			// Demo-Modus
			for _, s := range demoServices() {
				if s.Name == name {
					return &s, nil
				}
			}
			return nil, fmt.Errorf("services: service nicht gefunden: %s", name)
		}
		return nil, fmt.Errorf("services: get fehlgeschlagen: %w", err)
	}

	return parseServiceShow(string(out)), nil
}

// Execute führt eine Aktion auf einem Service aus.
// Gibt einen Fehler zurück wenn der Service geschützt ist
// oder die Aktion nicht erlaubt ist.
func Execute(ctx context.Context, name string, action Action) error {
	// Geschützte Services prüfen
	if isProtected(name) {
		switch action {
		case ActionStop, ActionDisable:
			return fmt.Errorf("services: '%s' ist ein kritischer service und kann nicht %s werden", name, action)
		}
	}

	// Aktion validieren
	switch action {
	case ActionStart, ActionStop, ActionRestart, ActionEnable, ActionDisable, ActionReload:
		// OK
	default:
		return fmt.Errorf("services: ungültige aktion: %s", action)
	}

	cmd := exec.CommandContext(ctx, "systemctl", string(action), name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if isNotFound(err) {
			// Demo-Modus auf Windows
			return nil
		}
		return fmt.Errorf("services: %s %s fehlgeschlagen: %s", action, name, strings.TrimSpace(string(out)))
	}

	return nil
}

// GetLogs gibt die letzten Log-Zeilen eines Services zurück.
func GetLogs(ctx context.Context, name string, lines int) ([]string, error) {
	if lines <= 0 {
		lines = 50
	}

	cmd := exec.CommandContext(ctx, "journalctl",
		"--unit="+name,
		fmt.Sprintf("--lines=%d", lines),
		"--no-pager",
		"--output=short-iso",
	)
	out, err := cmd.Output()
	if err != nil {
		if isNotFound(err) {
			return demoLogs(name), nil
		}
		return nil, fmt.Errorf("services: logs fehlgeschlagen: %w", err)
	}

	var logLines []string
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		logLines = append(logLines, scanner.Text())
	}
	return logLines, nil
}

// ── Parsing ───────────────────────────────────────────────────────────────────

// parseServiceList parst die Ausgabe von systemctl list-units.
func parseServiceList(output string) []Service {
	var services []Service
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		name := fields[0]
		if !strings.HasSuffix(name, ".service") {
			continue
		}

		svc := Service{
			Name:        name,
			LoadState:   fields[1],
			ActiveState: fields[2],
			SubState:    fields[3],
			IsProtected: isProtected(name),
		}

		if len(fields) > 4 {
			svc.Description = strings.Join(fields[4:], " ")
		}

		services = append(services, svc)
	}

	return services
}

// parseServiceShow parst die Ausgabe von systemctl show.
func parseServiceShow(output string) *Service {
	svc := &Service{}
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]

		switch key {
		case "Id":
			svc.Name = val
			svc.IsProtected = isProtected(val)
		case "Description":
			svc.Description = val
		case "LoadState":
			svc.LoadState = val
		case "ActiveState":
			svc.ActiveState = val
		case "SubState":
			svc.SubState = val
		case "UnitFileState":
			svc.Enabled = val == "enabled" || val == "static"
		}
	}

	return svc
}

// ── Hilfsfunktionen ───────────────────────────────────────────────────────────

func isProtected(name string) bool {
	return protectedServices[name]
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found") ||
		strings.Contains(err.Error(), "executable file not found")
}

// demoServices gibt Demo-Services für Windows-Entwicklung zurück.
func demoServices() []Service {
	return []Service{
		{Name: "nginx.service",    Description: "A high performance web server",     LoadState: "loaded", ActiveState: "active",   SubState: "running", Enabled: true},
		{Name: "sshd.service",     Description: "OpenSSH server daemon",              LoadState: "loaded", ActiveState: "active",   SubState: "running", Enabled: true,  IsProtected: true},
		{Name: "cron.service",     Description: "Regular background program processing daemon", LoadState: "loaded", ActiveState: "active", SubState: "running", Enabled: true},
		{Name: "docker.service",   Description: "Docker Application Container Engine", LoadState: "loaded", ActiveState: "active",  SubState: "running", Enabled: true},
		{Name: "ufw.service",      Description: "Uncomplicated firewall",             LoadState: "loaded", ActiveState: "active",   SubState: "exited",  Enabled: true},
		{Name: "postgresql.service", Description: "PostgreSQL RDBMS",               LoadState: "loaded", ActiveState: "inactive", SubState: "dead",    Enabled: false},
		{Name: "redis.service",    Description: "Advanced key-value store",           LoadState: "loaded", ActiveState: "active",   SubState: "running", Enabled: true},
		{Name: "fail2ban.service", Description: "Fail2Ban Service",                   LoadState: "loaded", ActiveState: "active",   SubState: "running", Enabled: true},
	}
}

// demoLogs gibt Demo-Logs für Windows-Entwicklung zurück.
func demoLogs(name string) []string {
	return []string{
		fmt.Sprintf("2026-06-22T10:00:00+0000 host systemd[1]: Started %s.", name),
		fmt.Sprintf("2026-06-22T10:00:01+0000 host %s[1234]: Service initialized", strings.TrimSuffix(name, ".service")),
		fmt.Sprintf("2026-06-22T10:00:02+0000 host %s[1234]: Listening on port 80", strings.TrimSuffix(name, ".service")),
	}
}
