package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Mike-7777777/cc-monitor/internal/config"
	"github.com/Mike-7777777/cc-monitor/internal/platform"
)

// validAccountName restricts account names to safe alphanumeric characters,
// hyphens, and underscores to prevent path traversal attacks.
var validAccountName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// sharedLinkDirs are subdirectories that are junctioned/symlinked from the
// primary config dir into every secondary account dir.
var sharedLinkDirs = []string{
	filepath.Join("plugins", "cache"),
	"ide",
}

func runInit() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: cc-monitor init <name> [--force]")
		os.Exit(1)
	}

	name := os.Args[2]
	if !validAccountName.MatchString(name) {
		fmt.Fprintf(os.Stderr, "cc-monitor init: invalid account name %q (only letters, digits, hyphens, underscores)\n", name)
		os.Exit(1)
	}
	force := hasFlag("--force")

	// Validate: primary config dir must exist.
	primaryDir, err := config.DetectConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor init: %v\n", err)
		os.Exit(1)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor init: %v\n", err)
		os.Exit(1)
	}

	targetDir := filepath.Join(home, ".claude-"+name)

	if _, err := os.Stat(targetDir); err == nil && !force {
		fmt.Fprintf(os.Stderr, "cc-monitor init: %q already exists; use --force to overwrite\n", targetDir)
		os.Exit(1)
	}

	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor init: creating target dir: %v\n", err)
		os.Exit(1)
	}

	// Create junctions/symlinks for shared dirs.
	for _, rel := range sharedLinkDirs {
		src := filepath.Join(primaryDir, rel)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}

		dst := filepath.Join(targetDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "cc-monitor init: creating parent for link %q: %v\n", dst, err)
			os.Exit(1)
		}
		// Remove existing link/dir before recreating.
		_ = os.RemoveAll(dst)
		if err := platform.CreateLink(dst, src); err != nil {
			fmt.Fprintf(os.Stderr, "cc-monitor init: linking %q → %q: %v\n", dst, src, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "  linked %s\n", rel)
	}

	// Sync shared config files.
	if err := syncFiles(primaryDir, targetDir, true); err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor init: syncing files: %v\n", err)
		os.Exit(1)
	}

	// Register account in registry.
	regPath, err := config.RegistryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor init: %v\n", err)
		os.Exit(1)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor init: %v\n", err)
		os.Exit(1)
	}

	// If no primary is registered yet, detect and set it.
	if reg.Primary == "" {
		reg.Primary = "primary"
		reg.AddAccount("primary", "")
	}

	reg.AddAccount(name, targetDir)

	if err := reg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "cc-monitor init: saving registry: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "initialized account %q at %s\n", name, targetDir)
	fmt.Fprintf(os.Stderr, "next step: CLAUDE_CONFIG_DIR=%s claude auth login\n", targetDir)
}

// hasFlag reports whether flag appears verbatim in os.Args[3:].
func hasFlag(flag string) bool {
	if len(os.Args) <= 3 {
		return false
	}
	for _, arg := range os.Args[3:] {
		if arg == flag {
			return true
		}
	}
	return false
}
