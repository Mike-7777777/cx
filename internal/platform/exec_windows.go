//go:build windows

package platform

import (
	"os"
	"os/exec"
)

// ExecProgram launches the named program as a child process and waits for it.
// On Windows, Go cannot replace the current process via exec, so we forward
// stdin/stdout/stderr and propagate the exit code.
func ExecProgram(name string, args []string, env []string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}
	os.Exit(0)
	return nil // unreachable
}
