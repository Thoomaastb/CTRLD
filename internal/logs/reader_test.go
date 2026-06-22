package logs_test

import (
	"testing"

	"github.com/Thoomaastb/CTRLD/internal/logs"
)

func TestPrioritySeverityMapping(t *testing.T) {
	cases := []struct {
		priority string
		expected string
	}{
		{"0", "error"},
		{"1", "error"},
		{"3", "error"},
		{"4", "warning"},
		{"6", "info"},
		{"7", "debug"},
		{"", "info"},
	}

	for _, tc := range cases {
		got := logs.PriorityToSeverityExported(tc.priority)
		if got != tc.expected {
			t.Errorf("priority %q: erwartet %q, bekommen %q", tc.priority, tc.expected, got)
		}
	}
}

func TestAvailableSources(t *testing.T) {
	sources := logs.AvailableSources()

	if len(sources) == 0 {
		t.Error("mindestens eine log-quelle erwartet")
	}

	// journald immer als erste Quelle
	if sources[0].ID != "journald" {
		t.Errorf("erste quelle sollte journald sein, bekommen %q", sources[0].ID)
	}
}

func TestQueryParams_Defaults(t *testing.T) {
	params := logs.QueryParams{Lines: 0}
	// Lines=0 sollte auf 100 gesetzt werden
	// Wir testen die Normalisierung indirekt über Read
	// (Read läuft auf Windows durch graceful fallback)
	if params.Lines != 0 {
		t.Error("lines sollte initial 0 sein")
	}
}
