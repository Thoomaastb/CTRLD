//go:build !linux && !windows

package handler

import "fmt"

func killProcess(pid int) error {
	return fmt.Errorf("prozess beenden nicht unterstützt auf diesem betriebssystem")
}