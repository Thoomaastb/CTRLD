package logs

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// TailParams enthält Parameter für den Live-Tail.
type TailParams struct {
	Unit   string // Optional: nur diese Unit
	Source string // journald (default)
}

// TailEntry ist ein einzelner Live-Tail-Eintrag.
type TailEntry struct {
	Entry
	IsNew bool `json:"is_new"` // Immer true bei Live-Tail
}

// Tail startet einen Live-Tail und sendet neue Einträge an den channel.
// Läuft bis der Context abgebrochen wird.
func Tail(ctx context.Context, params TailParams, out chan<- TailEntry) error {
	args := []string{
		"--follow",
		"--output=json",
		"--no-pager",
		"--lines=0", // Nur neue Einträge, keine History
	}

	if params.Unit != "" {
		args = append(args, "--unit="+params.Unit)
	}

	cmd := exec.CommandContext(ctx, "journalctl", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		if isNotFound(err) {
			// journalctl nicht verfügbar — Demo-Modus
			go tailDemo(ctx, out)
			return nil
		}
		return err
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
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

			select {
			case out <- TailEntry{
				Entry: Entry{
					Timestamp: ts,
					Severity:  priorityToSeverity(raw.Priority),
					Unit:      unit,
					Message:   raw.Message,
					Source:    "journald",
					PID:       raw.PID,
				},
				IsNew: true,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		<-ctx.Done()
		cmd.Process.Kill()
	}()

	return nil
}

// tailDemo simuliert Log-Einträge wenn journalctl nicht verfügbar ist (Dev-Modus).
func tailDemo(ctx context.Context, out chan<- TailEntry) {
	units := []string{"nginx.service", "sshd.service", "cron.service", "systemd-timesyncd.service"}
	severities := []string{"info", "info", "info", "warning", "error"}
	messages := []string{
		"Started A high performance web server and a reverse proxy server.",
		"New session 42 of user admin.",
		"pam_unix(cron:session): session opened for user root",
		"Connection closed by 192.168.1.100 port 22",
		"Failed to connect to database: connection refused",
	}

	i := 0
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			idx := i % len(messages)
			select {
			case out <- TailEntry{
				Entry: Entry{
					Timestamp: time.Now(),
					Severity:  severities[idx%len(severities)],
					Unit:      units[idx%len(units)],
					Message:   messages[idx],
					Source:    "journald",
				},
				IsNew: true,
			}:
			default:
			}
			i++
		}
	}
}
