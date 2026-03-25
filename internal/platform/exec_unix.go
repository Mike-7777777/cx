//go:build !windows

package platform

import (
	"os/exec"
	"syscall"
)

// ExecProgram replaces the current process with the named program.
// On Unix, this uses syscall.Exec for true process replacement.
func ExecProgram(name string, args []string, env []string) error {
	binary, err := exec.LookPath(name)
	if err != nil {
		return err
	}
	argv := append([]string{name}, args...)
	return syscall.Exec(binary, argv, env)
}
