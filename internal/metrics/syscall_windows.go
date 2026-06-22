//go:build windows

package metrics

import "errors"

// syscallStatfs ist ein Stub für Windows — CTRLD läuft produktiv auf Linux.
// Für lokale Windows-Entwicklung werden Disk-Metriken einfach übersprungen.
type syscallStatfs struct {
	Blocks uint64
	Bfree  uint64
	Bsize  int64
}

func statfs(_ string, _ *syscallStatfs) error {
	return errors.New("statfs: nicht verfügbar auf windows")
}
