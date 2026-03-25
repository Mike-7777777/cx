package platform

import (
	"os"
	"strings"
)

// Shell identifies the user's interactive shell.
type Shell int

const (
	ShellBash       Shell = iota // default / fallback
	ShellFish                    // $SHELL contains "fish"
	ShellPowerShell              // $PSModulePath is set
)

// DetectShell returns the Shell type of the current process's parent shell.
// Detection order: Bash (via BASH_VERSION/BASH) → PowerShell → Fish → Bash.
// On Windows, PSModulePath is set system-wide even inside Git Bash, so we
// check for Bash-specific env vars first to avoid false PowerShell detection.
func DetectShell() Shell {
	if os.Getenv("BASH_VERSION") != "" || os.Getenv("BASH") != "" {
		return ShellBash
	}
	if os.Getenv("PSModulePath") != "" {
		return ShellPowerShell
	}
	if shell := os.Getenv("SHELL"); strings.Contains(shell, "fish") {
		return ShellFish
	}
	return ShellBash
}
