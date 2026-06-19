package ratelimit_test

import (
	"testing"

	"github.com/Thoomaastb/CTRLD/internal/ratelimit"
)

func TestLimiter_NotBlocked_Initially(t *testing.T) {
	l := ratelimit.New()
	blocked, _ := l.IsBlocked("1.2.3.4")
	if blocked {
		t.Error("neue IP sollte nicht gesperrt sein")
	}
}

func TestLimiter_BlockAfterShortThreshold(t *testing.T) {
	l := ratelimit.New()
	ip := "10.0.0.1"

	for i := 0; i < ratelimit.MaxAttemptsShort; i++ {
		l.RecordFailure(ip)
	}

	blocked, remaining := l.IsBlocked(ip)
	if !blocked {
		t.Error("IP sollte nach 3 Fehlversuchen gesperrt sein")
	}
	if remaining <= 0 {
		t.Error("verbleibende sperrzeit sollte > 0 sein")
	}
}

func TestLimiter_BlockAfterLongThreshold(t *testing.T) {
	l := ratelimit.New()
	ip := "10.0.0.2"

	for i := 0; i < ratelimit.MaxAttemptsLong; i++ {
		l.RecordFailure(ip)
	}

	blocked, remaining := l.IsBlocked(ip)
	if !blocked {
		t.Error("IP sollte nach 10 Fehlversuchen gesperrt sein")
	}
	// Lange Sperre: ~1h
	if remaining < 59*60*1000000000 { // 59 Min in Nanosekunden
		t.Errorf("lange sperre sollte ~1h sein, bekommen: %v", remaining)
	}
}

func TestLimiter_Reset(t *testing.T) {
	l := ratelimit.New()
	ip := "10.0.0.3"

	for i := 0; i < ratelimit.MaxAttemptsShort; i++ {
		l.RecordFailure(ip)
	}

	blocked, _ := l.IsBlocked(ip)
	if !blocked {
		t.Fatal("IP sollte gesperrt sein")
	}

	l.Reset(ip)

	blocked, _ = l.IsBlocked(ip)
	if blocked {
		t.Error("IP sollte nach reset entsperrt sein")
	}
	if l.Attempts(ip) != 0 {
		t.Error("attempts sollte nach reset 0 sein")
	}
}

func TestLimiter_DifferentIPs(t *testing.T) {
	l := ratelimit.New()

	for i := 0; i < ratelimit.MaxAttemptsShort; i++ {
		l.RecordFailure("10.0.0.1")
	}

	// Andere IP sollte nicht betroffen sein
	blocked, _ := l.IsBlocked("10.0.0.2")
	if blocked {
		t.Error("andere IP sollte nicht gesperrt sein")
	}
}

func TestLimiter_Attempts_Count(t *testing.T) {
	l := ratelimit.New()
	ip := "10.0.0.4"

	if l.Attempts(ip) != 0 {
		t.Error("initiale attempts sollten 0 sein")
	}

	l.RecordFailure(ip)
	l.RecordFailure(ip)

	if l.Attempts(ip) != 2 {
		t.Errorf("erwartet 2 attempts, bekommen %d", l.Attempts(ip))
	}
}
