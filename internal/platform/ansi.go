package platform

import (
	"os"
	"runtime"
)

// ANSIEnabled reports whether ANSI colour escape codes should be emitted.
// On Unix systems this is always true. On Windows it checks for Windows
// Terminal (WT_SESSION) or a well-known terminal emulator (TERM_PROGRAM).
func ANSIEnabled() bool {
	if runtime.GOOS != "windows" {
		return true
	}
	if os.Getenv("WT_SESSION") != "" {
		return true
	}
	if os.Getenv("TERM_PROGRAM") != "" {
		return true
	}
	return false
}
