package logs

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExportTXT exportiert Einträge als lesbaren Text.
func ExportTXT(entries []Entry) string {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("[%s] [%s] [%s] %s\n",
			e.Timestamp.Format("2006-01-02 15:04:05"),
			strings.ToUpper(e.Severity),
			e.Unit,
			e.Message,
		))
	}
	return sb.String()
}

// ExportJSON exportiert Einträge als JSON.
func ExportJSON(entries []Entry) (string, error) {
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("logs: json export fehlgeschlagen: %w", err)
	}
	return string(data), nil
}
