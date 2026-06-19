//go:build linux

package metrics

import (
	"golang.org/x/sys/unix"
)

// syscallStatfs ist ein plattformunabhängiges Struct für statfs-Daten.
type syscallStatfs struct {
	Blocks uint64
	Bfree  uint64
	Bsize  int64
}

// statfs liest Dateisystem-Informationen für einen Mount-Punkt.
func statfs(path string, s *syscallStatfs) error {
	var stat unix.Statfs_t
	if err := unix.Statfs(path, &stat); err != nil {
		return err
	}
	s.Blocks = stat.Blocks
	s.Bfree  = stat.Bfree
	s.Bsize  = stat.Bsize
	return nil
}
