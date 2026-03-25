package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// DetectConfigDir returns the active Claude Code config directory.
//
// Priority:
//  1. $CLAUDE_CONFIG_DIR (if set, use exclusively)
//  2. $XDG_CONFIG_HOME/claude/ (or ~/.config/claude/ if XDG unset, Unix only)
//  3. ~/.claude/ (fallback)
//  4. If both #2 and #3 exist, prefer ~/.claude/ and log a warning.
func DetectConfigDir() (string, error) {
	if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
		abs, err := filepath.Abs(v)
		if err != nil {
			return "", fmt.Errorf("resolving CLAUDE_CONFIG_DIR: %w", err)
		}
		return abs, nil
	}
	return DefaultConfigDir()
}

// DefaultConfigDir returns the default Claude Code config directory
// (ignoring CLAUDE_CONFIG_DIR env var). This is used to resolve accounts
// with empty config_dir in the registry, which should always point to the
// default location rather than the active session's override.
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}

	dotClaude := filepath.Join(home, ".claude")

	var xdgPath string
	if runtime.GOOS != "windows" {
		xdgBase := os.Getenv("XDG_CONFIG_HOME")
		if xdgBase == "" {
			xdgBase = filepath.Join(home, ".config")
		}
		xdgPath = filepath.Join(xdgBase, "claude")
	}

	dotClaudeExists := dirExists(dotClaude)
	xdgExists := xdgPath != "" && dirExists(xdgPath)

	switch {
	case dotClaudeExists && xdgExists:
		log.Printf("cx: both %q and %q exist; using %q", xdgPath, dotClaude, dotClaude)
		return dotClaude, nil
	case dotClaudeExists:
		return dotClaude, nil
	case xdgExists:
		return xdgPath, nil
	}

	return "", fmt.Errorf("no Claude Code config directory found (tried %q and %q)", dotClaude, xdgPath)
}

// dirExists returns true when path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
