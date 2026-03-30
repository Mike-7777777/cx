//go:build !windows

package platform

import (
	"os"
	"syscall"
)

// IsProcessRunning checks if a process with the given PID exists.
// On Unix, signal 0 succeeds for any process owned by the current user.
func IsProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}
