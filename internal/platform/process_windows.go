//go:build windows

package platform

import "syscall"

// IsProcessRunning checks if a process with the given PID exists.
// On Windows, OpenProcess with PROCESS_QUERY_LIMITED_INFORMATION succeeds
// only if the process exists and is accessible.
func IsProcessRunning(pid int) bool {
	// PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	h, err := syscall.OpenProcess(0x1000, false, uint32(pid))
	if err != nil {
		return false
	}
	syscall.CloseHandle(h)
	return true
}
