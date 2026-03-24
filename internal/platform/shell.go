package platform

import (
	"os"
	"strings"
)

// Shell identifies the user's interactive shell.
type Shell int

const (
	ShellBash        Shell = iota // default / fallback
	ShellFish                     // $SHELL contains "fish"
	ShellPowerShell               // $PSModulePath is set
)

// DetectShell returns the Shell type of the current process's parent shell.
// Detection order: PowerShell → Fish → Bash.
func DetectShell() Shell {
	if os.Getenv("PSModulePath") != "" {
		return ShellPowerShell
	}
	if shell := os.Getenv("SHELL"); strings.Contains(shell, "fish") {
		return ShellFish
	}
	return ShellBash
}
