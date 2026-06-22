package alerts_test

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/Thoomaastb/CTRLD/internal/alerts"
	database "github.com/Thoomaastb/CTRLD/internal/db"
	"github.com/Thoomaastb/CTRLD/internal/metrics"
)

func newTestService(t *testing.T) (*alerts.Service, *database.DB) {
	t.Helper()
	db, err := database.Open(":memory:", zerolog.Nop())
	if err != nil {
		t.Fatalf("db öffnen fehlgeschlagen: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := alerts.NewService(db, "", "test-host", zerolog.Nop())
	return svc, db
}

func makeSnap(cpu, ram float64) *metrics.Snapshot {
	return &metrics.Snapshot{
		Timestamp: time.Now(),
		CPU:       metrics.CPUMetrics{UsagePercent: cpu, NumCores: 4},
		RAM: metrics.RAMMetrics{
			UsagePercent:   ram,
			TotalBytes:     8 * 1024 * 1024 * 1024,
			UsedBytes:      uint64(ram / 100 * 8 * 1024 * 1024 * 1024),
			AvailableBytes: uint64((100 - ram) / 100 * 8 * 1024 * 1024 * 1024),
		},
		LoadAvg: metrics.LoadAvgMetrics{Load1: 1.5, Load5: 1.2, Load15: 1.0},
	}
}

// Test: Kein Alert unter Schwellwert
func TestEvaluate_NoAlert_BelowThreshold(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// CPU bei 50% — unter Warning (80%)
	svc.Evaluate(ctx, makeSnap(50, 50))
	svc.Evaluate(ctx, makeSnap(50, 50))

	alerts, err := svc.ListActive(ctx)
	if err != nil {
		t.Fatalf("list fehlgeschlagen: %v", err)
	}
	if len(alerts) != 0 {
		t.Errorf("erwartet 0 alerts, bekommen %d", len(alerts))
	}
}

// Test: Alert wird NICHT bei erstem Überschreiten ausgelöst (Anti-Flapping)
func TestEvaluate_NoAlert_SingleViolation(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// Nur einmal über Schwellwert — kein Alert
	svc.Evaluate(ctx, makeSnap(85, 50))

	active, _ := svc.ListActive(ctx)
	if len(active) != 0 {
		t.Error("alert sollte erst nach 2 aufeinanderfolgenden Überschreitungen ausgelöst werden")
	}
}

// Test: Alert wird nach 2 aufeinanderfolgenden Überschreitungen ausgelöst
func TestEvaluate_AlertTriggered_AfterTwoViolations(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	svc.Evaluate(ctx, makeSnap(85, 50)) // Erste Überschreitung
	svc.Evaluate(ctx, makeSnap(87, 50)) // Zweite → Alert

	active, err := svc.ListActive(ctx)
	if err != nil {
		t.Fatalf("list fehlgeschlagen: %v", err)
	}
	if len(active) == 0 {
		t.Error("alert sollte nach 2 Überschreitungen ausgelöst sein")
	}
	if active[0].Severity != "warning" {
		t.Errorf("erwartet warning, bekommen %s", active[0].Severity)
	}
}

// Test: Critical-Alert bei kritischem Schwellwert
func TestEvaluate_CriticalAlert(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	svc.Evaluate(ctx, makeSnap(92, 50))
	svc.Evaluate(ctx, makeSnap(94, 50))

	active, _ := svc.ListActive(ctx)
	if len(active) == 0 {
		t.Fatal("kein alert ausgelöst")
	}
	if active[0].Severity != "critical" {
		t.Errorf("erwartet critical, bekommen %s", active[0].Severity)
	}
}

// Test: Kein doppelter Alert wenn bereits einer aktiv ist
func TestEvaluate_NoDuplicateAlert(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// Alert auslösen
	svc.Evaluate(ctx, makeSnap(85, 50))
	svc.Evaluate(ctx, makeSnap(85, 50))

	// Weitere Überschreitungen
	svc.Evaluate(ctx, makeSnap(87, 50))
	svc.Evaluate(ctx, makeSnap(89, 50))

	active, _ := svc.ListActive(ctx)
	if len(active) > 1 {
		t.Errorf("erwartet max 1 aktiver alert, bekommen %d", len(active))
	}
}

// Test: Alert wird aufgelöst wenn Wert unter Schwellwert fällt
func TestEvaluate_AlertResolved(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// Alert auslösen
	svc.Evaluate(ctx, makeSnap(85, 50))
	svc.Evaluate(ctx, makeSnap(85, 50))

	active, _ := svc.ListActive(ctx)
	if len(active) == 0 {
		t.Fatal("alert sollte ausgelöst sein")
	}

	// Wert fällt unter Schwellwert
	svc.Evaluate(ctx, makeSnap(60, 50))

	active, _ = svc.ListActive(ctx)
	if len(active) != 0 {
		t.Error("alert sollte aufgelöst sein")
	}
}

// Test: Schwellwerte können zur Laufzeit aktualisiert werden
func TestUpdateThresholds(t *testing.T) {
	svc, _ := newTestService(t)

	newThresholds := []alerts.Threshold{
		{Type: "cpu", Warning: 95, Critical: 99, Enabled: true},
	}
	svc.UpdateThresholds(newThresholds)

	got := svc.GetThresholds()
	if len(got) != 1 {
		t.Fatalf("erwartet 1 threshold, bekommen %d", len(got))
	}
	if got[0].Warning != 95 {
		t.Errorf("erwartet warning 95, bekommen %f", got[0].Warning)
	}
}

// Test: RAM-Alert korrekt
func TestEvaluate_RAMAlert(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	svc.Evaluate(ctx, makeSnap(50, 88))
	svc.Evaluate(ctx, makeSnap(50, 90))

	active, _ := svc.ListActive(ctx)
	hasRAM := false
	for _, a := range active {
		if a.Type == "ram" {
			hasRAM = true
		}
	}
	if !hasRAM {
		t.Error("ram alert sollte ausgelöst sein")
	}
}

// Test: Deaktivierter Threshold löst keinen Alert aus
func TestEvaluate_DisabledThreshold(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	svc.UpdateThresholds([]alerts.Threshold{
		{Type: "cpu", Warning: 70, Critical: 80, Enabled: false},
	})

	svc.Evaluate(ctx, makeSnap(95, 50))
	svc.Evaluate(ctx, makeSnap(95, 50))

	active, _ := svc.ListActive(ctx)
	if len(active) != 0 {
		t.Error("deaktivierter threshold sollte keinen alert auslösen")
	}
}
