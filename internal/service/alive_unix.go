//go:build !windows

package service

import (
	"os"
	"syscall"
)

// isProcessAlive sends signal 0 to the process; a nil error means it's alive.
func isProcessAlive(proc *os.Process) bool {
	err := proc.Signal(syscall.Signal(0))
	return err == nil
}
