package ratelimit

import (
	"sync"
	"time"
)

// Schwellwerte laut Briefing
const (
	MaxAttemptsShort    = 3               // Nach 3 Fehlern → kurze Sperre
	LockoutDurationShort = 5 * time.Minute // 5 Min Sperre
	MaxAttemptsLong     = 10              // Nach 10 Fehlern → lange Sperre
	LockoutDurationLong = time.Hour       // 1h Sperre
)

// entry speichert den Zustand pro IP.
type entry struct {
	attempts  int
	lockedUntil time.Time
}

// Limiter ist ein thread-sicherer In-Memory Rate-Limiter.
// Wird beim Start instanziiert und lebt im Memory (kein Persist — Restart = Reset).
type Limiter struct {
	mu      sync.Mutex
	entries map[string]*entry
}

// New erstellt einen neuen Limiter.
func New() *Limiter {
	l := &Limiter{
		entries: make(map[string]*entry),
	}
	// Hintergrund-Cleanup alle 10 Minuten
	go l.cleanup()
	return l
}

// IsBlocked prüft ob eine IP gesperrt ist.
// Gibt true zurück wenn gesperrt, sowie die verbleibende Sperrzeit.
func (l *Limiter) IsBlocked(ip string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[ip]
	if !ok {
		return false, 0
	}

	if time.Now().Before(e.lockedUntil) {
		remaining := time.Until(e.lockedUntil)
		return true, remaining
	}

	return false, 0
}

// RecordFailure zählt einen fehlgeschlagenen Versuch für eine IP
// und sperrt sie wenn die Schwellwerte überschritten sind.
func (l *Limiter) RecordFailure(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[ip]
	if !ok {
		e = &entry{}
		l.entries[ip] = e
	}

	// Wenn Sperre abgelaufen ist, Counter zurücksetzen
	if time.Now().After(e.lockedUntil) && e.lockedUntil != (time.Time{}) {
		e.attempts = 0
		e.lockedUntil = time.Time{}
	}

	e.attempts++

	switch {
	case e.attempts >= MaxAttemptsLong:
		e.lockedUntil = time.Now().Add(LockoutDurationLong)
	case e.attempts >= MaxAttemptsShort:
		e.lockedUntil = time.Now().Add(LockoutDurationShort)
	}
}

// Reset setzt den Counter einer IP zurück (nach erfolgreichem Login).
func (l *Limiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, ip)
}

// Attempts gibt die aktuelle Fehlversuch-Anzahl einer IP zurück.
func (l *Limiter) Attempts(ip string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[ip]
	if !ok {
		return 0
	}
	return e.attempts
}

// cleanup löscht abgelaufene Einträge periodisch.
func (l *Limiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for ip, e := range l.entries {
			// Einträge ohne aktive Sperre und ohne kürzliche Versuche löschen
			if now.After(e.lockedUntil) && e.attempts < MaxAttemptsShort {
				delete(l.entries, ip)
			}
		}
		l.mu.Unlock()
	}
}
