package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Mike-7777777/cx/internal/config"
	"github.com/Mike-7777777/cx/internal/platform"
)

// validAccountName restricts account names to safe alphanumeric characters,
// hyphens, and underscores to prevent path traversal attacks.
var validAccountName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// sharedLinkDirs are subdirectories that are junctioned/symlinked from the
// main config dir into every secondary account dir.
var sharedLinkDirs = []string{
	filepath.Join("plugins", "cache"),
	"ide",
	"skills",
	"projects",
}

func runInit() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: cx init <name> [--force]")
		os.Exit(1)
	}

	name := os.Args[2]
	if !validAccountName.MatchString(name) {
		fmt.Fprintf(os.Stderr, "cx init: invalid account name %q (only letters, digits, hyphens, underscores)\n", name)
		os.Exit(1)
	}
	force := hasFlag("--force")

	// Validate: main config dir must exist.
	mainDir, err := config.DetectConfigDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx init: %v\n", err)
		os.Exit(1)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx init: %v\n", err)
		os.Exit(1)
	}

	targetDir := filepath.Join(home, ".claude-"+name)

	if _, err := os.Stat(targetDir); err == nil && !force {
		fmt.Fprintf(os.Stderr, "cx init: %q already exists; use --force to overwrite\n", targetDir)
		os.Exit(1)
	}

	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "cx init: creating target dir: %v\n", err)
		os.Exit(1)
	}

	// Create junctions/symlinks for shared dirs.
	for _, rel := range sharedLinkDirs {
		src := filepath.Join(mainDir, rel)
		if _, err := os.Stat(src); os.IsNotExist(err) {
			continue
		}

		dst := filepath.Join(targetDir, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			fmt.Fprintf(os.Stderr, "cx init: creating parent for link %q: %v\n", dst, err)
			os.Exit(1)
		}
		// Remove existing link/dir before recreating.
		_ = os.RemoveAll(dst)
		if err := platform.CreateLink(dst, src); err != nil {
			fmt.Fprintf(os.Stderr, "cx init: linking %q → %q: %v\n", dst, src, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "  linked %s\n", rel)
	}

	// Sync shared config files.
	if err := syncFiles(mainDir, targetDir, true); err != nil {
		fmt.Fprintf(os.Stderr, "cx init: syncing files: %v\n", err)
		os.Exit(1)
	}

	// Register account in registry.
	regPath, err := config.RegistryPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx init: %v\n", err)
		os.Exit(1)
	}

	reg, err := config.LoadOrCreateRegistry(regPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cx init: %v\n", err)
		os.Exit(1)
	}

	// If no main account is registered yet, detect and set it.
	if reg.Main == "" {
		reg.Main = "main"
		reg.AddAccount("main", "")
	}

	reg.AddAccount(name, targetDir)

	if err := reg.Save(); err != nil {
		fmt.Fprintf(os.Stderr, "cx init: saving registry: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "initialized account %q at %s\n", name, targetDir)

	// Auto-launch login for the new account.
	fmt.Fprintf(os.Stderr, "[cx] Launching login for %q...\n", name)
	if err := launchLogin(targetDir); err != nil {
		fmt.Fprintf(os.Stderr, "[cx] login failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "  retry later with: cx login %s\n", name)
	} else {
		fmt.Fprintf(os.Stderr, "[cx] Login successful for %q.\n", name)
	}
}

// hasFlag reports whether flag appears verbatim in os.Args[3:].
func hasFlag(flag string) bool {
	return hasFlagFrom(flag, 3)
}

// hasFlagFrom reports whether flag appears verbatim in os.Args[start:].
func hasFlagFrom(flag string, start int) bool {
	if len(os.Args) <= start {
		return false
	}
	for _, arg := range os.Args[start:] {
		if arg == flag {
			return true
		}
	}
	return false
}
