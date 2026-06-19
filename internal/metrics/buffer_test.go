package metrics_test

import (
	"testing"
	"time"

	"github.com/Thoomaastb/CTRLD/internal/metrics"
)

func makeSnapshot(t time.Time, cpu float64) *metrics.Snapshot {
	return &metrics.Snapshot{
		Timestamp: t,
		CPU:       metrics.CPUMetrics{UsagePercent: cpu},
	}
}

func TestBuffer_Empty(t *testing.T) {
	b := metrics.NewBuffer()
	if b.Latest() != nil {
		t.Error("leerer buffer sollte nil zurückgeben")
	}
	if len(b.History()) != 0 {
		t.Error("leerer buffer sollte leere history zurückgeben")
	}
}

func TestBuffer_SinglePush(t *testing.T) {
	b := metrics.NewBuffer()
	now := time.Now()
	b.Push(makeSnapshot(now, 42.5))

	latest := b.Latest()
	if latest == nil {
		t.Fatal("latest sollte nicht nil sein")
	}
	if latest.CPU.UsagePercent != 42.5 {
		t.Errorf("erwartet 42.5, bekommen %f", latest.CPU.UsagePercent)
	}
}

func TestBuffer_History_Order(t *testing.T) {
	b := metrics.NewBuffer()
	base := time.Now()

	for i := 0; i < 5; i++ {
		b.Push(makeSnapshot(base.Add(time.Duration(i)*time.Second), float64(i*10)))
	}

	history := b.History()
	if len(history) != 5 {
		t.Fatalf("erwartet 5 einträge, bekommen %d", len(history))
	}

	// Chronologische Reihenfolge
	for i := 1; i < len(history); i++ {
		if !history[i].Timestamp.After(history[i-1].Timestamp) {
			t.Error("history sollte chronologisch sein")
		}
	}
}

func TestBuffer_Rolling(t *testing.T) {
	b := metrics.NewBuffer()
	base := time.Now()

	// Mehr als BufferSize Einträge pushen
	total := metrics.BufferSize + 10
	for i := 0; i < total; i++ {
		b.Push(makeSnapshot(base.Add(time.Duration(i)*time.Second), float64(i)))
	}

	history := b.History()
	if len(history) != metrics.BufferSize {
		t.Errorf("erwartet %d einträge, bekommen %d", metrics.BufferSize, len(history))
	}

	// Älteste Einträge sollten verdrängt sein
	if history[0].CPU.UsagePercent != float64(10) {
		t.Errorf("ältester eintrag sollte %d sein, bekommen %f", 10, history[0].CPU.UsagePercent)
	}
}

func TestBuffer_Since(t *testing.T) {
	b := metrics.NewBuffer()
	base := time.Now()

	for i := 0; i < 10; i++ {
		b.Push(makeSnapshot(base.Add(time.Duration(i)*time.Second), float64(i)))
	}

	cutoff := base.Add(5 * time.Second)
	recent := b.Since(cutoff)

	// Alle Snapshots nach cutoff
	for _, s := range recent {
		if !s.Timestamp.After(cutoff) {
			t.Error("since sollte nur einträge nach dem cutoff zurückgeben")
		}
	}
}
