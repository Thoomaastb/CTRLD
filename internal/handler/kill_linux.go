//go:build linux

package handler

import (
	"fmt"
	"os"
	"syscall"
)

// killProcess sendet SIGTERM an einen Prozess.
func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("prozess %d nicht gefunden: %w", pid, err)
	}
	return proc.Signal(syscall.SIGTERM)
}
